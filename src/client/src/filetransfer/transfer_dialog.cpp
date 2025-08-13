#include "transfer_dialog.h"
#include "progress_widget.h"
#include <QApplication>
#include <QMessageBox>
#include <QFileInfo>
#include <QDir>
#include <QMimeData>
#include <QScrollArea>
#include <QSettings>
#include <QCloseEvent>
#include <QResizeEvent>
#include <QSplitter>
#include <QGroupBox>
#include <QGridLayout>
#include <QFormLayout>
#include <QHeaderView>
#include <QStandardPaths>
#include <QDesktopServices>
#include <QUrl>
#include <QDebug>

// Constants
static const int DEFAULT_CHUNK_SIZE = 64 * 1024; // 64KB
static const int DEFAULT_MAX_CONCURRENT = 3;
static const int UPDATE_INTERVAL = 1000; // 1 second
static const qint64 MAX_FILE_SIZE = 100 * 1024 * 1024; // 100MB

TransferDialog::TransferDialog(FileTransferManager *manager, const QString &sessionId, 
                             const QString &technician, QWidget *parent)
    : QDialog(parent)
    , m_manager(manager)
    , m_sessionId(sessionId)
    , m_technician(technician)
    , m_mainLayout(nullptr)
    , m_mainSplitter(nullptr)
    , m_fileGroup(nullptr)
    , m_fileList(nullptr)
    , m_browseFilesBtn(nullptr)
    , m_browseFolderBtn(nullptr)
    , m_removeBtn(nullptr)
    , m_clearBtn(nullptr)
    , m_dropLabel(nullptr)
    , m_progressGroup(nullptr)
    , m_progressScrollArea(nullptr)
    , m_progressContainer(nullptr)
    , m_progressLayout(nullptr)
    , m_controlsGroup(nullptr)
    , m_startBtn(nullptr)
    , m_pauseAllBtn(nullptr)
    , m_resumeAllBtn(nullptr)
    , m_cancelAllBtn(nullptr)
    , m_settingsGroup(nullptr)
    , m_chunkSizeSpinBox(nullptr)
    , m_maxConcurrentSpinBox(nullptr)
    , m_encryptionCheckBox(nullptr)
    , m_compressionComboBox(nullptr)
    , m_statusGroup(nullptr)
    , m_totalFilesLabel(nullptr)
    , m_totalSizeLabel(nullptr)
    , m_activeTransfersLabel(nullptr)
    , m_completedTransfersLabel(nullptr)
    , m_overallSpeedLabel(nullptr)
    , m_overallProgressBar(nullptr)
    , m_totalFiles(0)
    , m_totalSize(0)
    , m_activeTransfers(0)
    , m_completedTransfers(0)
    , m_failedTransfers(0)
    , m_totalBytesTransferred(0)
    , m_overallSpeed(0)
    , m_chunkSize(DEFAULT_CHUNK_SIZE)
    , m_maxConcurrentTransfers(DEFAULT_MAX_CONCURRENT)
    , m_encryptionEnabled(true)
    , m_compressionEnabled(false)
    , m_isTransferring(false)
    , m_updateTimer(new QTimer(this))
{
    setWindowTitle(tr("File Transfer - Session %1").arg(sessionId));
    setWindowIcon(QIcon(":/icons/file_transfer.png"));
    setMinimumSize(800, 600);
    resize(1000, 700);
    
    // Enable drag and drop
    setAcceptDrops(true);
    
    setupUI();
    connectSignals();
    loadSettings();
    
    // Setup update timer
    m_updateTimer->setInterval(UPDATE_INTERVAL);
    connect(m_updateTimer, &QTimer::timeout, this, &TransferDialog::updateStatistics);
    m_updateTimer->start();
    
    updateUI();
}

TransferDialog::~TransferDialog()
{
    saveSettings();
}

