#include "FileTransferManager.h"
#include "ApprovalDialog.h"
#include <QJsonObject>
#include <QJsonDocument>
#include <QJsonArray>
#include <QFileInfo>
#include <QDir>
#include <QStandardPaths>
#include <QCryptographicHash>
#include <QRandomGenerator>
#include <QDebug>
#include <QApplication>
#include <QMessageBox>
#include <QProgressDialog>
#include <QMimeDatabase>
#include <QMimeType>
#include <QSettings>
#include <QMutexLocker>

// Constants
static const int DEFAULT_CHUNK_SIZE = 64 * 1024; // 64KB
static const int DEFAULT_MAX_CONCURRENT = 3;
static const int PING_INTERVAL = 30000; // 30 seconds
static const int RECONNECT_INTERVAL = 5000; // 5 seconds
static const qint64 MAX_FILE_SIZE = 100 * 1024 * 1024; // 100MB

FileTransferManager::FileTransferManager(QObject *parent)
    : QObject(parent)
    , m_webSocket(std::make_unique<QWebSocket>())
    , m_isConnected(false)
    , m_pingTimer(std::make_unique<QTimer>(this))
    , m_reconnectTimer(std::make_unique<QTimer>(this))
    , m_reconnectAttempts(0)
    , m_chunkSize(DEFAULT_CHUNK_SIZE)
    , m_maxConcurrentTransfers(DEFAULT_MAX_CONCURRENT)
    , m_encryptionEnabled(true)
    , m_compressionEnabled(false)
    , m_maxFileSize(MAX_FILE_SIZE)
    , m_autoApprovalEnabled(false)
    , m_approvalTimeout(30)
    , m_rememberDecisionEnabled(true)
    , m_settings(new QSettings("OnliDesk", "FileTransfer", this))
    , m_networkManager(std::make_unique<QNetworkAccessManager>(this))
{
    setupWebSocket();
    
    // Setup ping timer
    m_pingTimer->setInterval(PING_INTERVAL);
    m_pingTimer->setSingleShot(false);
    connect(m_pingTimer.get(), &QTimer::timeout, this, &FileTransferManager::onPingTimer);
    
    // Setup reconnect timer
    m_reconnectTimer->setInterval(RECONNECT_INTERVAL);
    m_reconnectTimer->setSingleShot(true);
    connect(m_reconnectTimer.get(), &QTimer::timeout, this, [this]() {
        if (m_reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
            m_reconnectAttempts++;
            qDebug() << "Attempting to reconnect (" << m_reconnectAttempts << "/" << MAX_RECONNECT_ATTEMPTS << ")";
            m_webSocket->open(QUrl(m_serverUrl));
        } else {
            qWarning() << "Max reconnection attempts reached";
            emit connectionError("Failed to reconnect after multiple attempts");
        }
    });
    
    // Setup default allowed file extensions
    m_allowedExtensions << ".txt" << ".pdf" << ".doc" << ".docx" << ".xls" << ".xlsx"
                       << ".zip" << ".rar" << ".jpg" << ".png" << ".gif" << ".bmp"
                       << ".ppt" << ".pptx" << ".csv" << ".rtf" << ".odt" << ".ods";
    
    // Load settings from registry/config
    loadSettings();
}

FileTransferManager::~FileTransferManager()
{
    disconnectFromServer();
}

void FileTransferManager::setupWebSocket()
{
    // Connect WebSocket signals
    connect(m_webSocket.get(), &QWebSocket::connected, this, &FileTransferManager::onWebSocketConnected);
    connect(m_webSocket.get(), &QWebSocket::disconnected, this, &FileTransferManager::onWebSocketDisconnected);
    connect(m_webSocket.get(), QOverload<QAbstractSocket::SocketError>::of(&QWebSocket::error),
            this, &FileTransferManager::onWebSocketError);
    connect(m_webSocket.get(), &QWebSocket::textMessageReceived, this, &FileTransferManager::onWebSocketTextMessageReceived);
    connect(m_webSocket.get(), &QWebSocket::binaryMessageReceived, this, &FileTransferManager::onWebSocketBinaryMessageReceived);
}

void FileTransferManager::connectToServer(const QString &serverUrl)
{
    m_serverUrl = serverUrl;
    m_reconnectAttempts = 0;
    
    qDebug() << "Connecting to file transfer server:" << serverUrl;
    m_webSocket->open(QUrl(serverUrl));
}

