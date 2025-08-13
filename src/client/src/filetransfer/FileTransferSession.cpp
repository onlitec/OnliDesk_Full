#include "FileTransferSession.h"
#include <QDebug>
#include <QDateTime>
#include <QFileInfo>
#include <QDir>
#include <QCryptographicHash>
#include <QTimer>

FileTransferSession::FileTransferSession(const FileTransferRequest &request, QObject *parent)
    : QObject(parent)
    , m_request(request)
    , m_status(TransferStatus::Pending)
    , m_progress()
    , m_startTime(QDateTime::currentDateTime())
    , m_endTime()
    , m_error()
    , m_retryCount(0)
    , m_maxRetries(3)
    , m_isPaused(false)
    , m_isCancelled(false)
    , m_file(nullptr)
    , m_totalChunks(0)
    , m_completedChunks(0)
    , m_lastProgressUpdate(QDateTime::currentDateTime())
    , m_speedCalculationTimer(new QTimer(this))
    , m_lastBytesTransferred(0)
{
    // Initialize progress
    m_progress.transferId = m_request.id;
    m_progress.totalBytes = m_request.fileSize;
    m_progress.bytesTransferred = 0;
    m_progress.percentage = 0.0;
    m_progress.speed = 0;
    m_progress.remainingTime = 0;
    
    // Setup speed calculation timer
    m_speedCalculationTimer->setInterval(1000); // Update every second
    connect(m_speedCalculationTimer, &QTimer::timeout, this, &FileTransferSession::updateSpeed);
    
    // Calculate total chunks
    if (m_request.fileSize > 0) {
        m_totalChunks = (m_request.fileSize + CHUNK_SIZE - 1) / CHUNK_SIZE;
    }
    
    qDebug() << "FileTransferSession created:" << m_request.id << m_request.filename;
}

FileTransferSession::~FileTransferSession()
{
    cleanup();
}

const FileTransferRequest& FileTransferSession::getRequest() const
{
    return m_request;
}

TransferStatus FileTransferSession::getStatus() const
{
    QMutexLocker locker(&m_mutex);
    return m_status;
}

void FileTransferSession::setStatus(TransferStatus status)
{
    {
        QMutexLocker locker(&m_mutex);
        if (m_status == status) {
            return;
        }
        
        TransferStatus oldStatus = m_status;
        m_status = status;
        
        // Update timestamps
        if (status == TransferStatus::InProgress && oldStatus == TransferStatus::Approved) {
            m_startTime = QDateTime::currentDateTime();
            m_speedCalculationTimer->start();
        } else if (status == TransferStatus::Completed || status == TransferStatus::Failed || 
                  status == TransferStatus::Cancelled) {
            m_endTime = QDateTime::currentDateTime();
            m_speedCalculationTimer->stop();
        } else if (status == TransferStatus::Paused) {
            m_speedCalculationTimer->stop();
        } else if (status == TransferStatus::InProgress && oldStatus == TransferStatus::Paused) {
            m_speedCalculationTimer->start();
        }
    }
    
    emit statusChanged(status);
    
    qDebug() << "Transfer status changed:" << m_request.id << statusToString(status);
}

FileTransferProgress FileTransferSession::getProgress() const
{
    QMutexLocker locker(&m_mutex);
    return m_progress;
}

void FileTransferSession::updateProgress(qint64 bytesTransferred)
{
    {
        QMutexLocker locker(&m_mutex);
        
        m_progress.bytesTransferred = bytesTransferred;
        
        if (m_progress.totalBytes > 0) {
            m_progress.percentage = (static_cast<double>(bytesTransferred) / m_progress.totalBytes) * 100.0;
        }
        
        m_lastProgressUpdate = QDateTime::currentDateTime();
    }
    
    emit progressUpdated(m_progress);
}

void FileTransferSession::updateChunkProgress(int completedChunks)
{
    {
        QMutexLocker locker(&m_mutex);
        m_completedChunks = completedChunks;
        
        if (m_totalChunks > 0) {
            qint64 bytesTransferred = static_cast<qint64>(completedChunks) * CHUNK_SIZE;
            if (completedChunks == m_totalChunks && m_request.fileSize > 0) {
                bytesTransferred = m_request.fileSize; // Last chunk might be smaller
            }
            
            m_progress.bytesTransferred = qMin(bytesTransferred, m_request.fileSize);
            
            if (m_progress.totalBytes > 0) {
                m_progress.percentage = (static_cast<double>(m_progress.bytesTransferred) / m_progress.totalBytes) * 100.0;
            }
        }
        
        m_lastProgressUpdate = QDateTime::currentDateTime();
    }
    
    emit progressUpdated(m_progress);
}

