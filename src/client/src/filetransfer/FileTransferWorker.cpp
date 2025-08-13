#include "FileTransferWorker.h"
#include "FileTransferSession.h"
#include "FileTransferManager.h"
#include <QDebug>
#include <QThread>
#include <QCryptographicHash>
#include <QTimer>
#include <QEventLoop>
#include <QRandomGenerator>

// Constants
static const int CHUNK_TIMEOUT = 30000; // 30 seconds
static const int MAX_CHUNK_RETRIES = 3;
static const int RETRY_DELAY_BASE = 1000; // 1 second base delay
static const int PROGRESS_UPDATE_INTERVAL = 500; // 500ms

FileTransferWorker::FileTransferWorker(FileTransferSession *session, FileTransferManager *manager, QObject *parent)
    : QObject(parent)
    , m_session(session)
    , m_manager(manager)
    , m_isRunning(false)
    , m_isPaused(false)
    , m_isCancelled(false)
    , m_currentChunkIndex(0)
    , m_totalChunks(0)
    , m_completedChunks(0)
    , m_failedChunks()
    , m_chunkRetries()
    , m_progressTimer(new QTimer(this))
    , m_chunkTimeoutTimer(new QTimer(this))
    , m_retryTimer(new QTimer(this))
    , m_pauseCondition()
    , m_mutex()
{
    // Setup progress timer
    m_progressTimer->setInterval(PROGRESS_UPDATE_INTERVAL);
    m_progressTimer->setSingleShot(false);
    connect(m_progressTimer, &QTimer::timeout, this, &FileTransferWorker::updateProgress);
    
    // Setup chunk timeout timer
    m_chunkTimeoutTimer->setInterval(CHUNK_TIMEOUT);
    m_chunkTimeoutTimer->setSingleShot(true);
    connect(m_chunkTimeoutTimer, &QTimer::timeout, this, &FileTransferWorker::onChunkTimeout);
    
    // Setup retry timer
    m_retryTimer->setSingleShot(true);
    connect(m_retryTimer, &QTimer::timeout, this, &FileTransferWorker::retryCurrentChunk);
    
    // Connect to session signals
    if (m_session) {
        connect(m_session, &FileTransferSession::statusChanged, this, &FileTransferWorker::onSessionStatusChanged);
        
        // Calculate total chunks
        if (m_session->getRequest().fileSize > 0) {
            m_totalChunks = (m_session->getRequest().fileSize + CHUNK_SIZE - 1) / CHUNK_SIZE;
        }
    }
    
    qDebug() << "FileTransferWorker created for transfer:" << (m_session ? m_session->getRequest().id : "unknown");
}

FileTransferWorker::~FileTransferWorker()
{
    stopTransfer();
}

void FileTransferWorker::startTransfer()
{
    QMutexLocker locker(&m_mutex);
    
    if (m_isRunning || !m_session) {
        return;
    }
    
    qDebug() << "Starting file transfer:" << m_session->getRequest().id;
    
    m_isRunning = true;
    m_isPaused = false;
    m_isCancelled = false;
    m_currentChunkIndex = 0;
    m_completedChunks = 0;
    m_failedChunks.clear();
    m_chunkRetries.clear();
    
    // Open file
    if (!m_session->openFile()) {
        QString error = m_session->getError();
        qWarning() << "Failed to open file:" << error;
        emit transferFailed(error);
        return;
    }
    
    // Start progress timer
    m_progressTimer->start();
    
    // Start the actual transfer process
    locker.unlock();
    
    if (m_session->getRequest().type == TransferType::Upload) {
        processUpload();
    } else {
        processDownload();
    }
}

void FileTransferWorker::pauseTransfer()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || m_isPaused) {
        return;
    }
    
    qDebug() << "Pausing transfer:" << (m_session ? m_session->getRequest().id : "unknown");
    
    m_isPaused = true;
    m_progressTimer->stop();
    m_chunkTimeoutTimer->stop();
    m_retryTimer->stop();
    
    if (m_session) {
        m_session->setPaused(true);
    }
}

void FileTransferWorker::resumeTransfer()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || !m_isPaused) {
        return;
    }
    
    qDebug() << "Resuming transfer:" << (m_session ? m_session->getRequest().id : "unknown");
    
    m_isPaused = false;
    m_progressTimer->start();
    
    if (m_session) {
        m_session->setPaused(false);
    }
    
    // Wake up any waiting operations
    m_pauseCondition.wakeAll();
}

