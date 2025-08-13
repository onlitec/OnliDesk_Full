#ifndef FILETRANSFERMANAGER_H
#define FILETRANSFERMANAGER_H

#include <QObject>
#include <QWebSocket>
#include <QFile>
#include <QTimer>
#include <QQueue>
#include <QMutex>
#include <QThread>
#include <QJsonObject>
#include <QJsonDocument>
#include <QCryptographicHash>
#include <QProgressBar>
#include <QLabel>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QSettings>
#include <memory>

class FileTransferSession;
class FileTransferWorker;
class ApprovalDialog;

// Transfer types
enum class TransferType {
    Upload,
    Download
};

// Transfer status
enum class TransferStatus {
    Pending,
    Approved,
    InProgress,
    Paused,
    Completed,
    Failed,
    Cancelled,
    Rejected
};

// File transfer request structure
struct FileTransferRequest {
    QString id;
    QString sessionId;
    QString filename;
    qint64 fileSize;
    QString checksum;
    TransferType type;
    QString technician;
    QString localPath;
    QString remotePath;
    QJsonObject metadata;
};

// File transfer progress structure
struct FileTransferProgress {
    QString transferId;
    qint64 bytesTransferred;
    qint64 totalBytes;
    double percentage;
    qint64 speed; // bytes per second
    qint64 remainingTime; // seconds
    TransferStatus status;
    QString errorMessage;
    QDateTime startTime;
    QDateTime lastUpdateTime;
};

// File chunk structure
struct FileChunk {
    QString transferId;
    int chunkIndex;
    QByteArray data;
    QString checksum;
    bool isLast;
};

class FileTransferManager : public QObject
{
    Q_OBJECT

public:
    explicit FileTransferManager(QObject *parent = nullptr);
    ~FileTransferManager();

    // Connection management
    void connectToServer(const QString &serverUrl);
    void disconnectFromServer();
    bool isConnected() const;

    // Transfer operations
    QString requestFileUpload(const QString &filePath, const QString &sessionId, const QString &technician);
    QString requestFileDownload(const QString &filename, const QString &sessionId, const QString &technician, const QString &savePath);
    
    // Transfer control
    void pauseTransfer(const QString &transferId);
    void resumeTransfer(const QString &transferId);
    void cancelTransfer(const QString &transferId);
    
    // Progress tracking
    FileTransferProgress getTransferProgress(const QString &transferId) const;
    QStringList getActiveTransfers() const;
    
    // Configuration
    void setChunkSize(int size);
    void setMaxConcurrentTransfers(int max);
    void setEncryptionEnabled(bool enabled);
    void setCompressionEnabled(bool enabled);
    
    // Security and validation
    bool validateFile(const QString &filePath, QString &errorMessage);
    QString calculateFileChecksum(const QString &filePath);
    
    // Approval dialog settings
    void setAutoApprovalEnabled(bool enabled);
    bool isAutoApprovalEnabled() const;
    void setApprovalTimeout(int seconds);
    int getApprovalTimeout() const;
    void setRememberDecisionEnabled(bool enabled);
    bool isRememberDecisionEnabled() const;
    
    // Security settings
    void addAllowedFileExtension(const QString &extension);
    void removeAllowedFileExtension(const QString &extension);
    QStringList getAllowedFileExtensions() const;
    void setMaxFileSize(qint64 maxSize);
    qint64 getMaxFileSize() const;

public slots:
    void onSessionRegistered(const QString &sessionId);
    void onTransferApprovalReceived(const QString &transferId, bool approved, const QString &message);

signals:
    // Connection signals
    void connected();
    void disconnected();
    void connectionError(const QString &error);
    
    // Transfer signals
    void transferRequested(const QString &transferId, const FileTransferRequest &request);
    void transferApproved(const QString &transferId);
    void transferRejected(const QString &transferId, const QString &reason);
    void transferStarted(const QString &transferId);
    void transferProgress(const QString &transferId, const FileTransferProgress &progress);
    void transferCompleted(const QString &transferId, const QString &filePath);
    void transferFailed(const QString &transferId, const QString &error);
    void transferCancelled(const QString &transferId);
    
    // Chunk signals
    void chunkSent(const QString &transferId, int chunkIndex);
    void chunkReceived(const QString &transferId, int chunkIndex);
    void chunkError(const QString &transferId, int chunkIndex, const QString &error);
    
    // Approval and security signals
    void transferApprovalRequested(const FileTransferRequest &request);
    void transferApprovalDecision(const QString &transferId, bool approved, const QString &message);
    void securityWarning(const QString &message, const QString &details);
    void fileValidationFailed(const QString &filePath, const QString &reason);
    void unauthorizedTransferAttempt(const QString &transferId, const QString &reason);

private slots:
    void onWebSocketConnected();
    void onWebSocketDisconnected();
    void onWebSocketError(QAbstractSocket::SocketError error);
    void onWebSocketTextMessageReceived(const QString &message);
    void onWebSocketBinaryMessageReceived(const QByteArray &data);
    void onPingTimer();
    void onTransferWorkerFinished();
    
    // Approval and security slots
    void onApprovalDialogFinished(int result);
    void onTransferRequestReceived(const QJsonObject &request);
    void processApprovalDecision(const QString &transferId, bool approved, const QString &message);

private:
    // WebSocket management
    void setupWebSocket();
    void registerSession();
    void sendPing();
    
    // Message handling
    void handleControlMessage(const QJsonObject &message);
    void handleTransferResponse(const QJsonObject &message);
    void handleTransferStatusUpdate(const QJsonObject &message);
    void handleChunkAcknowledgment(const QJsonObject &message);
    void handleProgressResponse(const QJsonObject &message);
    void handleErrorMessage(const QJsonObject &message);
    