QString FileTransferSession::getError() const
{
    QMutexLocker locker(&m_mutex);
    return m_error;
}

void FileTransferSession::setError(const QString &error)
{
    {
        QMutexLocker locker(&m_mutex);
        m_error = error;
    }
    
    setStatus(TransferStatus::Failed);
    
    qWarning() << "Transfer error:" << m_request.id << error;
}

QDateTime FileTransferSession::getStartTime() const
{
    QMutexLocker locker(&m_mutex);
    return m_startTime;
}

QDateTime FileTransferSession::getEndTime() const
{
    QMutexLocker locker(&m_mutex);
    return m_endTime;
}

qint64 FileTransferSession::getDuration() const
{
    QMutexLocker locker(&m_mutex);
    
    if (m_startTime.isValid()) {
        QDateTime endTime = m_endTime.isValid() ? m_endTime : QDateTime::currentDateTime();
        return m_startTime.msecsTo(endTime);
    }
    
    return 0;
}

qint64 FileTransferSession::getAverageSpeed() const
{
    QMutexLocker locker(&m_mutex);
    
    qint64 duration = getDuration();
    if (duration > 0 && m_progress.bytesTransferred > 0) {
        return (m_progress.bytesTransferred * 1000) / duration; // bytes per second
    }
    
    return 0;
}

bool FileTransferSession::isPaused() const
{
    QMutexLocker locker(&m_mutex);
    return m_isPaused;
}

void FileTransferSession::setPaused(bool paused)
{
    {
        QMutexLocker locker(&m_mutex);
        m_isPaused = paused;
    }
    
    if (paused) {
        setStatus(TransferStatus::Paused);
    } else {
        setStatus(TransferStatus::InProgress);
    }
}

bool FileTransferSession::isCancelled() const
{
    QMutexLocker locker(&m_mutex);
    return m_isCancelled;
}

void FileTransferSession::setCancelled(bool cancelled)
{
    {
        QMutexLocker locker(&m_mutex);
        m_isCancelled = cancelled;
    }
    
    if (cancelled) {
        setStatus(TransferStatus::Cancelled);
    }
}

int FileTransferSession::getRetryCount() const
{
    QMutexLocker locker(&m_mutex);
    return m_retryCount;
}

void FileTransferSession::incrementRetryCount()
{
    QMutexLocker locker(&m_mutex);
    m_retryCount++;
}

bool FileTransferSession::canRetry() const
{
    QMutexLocker locker(&m_mutex);
    return m_retryCount < m_maxRetries;
}

void FileTransferSession::setMaxRetries(int maxRetries)
{
    QMutexLocker locker(&m_mutex);
    m_maxRetries = maxRetries;
}

bool FileTransferSession::openFile()
{
    QMutexLocker locker(&m_mutex);
    
    if (m_file) {
        return true; // Already open
    }
    
    m_file = std::make_unique<QFile>(m_request.localPath);
    
    QIODevice::OpenMode mode;
    if (m_request.type == TransferType::Upload) {
        mode = QIODevice::ReadOnly;
    } else {
        mode = QIODevice::WriteOnly | QIODevice::Truncate;
        
        // Ensure directory exists for downloads
        QFileInfo fileInfo(m_request.localPath);
        QDir dir = fileInfo.absoluteDir();
        if (!dir.exists()) {
            if (!dir.mkpath(".")) {
                m_error = QString("Failed to create directory: %1").arg(dir.absolutePath());
                return false;
            }
        }
    }
    
    if (!m_file->open(mode)) {
        m_error = QString("Failed to open file: %1 - %2").arg(m_request.localPath, m_file->errorString());
        m_file.reset();
        return false;
    }
    
    return true;
}

void FileTransferSession::closeFile()
{
    QMutexLocker locker(&m_mutex);
    
    if (m_file) {
        m_file->close();
        m_file.reset();
    }
}