void FileTransferWorker::cancelTransfer()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning) {
        return;
    }
    
    qDebug() << "Cancelling transfer:" << (m_session ? m_session->getRequest().id : "unknown");
    
    m_isCancelled = true;
    m_isRunning = false;
    
    // Stop all timers
    m_progressTimer->stop();
    m_chunkTimeoutTimer->stop();
    m_retryTimer->stop();
    
    if (m_session) {
        m_session->setCancelled(true);
        m_session->closeFile();
    }
    
    // Wake up any waiting operations
    m_pauseCondition.wakeAll();
    
    emit transferCancelled();
}

void FileTransferWorker::stopTransfer()
{
    cancelTransfer();
}

void FileTransferWorker::onChunkAcknowledged(int chunkIndex)
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || m_isCancelled) {
        return;
    }
    
    // Stop timeout timer for this chunk
    if (chunkIndex == m_currentChunkIndex) {
        m_chunkTimeoutTimer->stop();
    }
    
    // Mark chunk as completed
    if (!m_completedChunkIndices.contains(chunkIndex)) {
        m_completedChunkIndices.insert(chunkIndex);
        m_completedChunks++;
        
        // Update session progress
        if (m_session) {
            m_session->updateChunkProgress(m_completedChunks);
        }
        
        qDebug() << "Chunk acknowledged:" << chunkIndex << "(" << m_completedChunks << "/" << m_totalChunks << ")";
        
        // Check if transfer is complete
        if (m_completedChunks >= m_totalChunks) {
            locker.unlock();
            completeTransfer();
            return;
        }
    }
    
    // Remove from failed chunks if it was there
    m_failedChunks.remove(chunkIndex);
    m_chunkRetries.remove(chunkIndex);
    
    // Continue with next chunk
    locker.unlock();
    processNextChunk();
}

void FileTransferWorker::onChunkTimeout()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || m_isCancelled) {
        return;
    }
    
    qWarning() << "Chunk timeout:" << m_currentChunkIndex;
    
    // Add to failed chunks
    m_failedChunks.insert(m_currentChunkIndex);
    
    // Increment retry count
    int retryCount = m_chunkRetries.value(m_currentChunkIndex, 0) + 1;
    m_chunkRetries[m_currentChunkIndex] = retryCount;
    
    if (retryCount >= MAX_CHUNK_RETRIES) {
        QString error = QString("Chunk %1 failed after %2 retries").arg(m_currentChunkIndex).arg(MAX_CHUNK_RETRIES);
        qWarning() << error;
        
        locker.unlock();
        emit transferFailed(error);
        return;
    }
    
    // Schedule retry
    int delay = RETRY_DELAY_BASE * (1 << (retryCount - 1)); // Exponential backoff
    m_retryTimer->setInterval(delay);
    m_retryTimer->start();
    
    qDebug() << "Scheduling chunk retry in" << delay << "ms (attempt" << retryCount << ")";
}

void FileTransferWorker::retryCurrentChunk()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || m_isCancelled) {
        return;
    }
    
    qDebug() << "Retrying chunk:" << m_currentChunkIndex;
    
    // Remove from failed chunks
    m_failedChunks.remove(m_currentChunkIndex);
    
    locker.unlock();
    
    // Retry the current chunk
    if (m_session && m_session->getRequest().type == TransferType::Upload) {
        sendChunk(m_currentChunkIndex);
    } else {
        requestChunk(m_currentChunkIndex);
    }
}

void FileTransferWorker::onSessionStatusChanged(TransferStatus status)
{
    if (status == TransferStatus::Paused) {
        pauseTransfer();
    } else if (status == TransferStatus::InProgress) {
        resumeTransfer();
    } else if (status == TransferStatus::Cancelled) {
        cancelTransfer();
    }
}

void FileTransferWorker::processUpload()
{
    if (!m_session || m_totalChunks == 0) {
        emit transferFailed("Invalid upload session");
        return;
    }
    
    qDebug() << "Processing upload:" << m_totalChunks << "chunks";
    
    // Start with first chunk
    m_currentChunkIndex = 0;
    sendChunk(m_currentChunkIndex);
}

void FileTransferWorker::processDownload()
{
    if (!m_session) {
        emit transferFailed("Invalid download session");
        return;
    }
    
    qDebug() << "Processing download";
    
    // For downloads, we wait for the server to send chunks
    // The total chunks will be determined by the server
    // We just need to be ready to receive them
    
    // Start with requesting the first chunk
    m_currentChunkIndex = 0;
    requestChunk(m_currentChunkIndex);
}