void TransferDialog::setupUI()
{
    m_mainLayout = new QVBoxLayout(this);
    m_mainSplitter = new QSplitter(Qt::Horizontal, this);
    
    setupFileListArea();
    setupProgressArea();
    setupControlsArea();
    setupSettingsArea();
    setupStatusArea();
    
    // Left panel (file selection and controls)
    QWidget *leftPanel = new QWidget();
    QVBoxLayout *leftLayout = new QVBoxLayout(leftPanel);
    leftLayout->addWidget(m_fileGroup);
    leftLayout->addWidget(m_controlsGroup);
    leftLayout->addWidget(m_settingsGroup);
    leftLayout->addStretch();
    
    // Right panel (progress and status)
    QWidget *rightPanel = new QWidget();
    QVBoxLayout *rightLayout = new QVBoxLayout(rightPanel);
    rightLayout->addWidget(m_progressGroup);
    rightLayout->addWidget(m_statusGroup);
    
    m_mainSplitter->addWidget(leftPanel);
    m_mainSplitter->addWidget(rightPanel);
    m_mainSplitter->setStretchFactor(0, 1);
    m_mainSplitter->setStretchFactor(1, 2);
    
    m_mainLayout->addWidget(m_mainSplitter);
}

void TransferDialog::setupFileListArea()
{
    m_fileGroup = new QGroupBox(tr("File Selection"));
    QVBoxLayout *layout = new QVBoxLayout(m_fileGroup);
    
    // Drop area label
    m_dropLabel = new QLabel(tr("Drag and drop files here or use the buttons below"));
    m_dropLabel->setAlignment(Qt::AlignCenter);
    m_dropLabel->setStyleSheet(
        "QLabel {"
        "    border: 2px dashed #aaa;"
        "    border-radius: 5px;"
        "    padding: 20px;"
        "    color: #666;"
        "    background-color: #f9f9f9;"
        "}"
    );
    layout->addWidget(m_dropLabel);
    
    // File list
    m_fileList = new QListWidget();
    m_fileList->setSelectionMode(QAbstractItemView::ExtendedSelection);
    m_fileList->setDragDropMode(QAbstractItemView::DropOnly);
    layout->addWidget(m_fileList);
    
    // Buttons
    QHBoxLayout *buttonLayout = new QHBoxLayout();
    m_browseFilesBtn = new QPushButton(tr("Browse Files..."));
    m_browseFolderBtn = new QPushButton(tr("Browse Folder..."));
    m_removeBtn = new QPushButton(tr("Remove Selected"));
    m_clearBtn = new QPushButton(tr("Clear All"));
    
    m_browseFilesBtn->setIcon(QIcon(":/icons/file.png"));
    m_browseFolderBtn->setIcon(QIcon(":/icons/folder.png"));
    m_removeBtn->setIcon(QIcon(":/icons/remove.png"));
    m_clearBtn->setIcon(QIcon(":/icons/clear.png"));
    
    buttonLayout->addWidget(m_browseFilesBtn);
    buttonLayout->addWidget(m_browseFolderBtn);
    buttonLayout->addStretch();
    buttonLayout->addWidget(m_removeBtn);
    buttonLayout->addWidget(m_clearBtn);
    
    layout->addLayout(buttonLayout);
}

void TransferDialog::setupProgressArea()
{
    m_progressGroup = new QGroupBox(tr("Transfer Progress"));
    QVBoxLayout *layout = new QVBoxLayout(m_progressGroup);
    
    // Scroll area for progress widgets
    m_progressScrollArea = new QScrollArea();
    m_progressScrollArea->setWidgetResizable(true);
    m_progressScrollArea->setHorizontalScrollBarPolicy(Qt::ScrollBarAsNeeded);
    m_progressScrollArea->setVerticalScrollBarPolicy(Qt::ScrollBarAsNeeded);
    
    m_progressContainer = new QWidget();
    m_progressLayout = new QVBoxLayout(m_progressContainer);
    m_progressLayout->addStretch();
    
    m_progressScrollArea->setWidget(m_progressContainer);
    layout->addWidget(m_progressScrollArea);
}