QByteArray FileTransferSession::readChunk(int chunkIndex)
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_file || !m_file->isOpen()) {
        qWarning() << "File not open for reading";
        return QByteArray();
    }
    
    qint64 offset = static_cast<qint64>(chunkIndex) * CHUNK_SIZE;
    qint64 remainingBytes = m_request.fileSize - offset;
    int chunkSize = static_cast<int>(qMin(static_cast<qint64>(CHUNK_SIZE), remainingBytes));
    
    if (chunkSize <= 0) {
        return QByteArray();
    }
    
    if (!m_file->seek(offset)) {
        qWarning() << "Failed to seek to position" << offset;
        return QByteArray();
    }
    
    QByteArray data = m_file->read(chunkSize);
    if (data.size() != chunkSize) {
        qWarning() << "Failed to read expected chunk size. Expected:" << chunkSize << "Got:" << data.size();
        return QByteArray();
    }
    
    return data;
}

bool FileTransferSession::writeChunk(int chunkIndex, const QByteArray &data)
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_file || !m_file->isOpen()) {
        qWarning() << "File not open for writing";
        return false;
    }
    
    qint64 offset = static_cast<qint64>(chunkIndex) * CHUNK_SIZE;
    
    if (!m_file->seek(offset)) {
        qWarning() << "Failed to seek to position" << offset;
        return false;
    }
    
    qint64 bytesWritten = m_file->write(data);
    if (bytesWritten != data.size()) {
        qWarning() << "Failed to write complete chunk. Expected:" << data.size() << "Written:" << bytesWritten;
        return false;
    }
    
    m_file->flush();
    return true;
}

QString FileTransferSession::calculateFileChecksum()
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_file || !m_file->isOpen()) {
        qWarning() << "File not open for checksum calculation";
        return QString();
    }
    
    qint64 originalPos = m_file->pos();
    m_file->seek(0);
    
    QCryptographicHash hash(QCryptographicHash::Sha256);
    
    const int bufferSize = 64 * 1024; // 64KB buffer
    QByteArray buffer;
    
    while (!m_file->atEnd()) {
        buffer = m_file->read(bufferSize);
        hash.addData(buffer);
    }
    
    m_file->seek(originalPos);
    
    return hash.result().toHex();
}

bool FileTransferSession::verifyChecksum(const QString &expectedChecksum)
{
    QString actualChecksum = calculateFileChecksum();
    
    if (actualChecksum.isEmpty()) {
        m_error = "Failed to calculate file checksum";
        return false;
    }
    
    if (actualChecksum.compare(expectedChecksum, Qt::CaseInsensitive) != 0) {
        m_error = QString("Checksum mismatch. Expected: %1, Actual: %2")
                 .arg(expectedChecksum, actualChecksum);
        return false;
    }
    
    return true;
}

int FileTransferSession::getTotalChunks() const
{
    QMutexLocker locker(&m_mutex);
    return m_totalChunks;
}

int FileTransferSession::getCompletedChunks() const
{
    QMutexLocker locker(&m_mutex);
    return m_completedChunks;
}

double FileTransferSession::getCompletionPercentage() const
{
    QMutexLocker locker(&m_mutex);
    return m_progress.percentage;
}

QJsonObject FileTransferSession::toJson() const
{
    QMutexLocker locker(&m_mutex);
    
    QJsonObject obj;
    obj["id"] = m_request.id;
    obj["session_id"] = m_request.sessionId;
    obj["filename"] = m_request.filename;
    obj["file_size"] = m_request.fileSize;
    obj["local_path"] = m_request.localPath;
    obj["checksum"] = m_request.checksum;
    obj["type"] = (m_request.type == TransferType::Upload) ? "upload" : "download";
    obj["technician"] = m_request.technician;
    obj["status"] = statusToString(m_status);
    obj["progress"] = m_progress.percentage;
    obj["bytes_transferred"] = m_progress.bytesTransferred;
    obj["speed"] = m_progress.speed;
    obj["remaining_time"] = m_progress.remainingTime;
    obj["start_time"] = m_startTime.toString(Qt::ISODate);
    obj["end_time"] = m_endTime.toString(Qt::ISODate);
    obj["duration"] = getDuration();
    obj["error"] = m_error;
    obj["retry_count"] = m_retryCount;
    obj["is_paused"] = m_isPaused;
    obj["is_cancelled"] = m_isCancelled;
    obj["total_chunks"] = m_totalChunks;
    obj["completed_chunks"] = m_completedChunks;
    
    return obj;
}