void FileTransferWorker::sendChunk(int chunkIndex)
{
    if (!m_session || !checkCanContinue()) {
        return;
    }
    
    // Read chunk data
    QByteArray chunkData = m_session->readChunk(chunkIndex);
    if (chunkData.isEmpty() && chunkIndex < m_totalChunks - 1) {
        QString error = QString("Failed to read chunk %1").arg(chunkIndex);
        qWarning() << error;
        emit transferFailed(error);
        return;
    }
    
    // Calculate chunk checksum
    QCryptographicHash hash(QCryptographicHash::Sha256);
    hash.addData(chunkData);
    QString checksum = hash.result().toHex();
    
    // Create chunk object
    FileChunk chunk;
    chunk.transferId = m_session->getRequest().id;
    chunk.chunkIndex = chunkIndex;
    chunk.data = chunkData;
    chunk.checksum = checksum;
    chunk.isLast = (chunkIndex == m_totalChunks - 1);
    
    // Update current chunk index
    {
        QMutexLocker locker(&m_mutex);
        m_currentChunkIndex = chunkIndex;
    }
    
    // Start timeout timer
    m_chunkTimeoutTimer->start();
    
    // Send chunk
    emit chunkReady(chunk);
    
    qDebug() << "Sent chunk:" << chunkIndex << "(" << chunkData.size() << "bytes)";
}

void FileTransferWorker::requestChunk(int chunkIndex)
{
    if (!m_session || !checkCanContinue()) {
        return;
    }
    
    // Update current chunk index
    {
        QMutexLocker locker(&m_mutex);
        m_currentChunkIndex = chunkIndex;
    }
    
    // Start timeout timer
    m_chunkTimeoutTimer->start();
    
    // Request chunk from server (this would be handled by the manager)
    // For now, we'll emit a signal that the manager can handle
    emit chunkRequested(m_session->getRequest().id, chunkIndex);
    
    qDebug() << "Requested chunk:" << chunkIndex;
}

void FileTransferWorker::processReceivedChunk(const FileChunk &chunk)
{
    if (!m_session || !checkCanContinue()) {
        return;
    }
    
    // Verify chunk checksum
    QCryptographicHash hash(QCryptographicHash::Sha256);
    hash.addData(chunk.data);
    QString actualChecksum = hash.result().toHex();
    
    if (actualChecksum.compare(chunk.checksum, Qt::CaseInsensitive) != 0) {
        qWarning() << "Chunk checksum mismatch for chunk" << chunk.chunkIndex;
        
        // Add to failed chunks for retry
        QMutexLocker locker(&m_mutex);
        m_failedChunks.insert(chunk.chunkIndex);
        
        int retryCount = m_chunkRetries.value(chunk.chunkIndex, 0) + 1;
        m_chunkRetries[chunk.chunkIndex] = retryCount;
        
        if (retryCount >= MAX_CHUNK_RETRIES) {
            QString error = QString("Chunk %1 checksum failed after %2 retries").arg(chunk.chunkIndex).arg(MAX_CHUNK_RETRIES);
            emit transferFailed(error);
            return;
        }
        
        // Schedule retry
        int delay = RETRY_DELAY_BASE * (1 << (retryCount - 1));
        m_retryTimer->setInterval(delay);
        m_retryTimer->start();
        return;
    }
    
    // Write chunk to file
    if (!m_session->writeChunk(chunk.chunkIndex, chunk.data)) {
        QString error = QString("Failed to write chunk %1 to file").arg(chunk.chunkIndex);
        qWarning() << error;
        emit transferFailed(error);
        return;
    }
    
    // Stop timeout timer
    m_chunkTimeoutTimer->stop();
    
    // Mark chunk as completed
    {
        QMutexLocker locker(&m_mutex);
        if (!m_completedChunkIndices.contains(chunk.chunkIndex)) {
            m_completedChunkIndices.insert(chunk.chunkIndex);
            m_completedChunks++;
            
            // Update session progress
            m_session->updateChunkProgress(m_completedChunks);
        }
        
        // Remove from failed chunks
        m_failedChunks.remove(chunk.chunkIndex);
        m_chunkRetries.remove(chunk.chunkIndex);
    }
    
    qDebug() << "Processed chunk:" << chunk.chunkIndex << "(" << m_completedChunks << "/" << m_totalChunks << ")";
    
    // Check if this was the last chunk
    if (chunk.isLast) {
        // Update total chunks if not set
        if (m_totalChunks == 0) {
            m_totalChunks = chunk.chunkIndex + 1;
        }
        
        // Check if transfer is complete
        if (m_completedChunks >= m_totalChunks) {
            completeTransfer();
            return;
        }
    }
    
    // Continue with next chunk
    processNextChunk();
}