    // Transfer management
    void startTransfer(const QString &transferId);
    void updateTransferProgress(const QString &transferId, const FileTransferProgress &progress);
    FileTransferSession* getTransferSession(const QString &transferId) const;
    
    // Utility functions
    QJsonObject createControlMessage(const QString &type, const QJsonObject &data = QJsonObject());
    void sendControlMessage(const QJsonObject &message);
    void sendBinaryChunk(const FileChunk &chunk);
    QString generateTransferId();
    
    // File operations
    bool prepareFileForUpload(const QString &filePath, FileTransferRequest &request);
    bool prepareFileForDownload(const QString &filename, const QString &savePath, FileTransferRequest &request);
    
    // Approval and security methods
    void showApprovalDialog(const FileTransferRequest &request);
    bool isFileExtensionAllowed(const QString &filePath) const;
    bool isFileSizeValid(qint64 fileSize) const;
    bool checkRememberedDecision(const QString &transferId, bool &approved) const;
    void saveRememberedDecision(const QString &transferId, bool approved);
    void loadSettings();
    void saveSettings();
    
    // Security
    QByteArray encryptData(const QByteArray &data);
    QByteArray decryptData(const QByteArray &encryptedData);
    QByteArray compressData(const QByteArray &data);
    QByteArray decompressData(const QByteArray &compressedData);

private:
    // WebSocket connection
    std::unique_ptr<QWebSocket> m_webSocket;
    QString m_serverUrl;
    QString m_sessionId;
    bool m_isConnected;
    
    // Connection management
    std::unique_ptr<QTimer> m_pingTimer;
    std::unique_ptr<QTimer> m_reconnectTimer;
    int m_reconnectAttempts;
    static const int MAX_RECONNECT_ATTEMPTS = 5;
    
    // Transfer management
    QMap<QString, std::unique_ptr<FileTransferSession>> m_transferSessions;
    QMap<QString, std::unique_ptr<FileTransferWorker>> m_transferWorkers;
    QMap<QString, std::unique_ptr<QThread>> m_transferThreads;
    
    // Configuration
    int m_chunkSize;
    int m_maxConcurrentTransfers;
    bool m_encryptionEnabled;
    bool m_compressionEnabled;
    qint64 m_maxFileSize;
    QStringList m_allowedExtensions;
    
    // Approval and security settings
    bool m_autoApprovalEnabled;
    int m_approvalTimeout;
    bool m_rememberDecisionEnabled;
    QSettings *m_settings;
    QHash<QString, bool> m_rememberedDecisions;
    QHash<QString, FileTransferRequest> m_pendingRequests;
    
    // Thread safety
    mutable QMutex m_mutex;
    
    // Network manager for HTTP operations
    std::unique_ptr<QNetworkAccessManager> m_networkManager;
};

// File transfer session class
class FileTransferSession : public QObject
{
    Q_OBJECT
    
public:
    explicit FileTransferSession(const FileTransferRequest &request, QObject *parent = nullptr);
    ~FileTransferSession();
    
    // Getters
    const FileTransferRequest& getRequest() const { return m_request; }
    TransferStatus getStatus() const { return m_status; }
    const FileTransferProgress& getProgress() const { return m_progress; }
    QString getErrorMessage() const { return m_errorMessage; }
    
    // Status management
    void setStatus(TransferStatus status);
    void setError(const QString &error);
    void updateProgress(qint64 bytesTransferred, qint64 speed = 0);
    
    // File operations
    bool openFile();
    void closeFile();
    QByteArray readChunk(int chunkIndex, int chunkSize);
    bool writeChunk(int chunkIndex, const QByteArray &data);
    
    // Validation
    bool validateChecksum();
    QString calculateCurrentChecksum();
    
signals:
    void statusChanged(TransferStatus status);
    void progressUpdated(const FileTransferProgress &progress);
    void errorOccurred(const QString &error);
    
private:
    void calculateProgress();
    void updateSpeed();
    
private:
    FileTransferRequest m_request;
    TransferStatus m_status;
    FileTransferProgress m_progress;
    QString m_errorMessage;
    
    std::unique_ptr<QFile> m_file;
    QDateTime m_startTime;
    QDateTime m_lastProgressUpdate;
    qint64 m_lastBytesTransferred;
    
    mutable QMutex m_mutex;
};

// File transfer worker for background operations
class FileTransferWorker : public QObject
{
    Q_OBJECT
    
public:
    explicit FileTransferWorker(FileTransferSession *session, FileTransferManager *manager, QObject *parent = nullptr);
    
public slots:
    void startTransfer();
    void pauseTransfer();
    void resumeTransfer();
    void cancelTransfer();
    
signals:
    void chunkReady(const FileChunk &chunk);
    void transferCompleted();
    void transferFailed(const QString &error);
    void progressUpdated(const FileTransferProgress &progress);
    
private slots:
    void processNextChunk();
    void onChunkAcknowledged(int chunkIndex);
    
private:
    void uploadFile();
    void downloadFile();
    void sendNextChunk();
    void handleReceivedChunk(const FileChunk &chunk);
    
private:
    FileTransferSession *m_session;
    FileTransferManager *m_manager;
    
    bool m_isPaused;
    bool m_isCancelled;
    int m_currentChunkIndex;
    int m_totalChunks;
    QQueue<int> m_pendingChunks;
    QSet<int> m_acknowledgedChunks;
    
    std::unique_ptr<QTimer> m_chunkTimer;
    
    mutable QMutex m_mutex;
};

#endif // FILETRANSFERMANAGER_H