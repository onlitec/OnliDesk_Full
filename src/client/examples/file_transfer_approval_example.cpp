#include "../src/filetransfer/FileTransferManager.h"
#include "../src/filetransfer/ApprovalDialog.h"
#include <QApplication>
#include <QMainWindow>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QPushButton>
#include <QLabel>
#include <QLineEdit>
#include <QSpinBox>
#include <QCheckBox>
#include <QGroupBox>
#include <QListWidget>
#include <QMessageBox>
#include <QFileDialog>
#include <QDebug>

class FileTransferExample : public QMainWindow
{
    Q_OBJECT

public:
    FileTransferExample(QWidget *parent = nullptr)
        : QMainWindow(parent)
        , m_transferManager(new FileTransferManager(this))
    {
        setupUI();
        setupConnections();
        
        // Configure default settings
        m_transferManager->setAutoApprovalEnabled(false);
        m_transferManager->setApprovalTimeout(30);
        m_transferManager->setRememberDecisionEnabled(true);
        m_transferManager->setMaxFileSize(50 * 1024 * 1024); // 50MB
        
        // Add some allowed extensions
        QStringList allowedExtensions = {
            ".txt", ".pdf", ".doc", ".docx", ".xls", ".xlsx",
            ".jpg", ".jpeg", ".png", ".gif", ".bmp",
            ".zip", ".rar", ".7z"
        };
        
        for (const QString &ext : allowedExtensions) {
            m_transferManager->addAllowedFileExtension(ext);
        }
    }

private slots:
    void connectToServer()
    {
        QString serverUrl = m_serverUrlEdit->text();
        if (serverUrl.isEmpty()) {
            serverUrl = "ws://localhost:8080/ws";
        }
        
        m_transferManager->connectToServer(serverUrl);
        m_statusLabel->setText("Connecting...");
    }
    
    void disconnectFromServer()
    {
        m_transferManager->disconnectFromServer();
        m_statusLabel->setText("Disconnected");
    }
    
    void onConnected()
    {
        m_statusLabel->setText("Connected");
        m_connectButton->setEnabled(false);
        m_disconnectButton->setEnabled(true);
        
        QMessageBox::information(this, "Connected", 
            "Successfully connected to file transfer server.\n"
            "You can now receive transfer requests from technicians.");
    }
    
    void onDisconnected()
    {
        m_statusLabel->setText("Disconnected");
        m_connectButton->setEnabled(true);
        m_disconnectButton->setEnabled(false);
    }
    
    void onConnectionError(const QString &error)
    {
        m_statusLabel->setText("Connection Error");
        QMessageBox::warning(this, "Connection Error", 
            QString("Failed to connect to server:\n%1").arg(error));
    }
    
    void onTransferApprovalRequested(const FileTransferRequest &request)
    {
        QString message = QString("Transfer approval requested:\n"
                                "File: %1\n"
                                "Size: %2 bytes\n"
                                "Type: %3\n"
                                "Technician: %4")
                         .arg(request.filename)
                         .arg(request.fileSize)
                         .arg(request.type == TransferType::Upload ? "Upload" : "Download")
                         .arg(request.technician);
        
        m_transferList->addItem(message);
        qDebug() << "Transfer approval requested:" << request.id;
    }
    
    void onTransferApprovalDecision(const QString &transferId, bool approved, const QString &message)
    {
        QString decision = approved ? "APPROVED" : "REJECTED";
        QString logMessage = QString("Transfer %1 %2: %3")
                           .arg(transferId)
                           .arg(decision)
                           .arg(message);
        
        m_transferList->addItem(logMessage);
        qDebug() << "Transfer decision:" << transferId << decision << message;
    }
    
    void onSecurityWarning(const QString &message, const QString &details)
    {
        QMessageBox::warning(this, "Security Warning", 
            QString("%1\n\nDetails: %2").arg(message, details));
        
        m_transferList->addItem(QString("SECURITY WARNING: %1").arg(message));
    }
    
    void onFileValidationFailed(const QString &filePath, const QString &reason)
    {
        QString message = QString("File validation failed for %1: %2")
                         .arg(filePath, reason);
        
        m_transferList->addItem(message);
        QMessageBox::warning(this, "File Validation Failed", message);
    }
    
    void updateAutoApproval(bool enabled)
    {
        m_transferManager->setAutoApprovalEnabled(enabled);
    }
    
    void updateApprovalTimeout(int seconds)
    {
        m_transferManager->setApprovalTimeout(seconds);
    }
    
    void updateRememberDecision(bool enabled)
    {
        m_transferManager->setRememberDecisionEnabled(enabled);
    }
    
    void addAllowedExtension()
    {
        QString extension = QInputDialog::getText(this, "Add Extension", 
            "Enter file extension (e.g., .txt):");
        
        if (!extension.isEmpty()) {
            m_transferManager->addAllowedFileExtension(extension);
            updateExtensionsList();
        }
    }
    
    void removeAllowedExtension()
    {
        QListWidgetItem *item = m_extensionsList->currentItem();
        if (item) {
            QString extension = item->text();
            m_transferManager->removeAllowedFileExtension(extension);
            updateExtensionsList();
        }
    }
    