void FileTransferWorker::processNextChunk()
{
    if (!checkCanContinue()) {
        return;
    }
    
    QMutexLocker locker(&m_mutex);
    
    // Check if we have completed all chunks
    if (m_completedChunks >= m_totalChunks) {
        locker.unlock();
        completeTransfer();
        return;
    }
    
    // Find next chunk to process
    int nextChunk = -1;
    
    // First, retry any failed chunks
    if (!m_failedChunks.isEmpty()) {
        nextChunk = *m_failedChunks.begin();
    } else {
        // Find next unprocessed chunk
        for (int i = 0; i < m_totalChunks; ++i) {
            if (!m_completedChunkIndices.contains(i)) {
                nextChunk = i;
                break;
            }
        }
    }
    
    if (nextChunk == -1) {
        // All chunks processed
        locker.unlock();
        completeTransfer();
        return;
    }
    
    m_currentChunkIndex = nextChunk;
    locker.unlock();
    
    // Process the next chunk
    if (m_session->getRequest().type == TransferType::Upload) {
        sendChunk(nextChunk);
    } else {
        requestChunk(nextChunk);
    }
}

void FileTransferWorker::completeTransfer()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isRunning || m_isCancelled) {
        return;
    }
    
    qDebug() << "Completing transfer:" << (m_session ? m_session->getRequest().id : "unknown");
    
    m_isRunning = false;
    
    // Stop all timers
    m_progressTimer->stop();
    m_chunkTimeoutTimer->stop();
    m_retryTimer->stop();
    
    if (!m_session) {
        emit transferFailed("Session is null");
        return;
    }
    
    // Verify file checksum for downloads
    if (m_session->getRequest().type == TransferType::Download && !m_session->getRequest().checksum.isEmpty()) {
        if (!m_session->verifyChecksum(m_session->getRequest().checksum)) {
            QString error = m_session->getError();
            qWarning() << "File checksum verification failed:" << error;
            
            locker.unlock();
            emit transferFailed(error);
            return;
        }
    }
    
    // Close file
    m_session->closeFile();
    
    // Update session status
    m_session->setStatus(TransferStatus::Completed);
    
    locker.unlock();
    
    emit transferCompleted();
    
    qDebug() << "Transfer completed successfully:" << m_session->getRequest().id;
}

void FileTransferWorker::updateProgress()
{
    if (!m_session || !m_isRunning || m_isCancelled) {
        return;
    }
    
    FileTransferProgress progress = m_session->getProgress();
    emit progressUpdated(progress);
}

bool FileTransferWorker::checkCanContinue()
{
    QMutexLocker locker(&m_mutex);
    
    // Check if cancelled
    if (m_isCancelled || !m_isRunning) {
        return false;
    }
    
    // Wait if paused
    while (m_isPaused && !m_isCancelled) {
        m_pauseCondition.wait(&m_mutex);
    }
    
    return !m_isCancelled && m_isRunning;
}

bool FileTransferWorker::isRunning() const
{
    QMutexLocker locker(&m_mutex);
    return m_isRunning;
}

bool FileTransferWorker::isPaused() const
{
    QMutexLocker locker(&m_mutex);
    return m_isPaused;
}

bool FileTransferWorker::isCancelled() const
{
    QMutexLocker locker(&m_mutex);
    return m_isCancelled;
}

int FileTransferWorker::getCurrentChunkIndex() const
{
    QMutexLocker locker(&m_mutex);
    return m_currentChunkIndex;
}

int FileTransferWorker::getCompletedChunks() const
{
    QMutexLocker locker(&m_mutex);
    return m_completedChunks;
}

int FileTransferWorker::getTotalChunks() const
{
    QMutexLocker locker(&m_mutex);
    return m_totalChunks;
}

QSet<int> FileTransferWorker::getFailedChunks() const
{
    QMutexLocker locker(&m_mutex);
    return m_failedChunks;
}

#include "FileTransferWorker.moc"