void FileTransferManager::disconnectFromServer()
{
    if (m_isConnected) {
        qDebug() << "Disconnecting from file transfer server";
        
        // Cancel all active transfers
        QStringList activeTransfers = getActiveTransfers();
        for (const QString &transferId : activeTransfers) {
            cancelTransfer(transferId);
        }
        
        m_pingTimer->stop();
        m_reconnectTimer->stop();
        m_webSocket->close();
    }
}

bool FileTransferManager::isConnected() const
{
    return m_isConnected;
}

QString FileTransferManager::requestFileUpload(const QString &filePath, const QString &sessionId, const QString &technician)
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isConnected) {
        qWarning() << "Cannot request file upload: not connected to server";
        return QString();
    }
    
    // Validate file
    QString errorMessage;
    if (!validateFile(filePath, errorMessage)) {
        qWarning() << "File validation failed:" << errorMessage;
        emit transferFailed("", errorMessage);
        return QString();
    }
    
    // Prepare upload request
    FileTransferRequest request;
    if (!prepareFileForUpload(filePath, request)) {
        qWarning() << "Failed to prepare file for upload";
        return QString();
    }
    
    request.id = generateTransferId();
    request.sessionId = sessionId;
    request.technician = technician;
    request.type = TransferType::Upload;
    
    // Create transfer session
    auto session = std::make_unique<FileTransferSession>(request, this);
    connect(session.get(), &FileTransferSession::statusChanged, this, [this, request](TransferStatus status) {
        if (status == TransferStatus::Approved) {
            startTransfer(request.id);
        }
    });
    
    m_transferSessions[request.id] = std::move(session);
    
    // Send request to server
    QJsonObject message = createControlMessage("file_transfer_request");
    message["id"] = request.id;
    message["session_id"] = request.sessionId;
    message["filename"] = request.filename;
    message["file_size"] = request.fileSize;
    message["checksum"] = request.checksum;
    message["type"] = "upload";
    message["technician"] = request.technician;
    
    sendControlMessage(message);
    
    emit transferRequested(request.id, request);
    
    qDebug() << "File upload requested:" << request.filename << "(" << request.fileSize << "bytes)";
    return request.id;
}

QString FileTransferManager::requestFileDownload(const QString &filename, const QString &sessionId, const QString &technician, const QString &savePath)
{
    QMutexLocker locker(&m_mutex);
    
    if (!m_isConnected) {
        qWarning() << "Cannot request file download: not connected to server";
        return QString();
    }
    
    // Prepare download request
    FileTransferRequest request;
    if (!prepareFileForDownload(filename, savePath, request)) {
        qWarning() << "Failed to prepare file for download";
        return QString();
    }
    
    request.id = generateTransferId();
    request.sessionId = sessionId;
    request.technician = technician;
    request.type = TransferType::Download;
    
    // Create transfer session
    auto session = std::make_unique<FileTransferSession>(request, this);
    connect(session.get(), &FileTransferSession::statusChanged, this, [this, request](TransferStatus status) {
        if (status == TransferStatus::Approved) {
            startTransfer(request.id);
        }
    });
    
    m_transferSessions[request.id] = std::move(session);
    
    // Send request to server
    QJsonObject message = createControlMessage("file_transfer_request");
    message["id"] = request.id;
    message["session_id"] = request.sessionId;
    message["filename"] = request.filename;
    message["file_size"] = request.fileSize;
    message["type"] = "download";
    message["technician"] = request.technician;
    
    sendControlMessage(message);
    
    emit transferRequested(request.id, request);
    
    qDebug() << "File download requested:" << request.filename;
    return request.id;
}

void FileTransferManager::pauseTransfer(const QString &transferId)
{
    QMutexLocker locker(&m_mutex);
    
    if (auto worker = m_transferWorkers.value(transferId)) {
        worker->pauseTransfer();
    }
    
    QJsonObject message = createControlMessage("transfer_control");
    message["transfer_id"] = transferId;
    message["action"] = "pause";
    sendControlMessage(message);
    
    qDebug() << "Transfer paused:" << transferId;
}

void FileTransferManager::resumeTransfer(const QString &transferId)
{
    QMutexLocker locker(&m_mutex);
    
    if (auto worker = m_transferWorkers.value(transferId)) {
        worker->resumeTransfer();
    }
    
    QJsonObject message = createControlMessage("transfer_control");
    message["transfer_id"] = transferId;
    message["action"] = "resume";
    sendControlMessage(message);
    
    qDebug() << "Transfer resumed:" << transferId;
}

