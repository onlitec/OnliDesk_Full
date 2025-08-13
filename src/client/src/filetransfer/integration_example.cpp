// Integration Example for File Transfer Dialog
// This file demonstrates how to integrate the TransferDialog into the main application

#include "transfer_dialog.h"
#include "FileTransferManager.h"
#include <QApplication>
#include <QMainWindow>
#include <QMenuBar>
#include <QAction>
#include <QMessageBox>
#include <QVBoxLayout>
#include <QWidget>
#include <QLabel>
#include <QPushButton>

class MainWindow : public QMainWindow
{
    Q_OBJECT

public:
    MainWindow(QWidget *parent = nullptr)
        : QMainWindow(parent)
        , m_transferManager(new FileTransferManager(this))
        , m_transferDialog(nullptr)
    {
        setupUI();
        setupConnections();
    }

private slots:
    void showFileTransferDialog()
    {
        if (!m_transferDialog) {
            m_transferDialog = new TransferDialog(m_transferManager, this);
            
            // Connect dialog signals
            connect(m_transferDialog, &TransferDialog::transferRequested,
                    this, &MainWindow::onTransferRequested);
            connect(m_transferDialog, &TransferDialog::transferCancelled,
                    this, &MainWindow::onTransferCancelled);
        }
        
        m_transferDialog->show();
        m_transferDialog->raise();
        m_transferDialog->activateWindow();
    }
    
    void onTransferRequested(const QString &filePath, FileTransferManager::TransferType type)
    {
        QMessageBox::information(this, "Transfer Requested", 
                               QString("Transfer requested for: %1\nType: %2")
                               .arg(filePath)
                               .arg(type == FileTransferManager::TransferType::Upload ? "Upload" : "Download"));
    }
    
    void onTransferCancelled()
    {
        QMessageBox::information(this, "Transfer Cancelled", "File transfer was cancelled.");
    }
    
    void onTransferManagerConnected()
    {
        statusBar()->showMessage("Connected to server", 3000);
    }
    
    void onTransferManagerDisconnected()
    {
        statusBar()->showMessage("Disconnected from server", 3000);
    }
    
    void onTransferManagerError(const QString &error)
    {
        QMessageBox::warning(this, "Transfer Error", error);
    }

private:
    void setupUI()
    {
        setWindowTitle("Onlidesk Client - File Transfer Demo");
        setMinimumSize(800, 600);
        
        // Create central widget
        auto *centralWidget = new QWidget(this);
        setCentralWidget(centralWidget);
        
        auto *layout = new QVBoxLayout(centralWidget);
        
        // Add status label
        auto *statusLabel = new QLabel("File Transfer Integration Demo", this);
        statusLabel->setAlignment(Qt::AlignCenter);
        statusLabel->setStyleSheet("font-size: 16px; font-weight: bold; margin: 20px;");
        layout->addWidget(statusLabel);
        
        // Add transfer button
        auto *transferButton = new QPushButton("Open File Transfer Dialog", this);
        transferButton->setMinimumHeight(40);
        connect(transferButton, &QPushButton::clicked, this, &MainWindow::showFileTransferDialog);
        layout->addWidget(transferButton);
        
        layout->addStretch();
        
        // Create menu bar
        auto *fileMenu = menuBar()->addMenu("&File");
        
        auto *transferAction = new QAction("&File Transfer...", this);
        transferAction->setShortcut(QKeySequence::New);
        connect(transferAction, &QAction::triggered, this, &MainWindow::showFileTransferDialog);
        fileMenu->addAction(transferAction);
        
        fileMenu->addSeparator();
        
        auto *exitAction = new QAction("E&xit", this);
        exitAction->setShortcut(QKeySequence::Quit);
        connect(exitAction, &QAction::triggered, this, &QWidget::close);
        fileMenu->addAction(exitAction);
        
        // Create status bar
        statusBar()->showMessage("Ready");
    }
    
    void setupConnections()
    {
        // Connect transfer manager signals
        connect(m_transferManager, &FileTransferManager::connected,
                this, &MainWindow::onTransferManagerConnected);
        connect(m_transferManager, &FileTransferManager::disconnected,
                this, &MainWindow::onTransferManagerDisconnected);
        connect(m_transferManager, &FileTransferManager::errorOccurred,
                this, &MainWindow::onTransferManagerError);
        
        // Auto-connect to server (in a real application, this would be configurable)
        QTimer::singleShot(1000, [this]() {
            m_transferManager->connectToServer("ws://localhost:8080/ws/filetransfer");
        });
    }

private:
    FileTransferManager *m_transferManager;
    TransferDialog *m_transferDialog;
};

// Example usage in main.cpp:
/*
int main(int argc, char *argv[])
{
    QApplication app(argc, argv);
    
    // Set application properties
    app.setApplicationName("Onlidesk Client");
    app.setApplicationVersion("1.0.0");
    app.setOrganizationName("Onlitec");
    
    // Create and show main window
    MainWindow window;
    window.show();
    
    return app.exec();
}
*/

#include "integration_example.moc"