void TransferDialog::setupControlsArea()
{
    m_controlsGroup = new QGroupBox(tr("Transfer Controls"));
    QGridLayout *layout = new QGridLayout(m_controlsGroup);
    
    m_startBtn = new QPushButton(tr("Start Transfers"));
    m_pauseAllBtn = new QPushButton(tr("Pause All"));
    m_resumeAllBtn = new QPushButton(tr("Resume All"));
    m_cancelAllBtn = new QPushButton(tr("Cancel All"));
    
    m_startBtn->setIcon(QIcon(":/icons/play.png"));
    m_pauseAllBtn->setIcon(QIcon(":/icons/pause.png"));
    m_resumeAllBtn->setIcon(QIcon(":/icons/resume.png"));
    m_cancelAllBtn->setIcon(QIcon(":/icons/stop.png"));
    
    layout->addWidget(m_startBtn, 0, 0);
    layout->addWidget(m_pauseAllBtn, 0, 1);
    layout->addWidget(m_resumeAllBtn, 1, 0);
    layout->addWidget(m_cancelAllBtn, 1, 1);
}

void TransferDialog::setupSettingsArea()
{
    m_settingsGroup = new QGroupBox(tr("Transfer Settings"));
    QFormLayout *layout = new QFormLayout(m_settingsGroup);
    
    // Chunk size
    m_chunkSizeSpinBox = new QSpinBox();
    m_chunkSizeSpinBox->setRange(1, 1024);
    m_chunkSizeSpinBox->setValue(m_chunkSize / 1024);
    m_chunkSizeSpinBox->setSuffix(" KB");
    layout->addRow(tr("Chunk Size:"), m_chunkSizeSpinBox);
    
    // Max concurrent transfers
    m_maxConcurrentSpinBox = new QSpinBox();
    m_maxConcurrentSpinBox->setRange(1, 10);
    m_maxConcurrentSpinBox->setValue(m_maxConcurrentTransfers);
    layout->addRow(tr("Max Concurrent:"), m_maxConcurrentSpinBox);
    
    // Encryption
    m_encryptionCheckBox = new QCheckBox(tr("Enable Encryption"));
    m_encryptionCheckBox->setChecked(m_encryptionEnabled);
    layout->addRow(m_encryptionCheckBox);
    
    // Compression
    m_compressionComboBox = new QComboBox();
    m_compressionComboBox->addItems({tr("No Compression"), tr("Fast"), tr("Best")});
    m_compressionComboBox->setCurrentIndex(m_compressionEnabled ? 1 : 0);
    layout->addRow(tr("Compression:"), m_compressionComboBox);
}

void TransferDialog::setupStatusArea()
{
    m_statusGroup = new QGroupBox(tr("Transfer Statistics"));
    QFormLayout *layout = new QFormLayout(m_statusGroup);
    
    m_totalFilesLabel = new QLabel("0");
    m_totalSizeLabel = new QLabel("0 B");
    m_activeTransfersLabel = new QLabel("0");
    m_completedTransfersLabel = new QLabel("0");
    m_overallSpeedLabel = new QLabel("0 B/s");
    
    layout->addRow(tr("Total Files:"), m_totalFilesLabel);
    layout->addRow(tr("Total Size:"), m_totalSizeLabel);
    layout->addRow(tr("Active Transfers:"), m_activeTransfersLabel);
    layout->addRow(tr("Completed:"), m_completedTransfersLabel);
    layout->addRow(tr("Overall Speed:"), m_overallSpeedLabel);
    
    // Overall progress bar
    m_overallProgressBar = new QProgressBar();
    m_overallProgressBar->setRange(0, 100);
    m_overallProgressBar->setValue(0);
    layout->addRow(tr("Overall Progress:"), m_overallProgressBar);
}