void FileTransferSession::fromJson(const QJsonObject &obj)
{
    QMutexLocker locker(&m_mutex);
    
    // Update progress information
    m_progress.percentage = obj["progress"].toDouble();
    m_progress.bytesTransferred = obj["bytes_transferred"].toVariant().toLongLong();
    m_progress.speed = obj["speed"].toVariant().toLongLong();
    m_progress.remainingTime = obj["remaining_time"].toVariant().toLongLong();
    
    // Update session state
    m_error = obj["error"].toString();
    m_retryCount = obj["retry_count"].toInt();
    m_isPaused = obj["is_paused"].toBool();
    m_isCancelled = obj["is_cancelled"].toBool();
    m_completedChunks = obj["completed_chunks"].toInt();
    
    // Update timestamps
    QString startTimeStr = obj["start_time"].toString();
    if (!startTimeStr.isEmpty()) {
        m_startTime = QDateTime::fromString(startTimeStr, Qt::ISODate);
    }
    
    QString endTimeStr = obj["end_time"].toString();
    if (!endTimeStr.isEmpty()) {
        m_endTime = QDateTime::fromString(endTimeStr, Qt::ISODate);
    }
    
    // Update status (this will emit signals)
    QString statusStr = obj["status"].toString();
    TransferStatus status = stringToStatus(statusStr);
    if (status != m_status) {
        setStatus(status);
    }
}

void FileTransferSession::reset()
{
    QMutexLocker locker(&m_mutex);
    
    // Reset progress
    m_progress.bytesTransferred = 0;
    m_progress.percentage = 0.0;
    m_progress.speed = 0;
    m_progress.remainingTime = 0;
    
    // Reset state
    m_status = TransferStatus::Pending;
    m_error.clear();
    m_retryCount = 0;
    m_isPaused = false;
    m_isCancelled = false;
    m_completedChunks = 0;
    
    // Reset timestamps
    m_startTime = QDateTime();
    m_endTime = QDateTime();
    
    // Close file
    closeFile();
    
    m_speedCalculationTimer->stop();
}

void FileTransferSession::cleanup()
{
    closeFile();
    
    if (m_speedCalculationTimer) {
        m_speedCalculationTimer->stop();
    }
    
    // Clean up temporary files if transfer was cancelled or failed
    if ((m_status == TransferStatus::Cancelled || m_status == TransferStatus::Failed) &&
        m_request.type == TransferType::Download) {
        
        QFile tempFile(m_request.localPath);
        if (tempFile.exists()) {
            if (!tempFile.remove()) {
                qWarning() << "Failed to remove temporary file:" << m_request.localPath;
            }
        }
    }
}

void FileTransferSession::updateSpeed()
{
    QMutexLocker locker(&m_mutex);
    
    qint64 currentBytes = m_progress.bytesTransferred;
    qint64 bytesDiff = currentBytes - m_lastBytesTransferred;
    
    // Calculate speed (bytes per second)
    m_progress.speed = bytesDiff; // Timer fires every second
    
    // Calculate remaining time
    if (m_progress.speed > 0 && m_progress.totalBytes > 0) {
        qint64 remainingBytes = m_progress.totalBytes - currentBytes;
        m_progress.remainingTime = remainingBytes / m_progress.speed;
    } else {
        m_progress.remainingTime = 0;
    }
    
    m_lastBytesTransferred = currentBytes;
    
    emit progressUpdated(m_progress);
}

QString FileTransferSession::statusToString(TransferStatus status)
{
    switch (status) {
        case TransferStatus::Pending: return "pending";
        case TransferStatus::Approved: return "approved";
        case TransferStatus::Rejected: return "rejected";
        case TransferStatus::InProgress: return "in_progress";
        case TransferStatus::Paused: return "paused";
        case TransferStatus::Completed: return "completed";
        case TransferStatus::Failed: return "failed";
        case TransferStatus::Cancelled: return "cancelled";
        default: return "unknown";
    }
}

TransferStatus FileTransferSession::stringToStatus(const QString &statusStr)
{
    if (statusStr == "pending") return TransferStatus::Pending;
    if (statusStr == "approved") return TransferStatus::Approved;
    if (statusStr == "rejected") return TransferStatus::Rejected;
    if (statusStr == "in_progress") return TransferStatus::InProgress;
    if (statusStr == "paused") return TransferStatus::Paused;
    if (statusStr == "completed") return TransferStatus::Completed;
    if (statusStr == "failed") return TransferStatus::Failed;
    if (statusStr == "cancelled") return TransferStatus::Cancelled;
    return TransferStatus::Pending;
}

#include "FileTransferSession.moc"