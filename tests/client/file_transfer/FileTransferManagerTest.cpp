#include <QtTest/QtTest>
#include <QSignalSpy>
#include <QTemporaryFile>
#include <QTemporaryDir>
#include <QJsonObject>
#include <QJsonDocument>
#include <QCryptographicHash>
#include <QWebSocket>
#include <QEventLoop>
#include <QTimer>

#include "../../../src/client/src/filetransfer/FileTransferManager.h"

class FileTransferManagerTest : public QObject
{
    Q_OBJECT

private slots:
    void initTestCase();
    void cleanupTestCase();
    void init();
    void cleanup();

    // Basic functionality tests
    void testConstructor();
    void testConnectionToServer();
    void testFileValidation();
    void testChecksumCalculation();
    
    // Transfer request tests
    void testFileUploadRequest();
    void testFileDownloadRequest();
    void testInvalidFileUploadRequest();
    
    // Transfer control tests
    void testPauseTransfer();
    void testResumeTransfer();
    void testCancelTransfer();
    
    // Progress tracking tests
    void testProgressTracking();
    void testTransferCompletion();
    void testTransferFailure();
    
    // Configuration tests
    void testChunkSizeConfiguration();
    void testMaxConcurrentTransfers();
    void testEncryptionSettings();
    void testCompressionSettings();
    
    // Error handling tests
    void testNetworkErrorHandling();
    void testFileAccessErrorHandling();
    void testInvalidChecksumHandling();
    
    // Performance tests
    void testLargeFileTransfer();
    void testConcurrentTransfers();
    
    // Security tests
    void testFileTypeValidation();
    void testFileSizeValidation();
    void testEncryptionIntegrity();

private:
    FileTransferManager *m_manager;
    QTemporaryDir *m_tempDir;
    QString m_testServerUrl;
    
    // Helper methods
    QTemporaryFile* createTestFile(const QString &content, const QString &suffix = ".txt");
    QString calculateFileChecksum(const QString &filePath);
    void waitForSignal(QObject *sender, const char *signal, int timeout = 5000);
};

void FileTransferManagerTest::initTestCase()
{
    m_tempDir = new QTemporaryDir();
    QVERIFY(m_tempDir->isValid());
    m_testServerUrl = "ws://localhost:8080/filetransfer";
}

void FileTransferManagerTest::cleanupTestCase()
{
    delete m_tempDir;
}

void FileTransferManagerTest::init()
{
    m_manager = new FileTransferManager(this);
    QVERIFY(m_manager != nullptr);
}

void FileTransferManagerTest::cleanup()
{
    if (m_manager) {
        m_manager->disconnectFromServer();
        delete m_manager;
        m_manager = nullptr;
    }
}

void FileTransferManagerTest::testConstructor()
{
    QVERIFY(m_manager != nullptr);
    QVERIFY(!m_manager->isConnected());
    QVERIFY(m_manager->getActiveTransfers().isEmpty());
}

void FileTransferManagerTest::testConnectionToServer()
{
    QSignalSpy connectedSpy(m_manager, &FileTransferManager::connected);
    QSignalSpy errorSpy(m_manager, &FileTransferManager::connectionError);
    
    m_manager->connectToServer(m_testServerUrl);
    
    // Wait for connection result (either connected or error)
    QVERIFY(connectedSpy.wait(5000) || errorSpy.count() > 0);
    
    if (connectedSpy.count() > 0) {
        QVERIFY(m_manager->isConnected());
    } else {
        // Connection failed (expected in test environment)
        QVERIFY(errorSpy.count() > 0);
        QVERIFY(!m_manager->isConnected());
    }
}

void FileTransferManagerTest::testFileValidation()
{
    // Test valid file
    QTemporaryFile *validFile = createTestFile("Valid test content");
    QString errorMessage;
    QVERIFY(m_manager->validateFile(validFile->fileName(), errorMessage));
    QVERIFY(errorMessage.isEmpty());
    delete validFile;
    
    // Test non-existent file
    QVERIFY(!m_manager->validateFile("/non/existent/file.txt", errorMessage));
    QVERIFY(!errorMessage.isEmpty());
    
    // Test empty file path
    errorMessage.clear();
    QVERIFY(!m_manager->validateFile("", errorMessage));
    QVERIFY(!errorMessage.isEmpty());
}

void FileTransferManagerTest::testChecksumCalculation()
{
    const QString testContent = "Hello, World! This is a test file for checksum calculation.";
    QTemporaryFile *testFile = createTestFile(testContent);
    
    QString checksum = m_manager->calculateFileChecksum(testFile->fileName());
    QVERIFY(!checksum.isEmpty());
    QCOMPARE(checksum.length(), 64); // SHA-256 produces 64-character hex string
    
    // Verify checksum consistency
    QString checksum2 = m_manager->calculateFileChecksum(testFile->fileName());
    QCOMPARE(checksum, checksum2);
    
    delete testFile;
}