void TransferDialog::connectSignals()
{
    // File operations
    connect(m_browseFilesBtn, &QPushButton::clicked, this, &TransferDialog::onBrowseFiles);
    connect(m_browseFolderBtn, &QPushButton::clicked, this, &TransferDialog::onBrowseFolder);
    connect(m_removeBtn, &QPushButton::clicked, this, &TransferDialog::onRemoveSelected);
    connect(m_clearBtn, &QPushButton::clicked, this, &TransferDialog::onClearAll);
    
    // Transfer controls
    connect(m_startBtn, &QPushButton::clicked, this, &TransferDialog::onStartTransfers);
    connect(m_pauseAllBtn, &QPushButton::clicked, this, &TransferDialog::onPauseAll);
    connect(m_resumeAllBtn, &QPushButton::clicked, this, &TransferDialog::onResumeAll);
    connect(m_cancelAllBtn, &QPushButton::clicked, this, &TransferDialog::onCancelAll);
    
    // Settings
    connect(m_chunkSizeSpinBox, QOverload<int>::of(&QSpinBox::valueChanged),
            this, &TransferDialog::onChunkSizeChanged);
    connect(m_maxConcurrentSpinBox, QOverload<int>::of(&QSpinBox::valueChanged),
            this, &TransferDialog::onMaxConcurrentChanged);
    connect(m_encryptionCheckBox, &QCheckBox::toggled,
            this, &TransferDialog::onEncryptionToggled);
    connect(m_compressionComboBox, QOverload<int>::of(&QComboBox::currentIndexChanged),
            this, &TransferDialog::onSettingsChanged);
    
    // File list
    connect(m_fileList, &QListWidget::itemSelectionChanged,
            this, &TransferDialog::updateButtonStates);
    
    // Transfer manager signals
    if (m_manager) {
        connect(m_manager, &FileTransferManager::transferRequested,
                this, &TransferDialog::onTransferRequested);
        connect(m_manager, &FileTransferManager::transferApproved,
                this, &TransferDialog::onTransferApproved);
        connect(m_manager, &FileTransferManager::transferRejected,
                this, &TransferDialog::onTransferRejected);
        connect(m_manager, &FileTransferManager::transferProgress,
                this, &TransferDialog::onTransferProgress);
        connect(m_manager, &FileTransferManager::transferCompleted,
                this, &TransferDialog::onTransferCompleted);
        connect(m_manager, &FileTransferManager::transferFailed,
                this, &TransferDialog::onTransferFailed);
    }
}

void TransferDialog::dragEnterEvent(QDragEnterEvent *event)
{
    if (event->mimeData()->hasUrls()) {
        event->acceptProposedAction();
    }
}

void TransferDialog::dragMoveEvent(QDragMoveEvent *event)
{
    if (event->mimeData()->hasUrls()) {
        event->acceptProposedAction();
    }
}

void TransferDialog::dropEvent(QDropEvent *event)
{
    const QMimeData *mimeData = event->mimeData();
    
    if (mimeData->hasUrls()) {
        QStringList filePaths;
        
        for (const QUrl &url : mimeData->urls()) {
            if (url.isLocalFile()) {
                QString filePath = url.toLocalFile();
                QFileInfo fileInfo(filePath);
                
                if (fileInfo.isFile()) {
                    filePaths.append(filePath);
                } else if (fileInfo.isDir()) {
                    // Add all files from directory
                    QDir dir(filePath);
                    QStringList filters;
                    filters << "*.*";
                    QFileInfoList fileList = dir.entryInfoList(filters, QDir::Files | QDir::Readable);
                    
                    for (const QFileInfo &info : fileList) {
                        filePaths.append(info.absoluteFilePath());
                    }
                }
            }
        }
        
        if (!filePaths.isEmpty()) {
            addFiles(filePaths);
        }
        
        event->acceptProposedAction();
    }
}

void TransferDialog::addFiles(const QStringList &filePaths)
{
    QStringList validFiles;
    QStringList errors;
    
    if (validateFiles(filePaths, validFiles, errors)) {
        for (const QString &filePath : validFiles) {
            if (!m_selectedFiles.contains(filePath)) {
                addFileToList(filePath);
                m_selectedFiles.append(filePath);
            }
        }
        
        updateUI();
    }
    
    if (!errors.isEmpty()) {
        QMessageBox::warning(this, tr("File Validation Errors"), 
                           errors.join("\n"));
    }
}

void TransferDialog::addFile(const QString &filePath)
{
    addFiles(QStringList() << filePath);
}

void TransferDialog::onBrowseFiles()
{
    QStringList filePaths = QFileDialog::getOpenFileNames(
        this,
        tr("Select Files to Transfer"),
        QStandardPaths::writableLocation(QStandardPaths::DocumentsLocation),
        tr("All Files (*.*)"));
    
    if (!filePaths.isEmpty()) {
        addFiles(filePaths);
    }
}