void FileTransferManager::cancelTransfer(const QString &transferId)
{
    QMutexLocker locker(&m_mutex);
    
    // Cancel worker
    if (auto worker = m_transferWorkers.value(transferId)) {
        worker->cancelTransfer();
    }
    
    // Stop thread
    if (auto thread = m_transferThreads.value(transferId)) {
        thread->quit();
        thread->wait(5000); // Wait up to 5 seconds
    }
    
    // Update session status
    if (auto session = m_transferSessions.value(transferId)) {
        session->setStatus(TransferStatus::Cancelled);
    }
    
    // Clean up
    m_transferWorkers.remove(transferId);
    m_transferThreads.remove(transferId);
    
    // Notify server
    QJsonObject message = createControlMessage("transfer_control");
    message["transfer_id"] = transferId;
    message["action"] = "cancel";
    sendControlMessage(message);
    
    emit transferCancelled(transferId);
    
    qDebug() << "Transfer cancelled:" << transferId;
}

FileTransferProgress FileTransferManager::getTransferProgress(const QString &transferId) const
{
    QMutexLocker locker(&m_mutex);
    
    if (auto session = m_transferSessions.value(transferId)) {
        return session->getProgress();
    }
    
    return FileTransferProgress{};
}

QStringList FileTransferManager::getActiveTransfers() const
{
    QMutexLocker locker(&m_mutex);
    
    QStringList activeTransfers;
    for (auto it = m_transferSessions.begin(); it != m_transferSessions.end(); ++it) {
        TransferStatus status = it.value()->getStatus();
        if (status == TransferStatus::Pending || status == TransferStatus::Approved ||
            status == TransferStatus::InProgress || status == TransferStatus::Paused) {
            activeTransfers.append(it.key());
        }
    }
    
    return activeTransfers;
}

void FileTransferManager::setChunkSize(int size)
{
    m_chunkSize = qMax(1024, qMin(size, 1024 * 1024)); // Between 1KB and 1MB
}

void FileTransferManager::setMaxConcurrentTransfers(int max)
{
    m_maxConcurrentTransfers = qMax(1, qMin(max, 10)); // Between 1 and 10
}

void FileTransferManager::setEncryptionEnabled(bool enabled)
{
    m_encryptionEnabled = enabled;
}

void FileTransferManager::setCompressionEnabled(bool enabled)
{
    m_compressionEnabled = enabled;
}

bool FileTransferManager::validateFile(const QString &filePath, QString &errorMessage)
{
    QFileInfo fileInfo(filePath);
    
    // Check if file exists
    if (!fileInfo.exists()) {
        errorMessage = "File does not exist";
        return false;
    }
    
    // Check if it's a file (not directory)
    if (!fileInfo.isFile()) {
        errorMessage = "Path is not a file";
        return false;
    }
    
    // Check file size
    if (fileInfo.size() > m_maxFileSize) {
        errorMessage = QString("File size (%1 MB) exceeds maximum allowed size (%2 MB)")
                      .arg(fileInfo.size() / (1024 * 1024))
                      .arg(m_maxFileSize / (1024 * 1024));
        return false;
    }
    
    // Check file extension
    QString extension = fileInfo.suffix().toLower();
    if (!extension.isEmpty() && !m_allowedExtensions.contains("." + extension)) {
        errorMessage = QString("File extension '.%1' is not allowed").arg(extension);
        return false;
    }
    
    // Check MIME type
    QMimeDatabase mimeDb;
    QMimeType mimeType = mimeDb.mimeTypeForFile(filePath);
    if (mimeType.name().startsWith("application/x-executable")) {
        errorMessage = "Executable files are not allowed";
        return false;
    }
    
    return true;
}

QString FileTransferManager::calculateFileChecksum(const QString &filePath)
{
    QFile file(filePath);
    if (!file.open(QIODevice::ReadOnly)) {
        qWarning() << "Failed to open file for checksum calculation:" << filePath;
        return QString();
    }
    
    QCryptographicHash hash(QCryptographicHash::Sha256);
    
    const int bufferSize = 64 * 1024; // 64KB buffer
    QByteArray buffer;
    
    while (!file.atEnd()) {
        buffer = file.read(bufferSize);
        hash.addData(buffer);
    }
    
    return hash.result().toHex();
}

void FileTransferManager::onSessionRegistered(const QString &sessionId)
{
    m_sessionId = sessionId;
    registerSession();
}