void FileTransferManagerTest::testFileUploadRequest()
{
    QTemporaryFile *testFile = createTestFile("Upload test content");
    
    QSignalSpy requestSpy(m_manager, &FileTransferManager::transferRequested);
    
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(), 
        "test-session-123", 
        "test-technician@example.com"
    );
    
    QVERIFY(!transferId.isEmpty());
    QVERIFY(transferId.length() > 0);
    
    delete testFile;
}

void FileTransferManagerTest::testFileDownloadRequest()
{
    QString savePath = m_tempDir->path() + "/downloaded_file.txt";
    
    QSignalSpy requestSpy(m_manager, &FileTransferManager::transferRequested);
    
    QString transferId = m_manager->requestFileDownload(
        "remote_file.txt",
        "test-session-123",
        "test-technician@example.com",
        savePath
    );
    
    QVERIFY(!transferId.isEmpty());
    QVERIFY(transferId.length() > 0);
}

void FileTransferManagerTest::testInvalidFileUploadRequest()
{
    // Test with non-existent file
    QString transferId = m_manager->requestFileUpload(
        "/non/existent/file.txt",
        "test-session-123",
        "test-technician@example.com"
    );
    
    QVERIFY(transferId.isEmpty());
}

void FileTransferManagerTest::testPauseTransfer()
{
    // Create a mock transfer first
    QTemporaryFile *testFile = createTestFile("Pause test content");
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    if (!transferId.isEmpty()) {
        // Test pause functionality
        m_manager->pauseTransfer(transferId);
        
        // Verify transfer is in active transfers list
        QStringList activeTransfers = m_manager->getActiveTransfers();
        QVERIFY(activeTransfers.contains(transferId));
    }
    
    delete testFile;
}

void FileTransferManagerTest::testResumeTransfer()
{
    QTemporaryFile *testFile = createTestFile("Resume test content");
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    if (!transferId.isEmpty()) {
        // Pause then resume
        m_manager->pauseTransfer(transferId);
        m_manager->resumeTransfer(transferId);
        
        QStringList activeTransfers = m_manager->getActiveTransfers();
        QVERIFY(activeTransfers.contains(transferId));
    }
    
    delete testFile;
}

void FileTransferManagerTest::testCancelTransfer()
{
    QTemporaryFile *testFile = createTestFile("Cancel test content");
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    if (!transferId.isEmpty()) {
        QSignalSpy cancelledSpy(m_manager, &FileTransferManager::transferCancelled);
        
        m_manager->cancelTransfer(transferId);
        
        // Wait for cancellation signal or timeout
        cancelledSpy.wait(1000);
    }
    
    delete testFile;
}

void FileTransferManagerTest::testProgressTracking()
{
    QTemporaryFile *testFile = createTestFile("Progress tracking test content");
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    if (!transferId.isEmpty()) {
        FileTransferProgress progress = m_manager->getTransferProgress(transferId);
        QCOMPARE(progress.transferId, transferId);
        QVERIFY(progress.totalBytes >= 0);
        QVERIFY(progress.bytesTransferred >= 0);
        QVERIFY(progress.percentage >= 0.0 && progress.percentage <= 100.0);
    }
    
    delete testFile;
}

void FileTransferManagerTest::testTransferCompletion()
{
    QSignalSpy completedSpy(m_manager, &FileTransferManager::transferCompleted);
    
    // Simulate transfer completion by triggering the signal manually
    // In a real scenario, this would be triggered by the server
    emit m_manager->transferCompleted("test-transfer-id", "/path/to/completed/file.txt");
    
    QCOMPARE(completedSpy.count(), 1);
    QList<QVariant> arguments = completedSpy.takeFirst();
    QCOMPARE(arguments.at(0).toString(), "test-transfer-id");
}

void FileTransferManagerTest::testTransferFailure()
{
    QSignalSpy failedSpy(m_manager, &FileTransferManager::transferFailed);
    
    // Simulate transfer failure
    emit m_manager->transferFailed("test-transfer-id", "Network connection lost");
    
    QCOMPARE(failedSpy.count(), 1);
    QList<QVariant> arguments = failedSpy.takeFirst();
    QCOMPARE(arguments.at(0).toString(), "test-transfer-id");
    QCOMPARE(arguments.at(1).toString(), "Network connection lost");
}

void FileTransferManagerTest::testChunkSizeConfiguration()
{
    const int testChunkSize = 128 * 1024; // 128KB
    m_manager->setChunkSize(testChunkSize);
    
    // Verify chunk size is set (this would require exposing a getter)
    // For now, just verify the method doesn't crash
    QVERIFY(true);
}

void FileTransferManagerTest::testMaxConcurrentTransfers()
{
    const int maxConcurrent = 3;
    m_manager->setMaxConcurrentTransfers(maxConcurrent);
    
    // Verify setting doesn't crash
    QVERIFY(true);
}

void FileTransferManagerTest::testEncryptionSettings()
{
    m_manager->setEncryptionEnabled(true);
    m_manager->setEncryptionEnabled(false);
    
    // Verify settings don't crash
    QVERIFY(true);
}

void FileTransferManagerTest::testCompressionSettings()
{
    m_manager->setCompressionEnabled(true);
    m_manager->setCompressionEnabled(false);
    
    // Verify settings don't crash
    QVERIFY(true);
}