void TransferDialog::onBrowseFolder()
{
    QString folderPath = QFileDialog::getExistingDirectory(
        this,
        tr("Select Folder to Transfer"),
        QStandardPaths::writableLocation(QStandardPaths::DocumentsLocation));
    
    if (!folderPath.isEmpty()) {
        QDir dir(folderPath);
        QStringList filters;
        filters << "*.*";
        QFileInfoList fileList = dir.entryInfoList(filters, QDir::Files | QDir::Readable);
        
        QStringList filePaths;
        for (const QFileInfo &info : fileList) {
            filePaths.append(info.absoluteFilePath());
        }
        
        if (!filePaths.isEmpty()) {
            addFiles(filePaths);
        }
    }
}

void TransferDialog::onStartTransfers()
{
    if (m_selectedFiles.isEmpty()) {
        QMessageBox::information(this, tr("No Files Selected"), 
                               tr("Please select files to transfer first."));
        return;
    }
    
    startSelectedTransfers();
}

void TransferDialog::startSelectedTransfers()
{
    if (!m_manager || !m_manager->isConnected()) {
        QMessageBox::warning(this, tr("Connection Error"), 
                           tr("Not connected to transfer server."));
        return;
    }
    
    m_isTransferring = true;
    
    for (const QString &filePath : m_selectedFiles) {
        QString transferId = m_manager->requestFileUpload(filePath, m_sessionId, m_technician);
        if (!transferId.isEmpty()) {
            // Create progress widget for this transfer
            ProgressWidget *progressWidget = new ProgressWidget(transferId, filePath, this);
            m_progressWidgets[transferId] = progressWidget;
            
            // Insert before the stretch
            m_progressLayout->insertWidget(m_progressLayout->count() - 1, progressWidget);
        }
    }
    
    updateUI();
}

void TransferDialog::updateUI()
{
    // Update file count and total size
    m_totalFiles = m_selectedFiles.size();
    m_totalSize = 0;
    
    for (const QString &filePath : m_selectedFiles) {
        QFileInfo fileInfo(filePath);
        m_totalSize += fileInfo.size();
    }
    
    updateButtonStates();
    updateStatistics();
}

void TransferDialog::updateButtonStates()
{
    bool hasFiles = !m_selectedFiles.isEmpty();
    bool hasSelection = !m_fileList->selectedItems().isEmpty();
    bool isConnected = m_manager && m_manager->isConnected();
    
    m_startBtn->setEnabled(hasFiles && isConnected && !m_isTransferring);
    m_removeBtn->setEnabled(hasSelection);
    m_clearBtn->setEnabled(hasFiles);
    m_pauseAllBtn->setEnabled(m_isTransferring && m_activeTransfers > 0);
    m_resumeAllBtn->setEnabled(m_isTransferring);
    m_cancelAllBtn->setEnabled(m_isTransferring);
}

void TransferDialog::updateStatistics()
{
    m_totalFilesLabel->setText(QString::number(m_totalFiles));
    m_totalSizeLabel->setText(formatFileSize(m_totalSize));
    m_activeTransfersLabel->setText(QString::number(m_activeTransfers));
    m_completedTransfersLabel->setText(QString::number(m_completedTransfers));
    m_overallSpeedLabel->setText(formatSpeed(m_overallSpeed));
    
    // Calculate overall progress
    if (m_totalSize > 0) {
        double percentage = (double)m_totalBytesTransferred / m_totalSize * 100.0;
        m_overallProgressBar->setValue(qRound(percentage));
    } else {
        m_overallProgressBar->setValue(0);
    }
}

QString TransferDialog::formatFileSize(qint64 bytes) const
{
    const qint64 KB = 1024;
    const qint64 MB = KB * 1024;
    const qint64 GB = MB * 1024;
    
    if (bytes >= GB) {
        return QString("%1 GB").arg(bytes / (double)GB, 0, 'f', 2);
    } else if (bytes >= MB) {
        return QString("%1 MB").arg(bytes / (double)MB, 0, 'f', 2);
    } else if (bytes >= KB) {
        return QString("%1 KB").arg(bytes / (double)KB, 0, 'f', 2);
    } else {
        return QString("%1 B").arg(bytes);
    }
}