void FileTransferManager::onTransferApprovalReceived(const QString &transferId, bool approved, const QString &message)
{
    QMutexLocker locker(&m_mutex);
    
    if (auto session = m_transferSessions.value(transferId)) {
        if (approved) {
            session->setStatus(TransferStatus::Approved);
            emit transferApproved(transferId);
        } else {
            session->setStatus(TransferStatus::Rejected);
            session->setError(message);
            emit transferRejected(transferId, message);
        }
    }
}

// WebSocket event handlers
void FileTransferManager::onWebSocketConnected()
{
    qDebug() << "Connected to file transfer server";
    
    m_isConnected = true;
    m_reconnectAttempts = 0;
    m_reconnectTimer->stop();
    
    // Start ping timer
    m_pingTimer->start();
    
    // Register session if we have one
    if (!m_sessionId.isEmpty()) {
        registerSession();
    }
    
    emit connected();
}

void FileTransferManager::onWebSocketDisconnected()
{
    qDebug() << "Disconnected from file transfer server";
    
    m_isConnected = false;
    m_pingTimer->stop();
    
    // Start reconnection attempts
    if (m_reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
        m_reconnectTimer->start();
    }
    
    emit disconnected();
}

void FileTransferManager::onWebSocketError(QAbstractSocket::SocketError error)
{
    QString errorString = m_webSocket->errorString();
    qWarning() << "WebSocket error:" << error << errorString;
    
    emit connectionError(errorString);
}

void FileTransferManager::onWebSocketTextMessageReceived(const QString &message)
{
    QJsonParseError parseError;
    QJsonDocument doc = QJsonDocument::fromJson(message.toUtf8(), &parseError);
    
    if (parseError.error != QJsonParseError::NoError) {
        qWarning() << "Failed to parse JSON message:" << parseError.errorString();
        return;
    }
    
    QJsonObject obj = doc.object();
    handleControlMessage(obj);
}

void FileTransferManager::onWebSocketBinaryMessageReceived(const QByteArray &data)
{
    // Handle binary file chunks
    if (data.size() < 4) {
        qWarning() << "Binary message too short";
        return;
    }
    
    // Extract header length (first 4 bytes)
    int headerLength = (static_cast<unsigned char>(data[0]) << 24) |
                      (static_cast<unsigned char>(data[1]) << 16) |
                      (static_cast<unsigned char>(data[2]) << 8) |
                      static_cast<unsigned char>(data[3]);
    
    if (headerLength > data.size() - 4) {
        qWarning() << "Invalid header length in binary message";
        return;
    }
    
    // Parse header
    QByteArray headerData = data.mid(4, headerLength);
    QByteArray chunkData = data.mid(4 + headerLength);
    
    QJsonParseError parseError;
    QJsonDocument headerDoc = QJsonDocument::fromJson(headerData, &parseError);
    
    if (parseError.error != QJsonParseError::NoError) {
        qWarning() << "Failed to parse chunk header:" << parseError.errorString();
        return;
    }
    
    QJsonObject header = headerDoc.object();
    
    FileChunk chunk;
    chunk.transferId = header["transfer_id"].toString();
    chunk.chunkIndex = header["chunk_index"].toInt();
    chunk.data = chunkData;
    chunk.checksum = header["checksum"].toString();
    chunk.isLast = header["is_last"].toBool();
    
    // Process chunk with appropriate worker
    if (auto worker = m_transferWorkers.value(chunk.transferId)) {
        // This would be handled by the worker's chunk processing logic
        emit chunkReceived(chunk.transferId, chunk.chunkIndex);
    }
}

void FileTransferManager::onPingTimer()
{
    sendPing();
}

void FileTransferManager::onTransferWorkerFinished()
{
    // Clean up finished worker
    FileTransferWorker *worker = qobject_cast<FileTransferWorker*>(sender());
    if (worker) {
        // Find and remove the worker
        for (auto it = m_transferWorkers.begin(); it != m_transferWorkers.end(); ++it) {
            if (it.value().get() == worker) {
                QString transferId = it.key();
                
                // Stop and clean up thread
                if (auto thread = m_transferThreads.value(transferId)) {
                    thread->quit();
                    thread->wait(5000);
                }
                
                m_transferWorkers.remove(transferId);
                m_transferThreads.remove(transferId);
                break;
            }
        }
    }
}

// Private helper methods
void FileTransferManager::registerSession()
{
    if (!m_isConnected || m_sessionId.isEmpty()) {
        return;
    }
    
    QJsonObject message = createControlMessage("session_register");
    message["session_id"] = m_sessionId;
    message["role"] = "client";
    
    sendControlMessage(message);
    
    qDebug() << "Session registered:" << m_sessionId;
}