    void clearTransferLog()
    {
        m_transferList->clear();
    }

private:
    void setupUI()
    {
        auto *centralWidget = new QWidget(this);
        setCentralWidget(centralWidget);
        
        auto *mainLayout = new QVBoxLayout(centralWidget);
        
        // Connection section
        auto *connectionGroup = new QGroupBox("Server Connection", this);
        auto *connectionLayout = new QHBoxLayout(connectionGroup);
        
        connectionLayout->addWidget(new QLabel("Server URL:"));
        m_serverUrlEdit = new QLineEdit("ws://localhost:8080/ws", this);
        connectionLayout->addWidget(m_serverUrlEdit);
        
        m_connectButton = new QPushButton("Connect", this);
        m_disconnectButton = new QPushButton("Disconnect", this);
        m_disconnectButton->setEnabled(false);
        connectionLayout->addWidget(m_connectButton);
        connectionLayout->addWidget(m_disconnectButton);
        
        m_statusLabel = new QLabel("Disconnected", this);
        connectionLayout->addWidget(m_statusLabel);
        
        mainLayout->addWidget(connectionGroup);
        
        // Settings section
        auto *settingsGroup = new QGroupBox("Approval Settings", this);
        auto *settingsLayout = new QVBoxLayout(settingsGroup);
        
        m_autoApprovalCheck = new QCheckBox("Enable auto-approval", this);
        settingsLayout->addWidget(m_autoApprovalCheck);
        
        auto *timeoutLayout = new QHBoxLayout();
        timeoutLayout->addWidget(new QLabel("Approval timeout (seconds):"));
        m_timeoutSpinBox = new QSpinBox(this);
        m_timeoutSpinBox->setRange(5, 300);
        m_timeoutSpinBox->setValue(30);
        timeoutLayout->addWidget(m_timeoutSpinBox);
        timeoutLayout->addStretch();
        settingsLayout->addLayout(timeoutLayout);
        
        m_rememberDecisionCheck = new QCheckBox("Remember approval decisions", this);
        m_rememberDecisionCheck->setChecked(true);
        settingsLayout->addWidget(m_rememberDecisionCheck);
        
        mainLayout->addWidget(settingsGroup);
        
        // Extensions section
        auto *extensionsGroup = new QGroupBox("Allowed File Extensions", this);
        auto *extensionsLayout = new QHBoxLayout(extensionsGroup);
        
        m_extensionsList = new QListWidget(this);
        extensionsLayout->addWidget(m_extensionsList);
        
        auto *extensionsButtonLayout = new QVBoxLayout();
        auto *addExtButton = new QPushButton("Add", this);
        auto *removeExtButton = new QPushButton("Remove", this);
        extensionsButtonLayout->addWidget(addExtButton);
        extensionsButtonLayout->addWidget(removeExtButton);
        extensionsButtonLayout->addStretch();
        extensionsLayout->addLayout(extensionsButtonLayout);
        
        mainLayout->addWidget(extensionsGroup);
        
        // Transfer log section
        auto *logGroup = new QGroupBox("Transfer Log", this);
        auto *logLayout = new QVBoxLayout(logGroup);
        
        m_transferList = new QListWidget(this);
        logLayout->addWidget(m_transferList);
        
        auto *clearLogButton = new QPushButton("Clear Log", this);
        logLayout->addWidget(clearLogButton);
        
        mainLayout->addWidget(logGroup);
        
        // Connect UI signals
        connect(m_connectButton, &QPushButton::clicked, this, &FileTransferExample::connectToServer);
        connect(m_disconnectButton, &QPushButton::clicked, this, &FileTransferExample::disconnectFromServer);
        connect(m_autoApprovalCheck, &QCheckBox::toggled, this, &FileTransferExample::updateAutoApproval);
        connect(m_timeoutSpinBox, QOverload<int>::of(&QSpinBox::valueChanged), this, &FileTransferExample::updateApprovalTimeout);
        connect(m_rememberDecisionCheck, &QCheckBox::toggled, this, &FileTransferExample::updateRememberDecision);
        connect(addExtButton, &QPushButton::clicked, this, &FileTransferExample::addAllowedExtension);
        connect(removeExtButton, &QPushButton::clicked, this, &FileTransferExample::removeAllowedExtension);
        connect(clearLogButton, &QPushButton::clicked, this, &FileTransferExample::clearTransferLog);
        
        setWindowTitle("File Transfer Approval Example");
        resize(800, 600);
        
        updateExtensionsList();
    }
    
    void setupConnections()
    {
        connect(m_transferManager, &FileTransferManager::connected, 
                this, &FileTransferExample::onConnected);
        connect(m_transferManager, &FileTransferManager::disconnected, 
                this, &FileTransferExample::onDisconnected);
        connect(m_transferManager, &FileTransferManager::connectionError, 
                this, &FileTransferExample::onConnectionError);
        connect(m_transferManager, &FileTransferManager::transferApprovalRequested, 
                this, &FileTransferExample::onTransferApprovalRequested);
        connect(m_transferManager, &FileTransferManager::transferApprovalDecision, 
                this, &FileTransferExample::onTransferApprovalDecision);
        connect(m_transferManager, &FileTransferManager::securityWarning, 
                this, &FileTransferExample::onSecurityWarning);
        connect(m_transferManager, &FileTransferManager::fileValidationFailed, 
                this, &FileTransferExample::onFileValidationFailed);
    }
    
    void updateExtensionsList()
    {
        m_extensionsList->clear();
        QStringList extensions = m_transferManager->getAllowedFileExtensions();
        m_extensionsList->addItems(extensions);
    }

private:
    FileTransferManager *m_transferManager;
    
    // UI components
    QLineEdit *m_serverUrlEdit;
    QPushButton *m_connectButton;
    QPushButton *m_disconnectButton;
    QLabel *m_statusLabel;
    
    QCheckBox *m_autoApprovalCheck;
    QSpinBox *m_timeoutSpinBox;
    QCheckBox *m_rememberDecisionCheck;
    
    QListWidget *m_extensionsList;
    QListWidget *m_transferList;
};

int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    
    FileTransferExample window;
    window.show();
    
    return app.exec();
}

#include "file_transfer_approval_example.moc"