QString TransferDialog::formatSpeed(qint64 bytesPerSecond) const
{
    return formatFileSize(bytesPerSecond) + "/s";
}

QString TransferDialog::formatDuration(qint64 seconds) const
{
    if (seconds < 60) {
        return QString("%1s").arg(seconds);
    } else if (seconds < 3600) {
        return QString("%1m %2s").arg(seconds / 60).arg(seconds % 60);
    } else {
        return QString("%1h %2m").arg(seconds / 3600).arg((seconds % 3600) / 60);
    }
}

bool TransferDialog::validateFiles(const QStringList &filePaths, QStringList &validFiles, QStringList &errors)
{
    validFiles.clear();
    errors.clear();
    
    for (const QString &filePath : filePaths) {
        QFileInfo fileInfo(filePath);
        
        if (!fileInfo.exists()) {
            errors.append(tr("File does not exist: %1").arg(filePath));
            continue;
        }
        
        if (!fileInfo.isFile()) {
            errors.append(tr("Not a file: %1").arg(filePath));
            continue;
        }
        
        if (!fileInfo.isReadable()) {
            errors.append(tr("File is not readable: %1").arg(filePath));
            continue;
        }
        
        if (fileInfo.size() > MAX_FILE_SIZE) {
            errors.append(tr("File too large (max %1): %2")
                         .arg(formatFileSize(MAX_FILE_SIZE))
                         .arg(filePath));
            continue;
        }
        
        validFiles.append(filePath);
    }
    
    return !validFiles.isEmpty();
}

void TransferDialog::addFileToList(const QString &filePath)
{
    QFileInfo fileInfo(filePath);
    QString displayText = QString("%1 (%2)")
                         .arg(fileInfo.fileName())
                         .arg(formatFileSize(fileInfo.size()));
    
    QListWidgetItem *item = new QListWidgetItem(displayText);
    item->setData(Qt::UserRole, filePath);
    item->setToolTip(filePath);
    
    m_fileList->addItem(item);
}

void TransferDialog::onTransferProgress(const QString &transferId, const FileTransferProgress &progress)
{
    if (ProgressWidget *widget = findProgressWidget(transferId)) {
        widget->updateProgress(progress);
    }
    
    updateStatistics();
}

ProgressWidget* TransferDialog::findProgressWidget(const QString &transferId)
{
    return m_progressWidgets.value(transferId, nullptr);
}

void TransferDialog::onTransferCompleted(const QString &transferId, const QString &filePath)
{
    Q_UNUSED(filePath)
    
    if (ProgressWidget *widget = findProgressWidget(transferId)) {
        widget->setCompleted();
    }
    
    m_completedTransfers++;
    m_activeTransfers--;
    
    updateStatistics();
    updateButtonStates();
}

void TransferDialog::onTransferFailed(const QString &transferId, const QString &error)
{
    if (ProgressWidget *widget = findProgressWidget(transferId)) {
        widget->setFailed(error);
    }
    
    m_failedTransfers++;
    m_activeTransfers--;
    
    updateStatistics();
    updateButtonStates();
}

void TransferDialog::onTransferRequested(const QString &transferId, const FileTransferRequest &request)
{
    m_transferRequests[transferId] = request;
    showApprovalDialog(transferId, request);
}

void TransferDialog::showApprovalDialog(const QString &transferId, const FileTransferRequest &request)
{
    QString message = tr("Technician %1 wants to transfer file:\n\n"
                        "File: %2\n"
                        "Size: %3\n\n"
                        "Do you want to approve this transfer?")
                     .arg(request.technician)
                     .arg(request.filename)
                     .arg(formatFileSize(request.fileSize));
    
    int result = QMessageBox::question(this, tr("File Transfer Request"), message,
                                     QMessageBox::Yes | QMessageBox::No,
                                     QMessageBox::No);
    
    if (result == QMessageBox::Yes) {
        // Approve transfer
        if (m_manager) {
            m_manager->onTransferApprovalReceived(transferId, true, QString());
        }
    } else {
        // Reject transfer
        if (m_manager) {
            m_manager->onTransferApprovalReceived(transferId, false, tr("Transfer rejected by user"));
        }
    }
}