void FileTransferManager::sendPing()
{
    if (m_isConnected) {
        QJsonObject message = createControlMessage("ping");
        sendControlMessage(message);
    }
}

void FileTransferManager::handleControlMessage(const QJsonObject &message)
{
    QString type = message["type"].toString();
    
    if (type == "file_transfer_response") {
        handleTransferResponse(message);
    } else if (type == "transfer_status_update") {
        handleTransferStatusUpdate(message);
    } else if (type == "chunk_ack") {
        handleChunkAcknowledgment(message);
    } else if (type == "progress_response") {
        handleProgressResponse(message);
    } else if (type == "error") {
        handleErrorMessage(message);
    } else if (type == "pong") {
        // Pong received, connection is alive
    } else if (type == "transfer_request") {
        // Handle incoming transfer request from technician
        onTransferRequestReceived(message);
    } else {
        qDebug() << "Unknown message type:" << type;
    }
}

void FileTransferManager::handleTransferResponse(const QJsonObject &message)
{
    QString transferId = message["transfer_id"].toString();
    QString status = message["status"].toString();
    QString responseMessage = message["message"].toString();
    
    qDebug() << "Transfer response:" << transferId << status << responseMessage;
    
    if (auto session = m_transferSessions.value(transferId)) {
        if (status == "pending") {
            session->setStatus(TransferStatus::Pending);
        } else if (status == "approved") {
            session->setStatus(TransferStatus::Approved);
        } else if (status == "rejected") {
            session->setStatus(TransferStatus::Rejected);
            session->setError(responseMessage);
        }
    }
}

void FileTransferManager::handleTransferStatusUpdate(const QJsonObject &message)
{
    QString transferId = message["transfer_id"].toString();
    QString status = message["status"].toString();
    QString statusMessage = message["message"].toString();
    
    if (auto session = m_transferSessions.value(transferId)) {
        if (status == "approved") {
            session->setStatus(TransferStatus::Approved);
            emit transferApproved(transferId);
        } else if (status == "rejected") {
            session->setStatus(TransferStatus::Rejected);
            session->setError(statusMessage);
            emit transferRejected(transferId, statusMessage);
        }
    }
}

void FileTransferManager::handleChunkAcknowledgment(const QJsonObject &message)
{
    QString transferId = message["transfer_id"].toString();
    int chunkIndex = message["chunk_index"].toInt();
    
    emit chunkSent(transferId, chunkIndex);
    
    // Notify worker about acknowledgment
    if (auto worker = m_transferWorkers.value(transferId)) {
        QMetaObject::invokeMethod(worker.get(), "onChunkAcknowledged", 
                                 Qt::QueuedConnection, Q_ARG(int, chunkIndex));
    }
}

void FileTransferManager::handleProgressResponse(const QJsonObject &message)
{
    // Handle progress updates from server
    QJsonObject progressObj = message["progress"].toObject();
    
    FileTransferProgress progress;
    progress.transferId = progressObj["transfer_id"].toString();
    progress.bytesTransferred = progressObj["bytes_transferred"].toVariant().toLongLong();
    progress.totalBytes = progressObj["total_bytes"].toVariant().toLongLong();
    progress.percentage = progressObj["percentage"].toDouble();
    progress.speed = progressObj["speed"].toVariant().toLongLong();
    progress.remainingTime = progressObj["remaining_time"].toVariant().toLongLong();
    
    emit transferProgress(progress.transferId, progress);
}

void FileTransferManager::handleErrorMessage(const QJsonObject &message)
{
    QString error = message["error"].toString();
    QString errorMessage = message["message"].toString();
    
    qWarning() << "Server error:" << error << errorMessage;
    emit connectionError(errorMessage);
}