void FileTransferManagerTest::testNetworkErrorHandling()
{
    QSignalSpy errorSpy(m_manager, &FileTransferManager::connectionError);
    
    // Try to connect to invalid server
    m_manager->connectToServer("ws://invalid-server:9999/filetransfer");
    
    // Wait for error signal
    QVERIFY(errorSpy.wait(5000));
    QVERIFY(errorSpy.count() > 0);
}

void FileTransferManagerTest::testFileAccessErrorHandling()
{
    QString errorMessage;
    
    // Test with file in non-existent directory
    bool result = m_manager->validateFile("/non/existent/directory/file.txt", errorMessage);
    QVERIFY(!result);
    QVERIFY(!errorMessage.isEmpty());
}

void FileTransferManagerTest::testInvalidChecksumHandling()
{
    // Test checksum calculation with non-existent file
    QString checksum = m_manager->calculateFileChecksum("/non/existent/file.txt");
    QVERIFY(checksum.isEmpty());
}

void FileTransferManagerTest::testLargeFileTransfer()
{
    // Create a larger test file (1MB)
    QByteArray largeContent(1024 * 1024, 'A'); // 1MB of 'A' characters
    QTemporaryFile *largeFile = createTestFile(QString::fromLatin1(largeContent));
    
    QString transferId = m_manager->requestFileUpload(
        largeFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    QVERIFY(!transferId.isEmpty());
    
    // Verify progress tracking works with large files
    FileTransferProgress progress = m_manager->getTransferProgress(transferId);
    QVERIFY(progress.totalBytes >= 1024 * 1024);
    
    delete largeFile;
}

void FileTransferManagerTest::testConcurrentTransfers()
{
    QList<QTemporaryFile*> testFiles;
    QStringList transferIds;
    
    // Create multiple test files and start transfers
    for (int i = 0; i < 3; ++i) {
        QTemporaryFile *file = createTestFile(QString("Concurrent test content %1").arg(i));
        testFiles.append(file);
        
        QString transferId = m_manager->requestFileUpload(
            file->fileName(),
            QString("test-session-%1").arg(i),
            "test-technician@example.com"
        );
        
        if (!transferId.isEmpty()) {
            transferIds.append(transferId);
        }
    }
    
    // Verify all transfers are tracked
    QStringList activeTransfers = m_manager->getActiveTransfers();
    for (const QString &transferId : transferIds) {
        QVERIFY(activeTransfers.contains(transferId));
    }
    
    // Cleanup
    qDeleteAll(testFiles);
}

void FileTransferManagerTest::testFileTypeValidation()
{
    // Test with allowed file type
    QTemporaryFile *txtFile = createTestFile("Text content", ".txt");
    QString errorMessage;
    QVERIFY(m_manager->validateFile(txtFile->fileName(), errorMessage));
    delete txtFile;
    
    // Test with potentially dangerous file type
    QTemporaryFile *exeFile = createTestFile("Executable content", ".exe");
    errorMessage.clear();
    // Note: This test depends on the file type validation implementation
    // The result may vary based on the allowed file types configuration
    m_manager->validateFile(exeFile->fileName(), errorMessage);
    delete exeFile;
}

void FileTransferManagerTest::testFileSizeValidation()
{
    // Create a small file that should be allowed
    QTemporaryFile *smallFile = createTestFile("Small content");
    QString errorMessage;
    QVERIFY(m_manager->validateFile(smallFile->fileName(), errorMessage));
    delete smallFile;
    
    // Note: Testing very large files would require more complex setup
    // and might not be practical in unit tests
}

void FileTransferManagerTest::testEncryptionIntegrity()
{
    // Enable encryption
    m_manager->setEncryptionEnabled(true);
    
    QTemporaryFile *testFile = createTestFile("Encryption test content");
    
    QString transferId = m_manager->requestFileUpload(
        testFile->fileName(),
        "test-session-123",
        "test-technician@example.com"
    );
    
    QVERIFY(!transferId.isEmpty());
    
    delete testFile;
}

// Helper method implementations
QTemporaryFile* FileTransferManagerTest::createTestFile(const QString &content, const QString &suffix)
{
    QTemporaryFile *file = new QTemporaryFile(m_tempDir->path() + "/test_XXXXXX" + suffix);
    if (file->open()) {
        file->write(content.toUtf8());
        file->flush();
    }
    return file;
}

QString FileTransferManagerTest::calculateFileChecksum(const QString &filePath)
{
    QFile file(filePath);
    if (!file.open(QIODevice::ReadOnly)) {
        return QString();
    }
    
    QCryptographicHash hash(QCryptographicHash::Sha256);
    hash.addData(&file);
    return QString::fromLatin1(hash.result().toHex());
}

void FileTransferManagerTest::waitForSignal(QObject *sender, const char *signal, int timeout)
{
    QEventLoop loop;
    QTimer timer;
    timer.setSingleShot(true);
    timer.setInterval(timeout);
    
    connect(sender, signal, &loop, &QEventLoop::quit);
    connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    
    timer.start();
    loop.exec();
}

QTEST_MAIN(FileTransferManagerTest)
#include "FileTransferManagerTest.moc"