void TransferDialog::onTransferApproved(const QString &transferId)
{
    m_activeTransfers++;
    updateButtonStates();
}

void TransferDialog::onTransferRejected(const QString &transferId, const QString &reason)
{
    Q_UNUSED(transferId)
    Q_UNUSED(reason)
    
    updateButtonStates();
}

void TransferDialog::onRemoveSelected()
{
    QList<QListWidgetItem*> selectedItems = m_fileList->selectedItems();
    
    for (QListWidgetItem *item : selectedItems) {
        QString filePath = item->data(Qt::UserRole).toString();
        m_selectedFiles.removeAll(filePath);
        delete item;
    }
    
    updateUI();
}

void TransferDialog::onClearAll()
{
    m_fileList->clear();
    m_selectedFiles.clear();
    updateUI();
}

void TransferDialog::onPauseAll()
{
    for (auto it = m_progressWidgets.begin(); it != m_progressWidgets.end(); ++it) {
        if (m_manager) {
            m_manager->pauseTransfer(it.key());
        }
    }
}

void TransferDialog::onResumeAll()
{
    for (auto it = m_progressWidgets.begin(); it != m_progressWidgets.end(); ++it) {
        if (m_manager) {
            m_manager->resumeTransfer(it.key());
        }
    }
}

void TransferDialog::onCancelAll()
{
    for (auto it = m_progressWidgets.begin(); it != m_progressWidgets.end(); ++it) {
        if (m_manager) {
            m_manager->cancelTransfer(it.key());
        }
    }
    
    m_isTransferring = false;
    updateButtonStates();
}

void TransferDialog::onChunkSizeChanged(int size)
{
    m_chunkSize = size * 1024; // Convert KB to bytes
    if (m_manager) {
        m_manager->setChunkSize(m_chunkSize);
    }
}

void TransferDialog::onMaxConcurrentChanged(int count)
{
    m_maxConcurrentTransfers = count;
    if (m_manager) {
        m_manager->setMaxConcurrentTransfers(count);
    }
}

void TransferDialog::onEncryptionToggled(bool enabled)
{
    m_encryptionEnabled = enabled;
    if (m_manager) {
        m_manager->setEncryptionEnabled(enabled);
    }
}

void TransferDialog::onSettingsChanged()
{
    m_compressionEnabled = m_compressionComboBox->currentIndex() > 0;
    // Apply compression settings if needed
}

void TransferDialog::loadSettings()
{
    QSettings settings;
    settings.beginGroup("FileTransfer");
    
    m_chunkSize = settings.value("chunkSize", DEFAULT_CHUNK_SIZE).toInt();
    m_maxConcurrentTransfers = settings.value("maxConcurrent", DEFAULT_MAX_CONCURRENT).toInt();
    m_encryptionEnabled = settings.value("encryption", true).toBool();
    m_compressionEnabled = settings.value("compression", false).toBool();
    
    // Restore window geometry
    restoreGeometry(settings.value("geometry").toByteArray());
    
    settings.endGroup();
}

void TransferDialog::saveSettings()
{
    QSettings settings;
    settings.beginGroup("FileTransfer");
    
    settings.setValue("chunkSize", m_chunkSize);
    settings.setValue("maxConcurrent", m_maxConcurrentTransfers);
    settings.setValue("encryption", m_encryptionEnabled);
    settings.setValue("compression", m_compressionEnabled);
    
    // Save window geometry
    settings.setValue("geometry", saveGeometry());
    
    settings.endGroup();
}

void TransferDialog::closeEvent(QCloseEvent *event)
{
    if (m_isTransferring && m_activeTransfers > 0) {
        int result = QMessageBox::question(this, tr("Active Transfers"),
                                         tr("There are active file transfers. Do you want to cancel them and close?"),
                                         QMessageBox::Yes | QMessageBox::No,
                                         QMessageBox::No);
        
        if (result == QMessageBox::Yes) {
            onCancelAll();
            event->accept();
        } else {
            event->ignore();
        }
    } else {
        event->accept();
    }
}

void TransferDialog::resizeEvent(QResizeEvent *event)
{
    QDialog::resizeEvent(event);
    // Additional resize handling if needed
}

#include "transfer_dialog.moc"