void FileTransferManager::startTransfer(const QString &transferId)
{
    QMutexLocker locker(&m_mutex);
    
    auto session = m_transferSessions.value(transferId);
    if (!session) {
        qWarning() << "Cannot start transfer: session not found" << transferId;
        return;
    }
    
    // Check if we've reached max concurrent transfers
    int activeCount = 0;
    for (const auto &worker : m_transferWorkers) {
        Q_UNUSED(worker)
        activeCount++;
    }
    
    if (activeCount >= m_maxConcurrentTransfers) {
        qWarning() << "Cannot start transfer: max concurrent transfers reached";
        return;
    }
    
    // Create worker and thread
    auto worker = std::make_unique<FileTransferWorker>(session.get(), this);
    auto thread = std::make_unique<QThread>();
    
    // Move worker to thread
    worker->moveToThread(thread.get());
    
    // Connect signals
    connect(thread.get(), &QThread::started, worker.get(), &FileTransferWorker::startTransfer);
    connect(worker.get(), &FileTransferWorker::transferCompleted, this, [this, transferId]() {
        emit transferCompleted(transferId, m_transferSessions[transferId]->getRequest().localPath);
    });
    connect(worker.get(), &FileTransferWorker::transferFailed, this, [this, transferId](const QString &error) {
        emit transferFailed(transferId, error);
    });
    connect(worker.get(), &FileTransferWorker::progressUpdated, this, [this, transferId](const FileTransferProgress &progress) {
        emit transferProgress(transferId, progress);
    });
    connect(worker.get(), &FileTransferWorker::chunkReady, this, [this](const FileChunk &chunk) {
        sendBinaryChunk(chunk);
    });
    
    // Store worker and thread
    m_transferWorkers[transferId] = std::move(worker);
    m_transferThreads[transferId] = std::move(thread);
    
    // Start thread
    m_transferThreads[transferId]->start();
    
    session->setStatus(TransferStatus::InProgress);
    emit transferStarted(transferId);
    
    qDebug() << "Transfer started:" << transferId;
}

QJsonObject FileTransferManager::createControlMessage(const QString &type, const QJsonObject &data)
{
    QJsonObject message;
    message["type"] = type;
    message["timestamp"] = QDateTime::currentDateTime().toString(Qt::ISODate);
    
    // Merge additional data
    for (auto it = data.begin(); it != data.end(); ++it) {
        message[it.key()] = it.value();
    }
    
    return message;
}

void FileTransferManager::sendControlMessage(const QJsonObject &message)
{
    if (!m_isConnected) {
        qWarning() << "Cannot send message: not connected";
        return;
    }
    
    QJsonDocument doc(message);
    m_webSocket->sendTextMessage(doc.toJson(QJsonDocument::Compact));
}

void FileTransferManager::sendBinaryChunk(const FileChunk &chunk)
{
    if (!m_isConnected) {
        qWarning() << "Cannot send chunk: not connected";
        return;
    }
    
    // Create chunk header
    QJsonObject header;
    header["transfer_id"] = chunk.transferId;
    header["chunk_index"] = chunk.chunkIndex;
    header["checksum"] = chunk.checksum;
    header["is_last"] = chunk.isLast;
    
    QJsonDocument headerDoc(header);
    QByteArray headerData = headerDoc.toJson(QJsonDocument::Compact);
    
    // Create binary message: [header_length(4 bytes)][header][chunk_data]
    QByteArray message;
    
    // Header length (4 bytes, big-endian)
    int headerLength = headerData.size();
    message.append(static_cast<char>((headerLength >> 24) & 0xFF));
    message.append(static_cast<char>((headerLength >> 16) & 0xFF));
    message.append(static_cast<char>((headerLength >> 8) & 0xFF));
    message.append(static_cast<char>(headerLength & 0xFF));
    
    // Header and chunk data
    message.append(headerData);
    message.append(chunk.data);
    
    m_webSocket->sendBinaryMessage(message);
}

QString FileTransferManager::generateTransferId()
{
    return QUuid::createUuid().toString(QUuid::WithoutBraces);
}

bool FileTransferManager::prepareFileForUpload(const QString &filePath, FileTransferRequest &request)
{
    QFileInfo fileInfo(filePath);
    
    request.filename = fileInfo.fileName();
    request.fileSize = fileInfo.size();
    request.localPath = filePath;
    request.checksum = calculateFileChecksum(filePath);
    
    if (request.checksum.isEmpty()) {
        qWarning() << "Failed to calculate file checksum";
        return false;
    }
    
    return true;
}

bool FileTransferManager::prepareFileForDownload(const QString &filename, const QString &savePath, FileTransferRequest &request)
{
    request.filename = filename;
    request.localPath = savePath;
    request.fileSize = 0; // Will be set by server
    
    // Ensure save directory exists
    QFileInfo saveInfo(savePath);
    QDir saveDir = saveInfo.absoluteDir();
    if (!saveDir.exists()) {
        if (!saveDir.mkpath(".")) {
            qWarning() << "Failed to create save directory:" << saveDir.absolutePath();
            return false;
        }
    }
    
    return true;
}

// Approval dialog settings
void FileTransferManager::setAutoApprovalEnabled(bool enabled)
{
    QMutexLocker locker(&m_mutex);
    m_autoApprovalEnabled = enabled;
    saveSettings();
}

bool FileTransferManager::isAutoApprovalEnabled() const
{
    QMutexLocker locker(&m_mutex);
    return m_autoApprovalEnabled;
}

void FileTransferManager::setApprovalTimeout(int seconds)
{
    QMutexLocker locker(&m_mutex);
    m_approvalTimeout = qMax(5, seconds); // Minimum 5 seconds
    saveSettings();
}

int FileTransferManager::getApprovalTimeout() const
{
    QMutexLocker locker(&m_mutex);
    return m_approvalTimeout;
}

void FileTransferManager::setRememberDecisionEnabled(bool enabled)
{
    QMutexLocker locker(&m_mutex);
    m_rememberDecisionEnabled = enabled;
    saveSettings();
}

bool FileTransferManager::isRememberDecisionEnabled() const
{
    QMutexLocker locker(&m_mutex);
    return m_rememberDecisionEnabled;
}

// Security settings
void FileTransferManager::addAllowedFileExtension(const QString &extension)
{
    QMutexLocker locker(&m_mutex);
    QString ext = extension.toLower();
    if (!ext.startsWith(".")) {
        ext.prepend(".");
    }
    if (!m_allowedExtensions.contains(ext)) {
        m_allowedExtensions.append(ext);
        saveSettings();
    }
}

void FileTransferManager::removeAllowedFileExtension(const QString &extension)
{
    QMutexLocker locker(&m_mutex);
    QString ext = extension.toLower();
    if (!ext.startsWith(".")) {
        ext.prepend(".");
    }
    if (m_allowedExtensions.removeAll(ext) > 0) {
        saveSettings();
    }
}

QStringList FileTransferManager::getAllowedFileExtensions() const
{
    QMutexLocker locker(&m_mutex);
    return m_allowedExtensions;
}

void FileTransferManager::setMaxFileSize(qint64 maxSize)
{
    QMutexLocker locker(&m_mutex);
    m_maxFileSize = qMax(1024LL, maxSize); // Minimum 1KB
    saveSettings();
}

qint64 FileTransferManager::getMaxFileSize() const
{
    QMutexLocker locker(&m_mutex);
    return m_maxFileSize;
}

// Approval and security methods
void FileTransferManager::showApprovalDialog(const FileTransferRequest &request)
{
    // Check if we have a remembered decision
    bool approved = false;
    if (m_rememberDecisionEnabled && checkRememberedDecision(request.id, approved)) {
        processApprovalDecision(request.id, approved, 
                              approved ? tr("Auto-approved (remembered)") : tr("Auto-rejected (remembered)"));
        return;
    }
    
    // Show approval dialog
    ApprovalDialog *dialog = new ApprovalDialog(request, qobject_cast<QWidget*>(parent()));
    
    // Configure dialog
    if (m_approvalTimeout > 0) {
        dialog->setAutoTimeout(m_approvalTimeout);
    }
    dialog->setRememberOptionEnabled(m_rememberDecisionEnabled);
    
    // Connect dialog signals
    connect(dialog, QOverload<int>::of(&QDialog::finished), this, &FileTransferManager::onApprovalDialogFinished);
    
    // Store dialog reference
    dialog->setProperty("transferId", request.id);
    
    // Show dialog
    dialog->show();
    dialog->raise();
    dialog->activateWindow();
}

bool FileTransferManager::isFileExtensionAllowed(const QString &filePath) const
{
    QFileInfo fileInfo(filePath);
    QString extension = "." + fileInfo.suffix().toLower();
    
    QMutexLocker locker(&m_mutex);
    return m_allowedExtensions.contains(extension);
}

bool FileTransferManager::isFileSizeValid(qint64 fileSize) const
{
    QMutexLocker locker(&m_mutex);
    return fileSize > 0 && fileSize <= m_maxFileSize;
}

bool FileTransferManager::checkRememberedDecision(const QString &transferId, bool &approved) const
{
    QMutexLocker locker(&m_mutex);
    if (m_rememberedDecisions.contains(transferId)) {
        approved = m_rememberedDecisions.value(transferId);
        return true;
    }
    return false;
}

void FileTransferManager::saveRememberedDecision(const QString &transferId, bool approved)
{
    QMutexLocker locker(&m_mutex);
    m_rememberedDecisions.insert(transferId, approved);
    
    // Save to persistent storage
    m_settings->beginGroup("RememberedDecisions");
    m_settings->setValue(transferId, approved);
    m_settings->endGroup();
    m_settings->sync();
}

void FileTransferManager::loadSettings()
{
    // Load approval settings
    m_autoApprovalEnabled = m_settings->value("AutoApproval/Enabled", false).toBool();
    m_approvalTimeout = m_settings->value("AutoApproval/Timeout", 30).toInt();
    m_rememberDecisionEnabled = m_settings->value("AutoApproval/RememberDecision", true).toBool();
    
    // Load security settings
    m_maxFileSize = m_settings->value("Security/MaxFileSize", MAX_FILE_SIZE).toLongLong();
    
    // Load allowed extensions (if saved)
    QStringList savedExtensions = m_settings->value("Security/AllowedExtensions").toStringList();
    if (!savedExtensions.isEmpty()) {
        m_allowedExtensions = savedExtensions;
    }
    
    // Load remembered decisions
    m_settings->beginGroup("RememberedDecisions");
    QStringList keys = m_settings->childKeys();
    for (const QString &key : keys) {
        bool approved = m_settings->value(key).toBool();
        m_rememberedDecisions.insert(key, approved);
    }
    m_settings->endGroup();
}

void FileTransferManager::saveSettings()
{
    // Save approval settings
    m_settings->setValue("AutoApproval/Enabled", m_autoApprovalEnabled);
    m_settings->setValue("AutoApproval/Timeout", m_approvalTimeout);
    m_settings->setValue("AutoApproval/RememberDecision", m_rememberDecisionEnabled);
    
    // Save security settings
    m_settings->setValue("Security/MaxFileSize", m_maxFileSize);
    m_settings->setValue("Security/AllowedExtensions", m_allowedExtensions);
    
    m_settings->sync();
}

// Approval and security slots
void FileTransferManager::onApprovalDialogFinished(int result)
{
    ApprovalDialog *dialog = qobject_cast<ApprovalDialog*>(sender());
    if (!dialog) {
        return;
    }
    
    QString transferId = dialog->property("transferId").toString();
    bool approved = dialog->isApproved();
    QString message = dialog->getMessage();
    
    // Save decision if requested
    if (dialog->shouldRememberDecision()) {
        saveRememberedDecision(transferId, approved);
    }
    
    // Process the decision
    processApprovalDecision(transferId, approved, message);
    
    // Clean up
    dialog->deleteLater();
}

void FileTransferManager::onTransferRequestReceived(const QJsonObject &request)
{
    FileTransferRequest transferRequest;
    transferRequest.id = request["transfer_id"].toString();
    transferRequest.filename = request["filename"].toString();
    transferRequest.fileSize = request["file_size"].toVariant().toLongLong();
    transferRequest.type = static_cast<TransferType>(request["type"].toInt());
    transferRequest.sessionId = request["session_id"].toString();
    transferRequest.technician = request["technician"].toString();
    transferRequest.checksum = request["checksum"].toString();
    
    // Store pending request
    m_pendingRequests.insert(transferRequest.id, transferRequest);
    
    // Validate file
    QString errorMessage;
    if (!isFileExtensionAllowed(transferRequest.filename)) {
        errorMessage = tr("File extension not allowed: %1").arg(QFileInfo(transferRequest.filename).suffix());
        emit fileValidationFailed(transferRequest.filename, errorMessage);
        processApprovalDecision(transferRequest.id, false, errorMessage);
        return;
    }
    
    if (!isFileSizeValid(transferRequest.fileSize)) {
        errorMessage = tr("File size exceeds maximum allowed: %1 bytes").arg(transferRequest.fileSize);
        emit fileValidationFailed(transferRequest.filename, errorMessage);
        processApprovalDecision(transferRequest.id, false, errorMessage);
        return;
    }
    
    // Check for auto-approval
    if (m_autoApprovalEnabled) {
        processApprovalDecision(transferRequest.id, true, tr("Auto-approved"));
        return;
    }
    
    // Show approval dialog
    emit transferApprovalRequested(transferRequest);
    showApprovalDialog(transferRequest);
}

void FileTransferManager::processApprovalDecision(const QString &transferId, bool approved, const QString &message)
{
    // Remove from pending requests
    m_pendingRequests.remove(transferId);
    
    // Send decision to server
    QJsonObject response = createControlMessage("transfer_approval");
    response["transfer_id"] = transferId;
    response["approved"] = approved;
    response["message"] = message;
    
    sendControlMessage(response);
    
    // Emit signal
    emit transferApprovalDecision(transferId, approved, message);
    
    // Log the decision
    qDebug() << "Transfer" << transferId << (approved ? "approved" : "rejected") << "with message:" << message;
}

#include "FileTransferManager.moc"