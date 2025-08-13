#include "progress_widget.h"
#include <QApplication>
#include <QStyle>
#include <QMouseEvent>
#include <QContextMenuEvent>
#include <QMenu>
#include <QAction>
#include <QToolTip>
#include <QDesktopServices>
#include <QUrl>
#include <QDir>
#include <QDebug>
#include <QStyleOption>
#include <QPainter>
#include <QFontMetrics>
#include <QClipboard>
#include <QMimeData>

ProgressWidget::ProgressWidget(const QString &transferId, const QString &filePath, QWidget *parent)
    : QFrame(parent)
    , m_transferId(transferId)
    , m_filePath(filePath)
    , m_totalSize(0)
    , m_bytesTransferred(0)
    , m_transferSpeed(0)
    , m_status(Pending)
    , m_lastBytesTransferred(0)
    , m_mainLayout(nullptr)
    , m_topLayout(nullptr)
    , m_bottomLayout(nullptr)
    , m_infoLayout(nullptr)
    , m_fileIconLabel(nullptr)
    , m_fileNameLabel(nullptr)
    , m_fileSizeLabel(nullptr)
    , m_progressBar(nullptr)
    , m_progressLabel(nullptr)
    , m_speedLabel(nullptr)
    , m_etaLabel(nullptr)
    , m_statusLabel(nullptr)
    , m_statusIconLabel(nullptr)
    , m_pauseBtn(nullptr)
    , m_resumeBtn(nullptr)
    , m_cancelBtn(nullptr)
    , m_retryBtn(nullptr)
    , m_removeBtn(nullptr)
    , m_updateTimer(new QTimer(this))
{
    QFileInfo fileInfo(filePath);
    m_fileName = fileInfo.fileName();
    m_totalSize = fileInfo.size();
    
    m_startTime = QDateTime::currentDateTime();
    m_lastUpdateTime = m_startTime;
    m_lastSpeedUpdate = m_startTime;
    m_elapsedTimer.start();
    
    setupUI();
    connectSignals();
    
    // Setup update timer
    m_updateTimer->setInterval(UPDATE_INTERVAL);
    connect(m_updateTimer, &QTimer::timeout, this, &ProgressWidget::updateDisplay);
    m_updateTimer->start();
    
    updateDisplay();
    updateButtonStates();
}

ProgressWidget::~ProgressWidget()
{
    if (m_updateTimer) {
        m_updateTimer->stop();
    }
}

void ProgressWidget::setupUI()
{
    setFrameStyle(QFrame::StyledPanel | QFrame::Raised);
    setLineWidth(1);
    setContentsMargins(8, 8, 8, 8);
    
    // Set minimum and maximum sizes
    setMinimumHeight(80);
    setMaximumHeight(120);
    
    // Main layout
    m_mainLayout = new QVBoxLayout(this);
    m_mainLayout->setSpacing(4);
    m_mainLayout->setContentsMargins(4, 4, 4, 4);
    
    // Top layout (file info and controls)
    m_topLayout = new QHBoxLayout();
    m_topLayout->setSpacing(8);
    
    // File icon
    m_fileIconLabel = new QLabel();
    m_fileIconLabel->setFixedSize(32, 32);
    m_fileIconLabel->setScaledContents(true);
    m_fileIconLabel->setPixmap(getFileIcon().pixmap(32, 32));
    m_topLayout->addWidget(m_fileIconLabel);
    
    // File information layout
    m_infoLayout = new QGridLayout();
    m_infoLayout->setSpacing(2);
    
    // File name
    m_fileNameLabel = new QLabel(m_fileName);
    m_fileNameLabel->setFont(QFont(m_fileNameLabel->font().family(), -1, QFont::Bold));
    m_fileNameLabel->setToolTip(m_filePath);
    m_fileNameLabel->setWordWrap(false);
    m_fileNameLabel->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Preferred);
    
    // Elide text if too long
    QFontMetrics fm(m_fileNameLabel->font());
    QString elidedText = fm.elidedText(m_fileName, Qt::ElideMiddle, 300);
    m_fileNameLabel->setText(elidedText);
    
    m_infoLayout->addWidget(m_fileNameLabel, 0, 0, 1, 2);
    
    // File size
    m_fileSizeLabel = new QLabel(formatFileSize(m_totalSize));
    m_fileSizeLabel->setStyleSheet("color: #666;");
    m_infoLayout->addWidget(m_fileSizeLabel, 1, 0);
    
    // Status
    QHBoxLayout *statusLayout = new QHBoxLayout();
    statusLayout->setSpacing(4);
    
    m_statusIconLabel = new QLabel();
    m_statusIconLabel->setFixedSize(16, 16);
    m_statusIconLabel->setScaledContents(true);
    statusLayout->addWidget(m_statusIconLabel);
    
    m_statusLabel = new QLabel(getStatusText());
    m_statusLabel->setStyleSheet("color: #666;");
    statusLayout->addWidget(m_statusLabel);
    statusLayout->addStretch();
    
    QWidget *statusWidget = new QWidget();
    statusWidget->setLayout(statusLayout);
    m_infoLayout->addWidget(statusWidget, 1, 1);
    
    m_topLayout->addLayout(m_infoLayout, 1);
    
    // Control buttons
    QHBoxLayout *buttonLayout = new QHBoxLayout();
    buttonLayout->setSpacing(2);
    
    m_pauseBtn = new QPushButton();
    m_pauseBtn->setIcon(QIcon(":/icons/pause.png"));
    m_pauseBtn->setFixedSize(BUTTON_SIZE, BUTTON_SIZE);
    m_pauseBtn->setToolTip(tr("Pause Transfer"));
    m_pauseBtn->setFlat(true);
    buttonLayout->addWidget(m_pauseBtn);
    
    m_resumeBtn = new QPushButton();
    m_resumeBtn->setIcon(QIcon(":/icons/resume.png"));
    m_resumeBtn->setFixedSize(BUTTON_SIZE, BUTTON_SIZE);
    m_resumeBtn->setToolTip(tr("Resume Transfer"));
    m_resumeBtn->setFlat(true);
    buttonLayout->addWidget(m_resumeBtn);
    
    m_cancelBtn = new QPushButton();
    m_cancelBtn->setIcon(QIcon(":/icons/stop.png"));
    m_cancelBtn->setFixedSize(BUTTON_SIZE, BUTTON_SIZE);
    m_cancelBtn->setToolTip(tr("Cancel Transfer"));
    m_cancelBtn->setFlat(true);
    buttonLayout->addWidget(m_cancelBtn);
    
    m_retryBtn = new QPushButton();
    m_retryBtn->setIcon(QIcon(":/icons/retry.png"));
    m_retryBtn->setFixedSize(BUTTON_SIZE, BUTTON_SIZE);
    m_retryBtn->setToolTip(tr("Retry Transfer"));
    m_retryBtn->setFlat(true);
    buttonLayout->addWidget(m_retryBtn);
    
    m_removeBtn = new QPushButton();
    m_removeBtn->setIcon(QIcon(":/icons/remove.png"));
    m_removeBtn->setFixedSize(BUTTON_SIZE, BUTTON_SIZE);
    m_removeBtn->setToolTip(tr("Remove from List"));
    m_removeBtn->setFlat(true);
    buttonLayout->addWidget(m_removeBtn);
    
    m_topLayout->addLayout(buttonLayout);
    
    m_mainLayout->addLayout(m_topLayout);
    
    // Progress bar
    m_progressBar = new QProgressBar();
    m_progressBar->setRange(0, 100);
    m_progressBar->setValue(0);
    m_progressBar->setFixedHeight(PROGRESS_BAR_HEIGHT);
    m_progressBar->setTextVisible(true);
    m_mainLayout->addWidget(m_progressBar);
    
    // Bottom layout (progress details)
    m_bottomLayout = new QHBoxLayout();
    m_bottomLayout->setSpacing(8);
    
    // Progress label
    m_progressLabel = new QLabel("0 B / " + formatFileSize(m_totalSize));
    m_progressLabel->setStyleSheet("color: #666; font-size: 11px;");
    m_bottomLayout->addWidget(m_progressLabel);
    
    m_bottomLayout->addStretch();
    
    // Speed label
    m_speedLabel = new QLabel("0 B/s");
    m_speedLabel->setStyleSheet("color: #666; font-size: 11px;");
    m_bottomLayout->addWidget(m_speedLabel);
    
    // ETA label
    m_etaLabel = new QLabel("--:--");
    m_etaLabel->setStyleSheet("color: #666; font-size: 11px;");
    m_bottomLayout->addWidget(m_etaLabel);
    
    m_mainLayout->addLayout(m_bottomLayout);
}

void ProgressWidget::connectSignals()
{
    connect(m_pauseBtn, &QPushButton::clicked, this, &ProgressWidget::onPauseClicked);
    connect(m_resumeBtn, &QPushButton::clicked, this, &ProgressWidget::onResumeClicked);
    connect(m_cancelBtn, &QPushButton::clicked, this, &ProgressWidget::onCancelClicked);
    connect(m_retryBtn, &QPushButton::clicked, this, &ProgressWidget::onRetryClicked);
    connect(m_removeBtn, &QPushButton::clicked, this, &ProgressWidget::onRemoveClicked);
}

void ProgressWidget::updateProgress(const FileTransferProgress &progress)
{
    m_bytesTransferred = progress.bytesTransferred;
    m_totalSize = progress.totalSize;
    
    // Calculate progress percentage
    int percentage = 0;
    if (m_totalSize > 0) {
        percentage = qRound((double)m_bytesTransferred / m_totalSize * 100.0);
    }
    
    m_progressBar->setValue(percentage);
    
    // Update speed calculation
    QDateTime currentTime = QDateTime::currentDateTime();
    qint64 timeDiff = m_lastSpeedUpdate.msecsTo(currentTime);
    
    if (timeDiff >= 1000) { // Update speed every second
        qint64 bytesDiff = m_bytesTransferred - m_lastBytesTransferred;
        if (timeDiff > 0) {
            m_transferSpeed = (bytesDiff * 1000) / timeDiff; // bytes per second
            
            // Add to speed history for smoothing
            m_speedHistory.append(m_transferSpeed);
            if (m_speedHistory.size() > SPEED_HISTORY_SIZE) {
                m_speedHistory.removeFirst();
            }
            
            // Calculate average speed
            qint64 totalSpeed = 0;
            for (qint64 speed : m_speedHistory) {
                totalSpeed += speed;
            }
            m_transferSpeed = totalSpeed / m_speedHistory.size();
        }
        
        m_lastBytesTransferred = m_bytesTransferred;
        m_lastSpeedUpdate = currentTime;
    }
    
    m_lastUpdateTime = currentTime;
    
    if (m_status != Active) {
        m_status = Active;
        updateStatusDisplay();
        updateButtonStates();
    }
    
    updateDisplay();
}

void ProgressWidget::setCompleted()
{
    m_status = Completed;
    m_progressBar->setValue(100);
    m_transferSpeed = 0;
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
    
    if (m_updateTimer) {
        m_updateTimer->stop();
    }
}

void ProgressWidget::setFailed(const QString &errorMessage)
{
    m_status = Failed;
    m_errorMessage = errorMessage;
    m_transferSpeed = 0;
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
    
    if (m_updateTimer) {
        m_updateTimer->stop();
    }
}

void ProgressWidget::setCancelled()
{
    m_status = Cancelled;
    m_transferSpeed = 0;
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
    
    if (m_updateTimer) {
        m_updateTimer->stop();
    }
}

void ProgressWidget::setPaused()
{
    m_status = Paused;
    m_transferSpeed = 0;
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
}

void ProgressWidget::setResumed()
{
    m_status = Active;
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
    
    if (m_updateTimer && !m_updateTimer->isActive()) {
        m_updateTimer->start();
    }
}

void ProgressWidget::reset()
{
    m_bytesTransferred = 0;
    m_transferSpeed = 0;
    m_status = Pending;
    m_errorMessage.clear();
    m_speedHistory.clear();
    
    m_startTime = QDateTime::currentDateTime();
    m_lastUpdateTime = m_startTime;
    m_lastSpeedUpdate = m_startTime;
    m_lastBytesTransferred = 0;
    m_elapsedTimer.restart();
    
    m_progressBar->setValue(0);
    
    updateStatusDisplay();
    updateButtonStates();
    updateDisplay();
    
    if (m_updateTimer && !m_updateTimer->isActive()) {
        m_updateTimer->start();
    }
}

void ProgressWidget::onPauseClicked()
{
    emit pauseRequested(m_transferId);
}

void ProgressWidget::onResumeClicked()
{
    emit resumeRequested(m_transferId);
}

void ProgressWidget::onCancelClicked()
{
    emit cancelRequested(m_transferId);
}

void ProgressWidget::onRetryClicked()
{
    reset();
    emit retryRequested(m_transferId);
}

void ProgressWidget::onRemoveClicked()
{
    emit removeRequested(m_transferId);
}

void ProgressWidget::updateDisplay()
{
    // Update progress label
    QString progressText = QString("%1 / %2")
                          .arg(formatFileSize(m_bytesTransferred))
                          .arg(formatFileSize(m_totalSize));
    m_progressLabel->setText(progressText);
    
    // Update speed label
    m_speedLabel->setText(formatSpeed(m_transferSpeed));
    
    // Update ETA
    updateETA();
    
    // Update progress bar text
    if (m_totalSize > 0) {
        int percentage = qRound((double)m_bytesTransferred / m_totalSize * 100.0);
        m_progressBar->setFormat(QString("%1%").arg(percentage));
    } else {
        m_progressBar->setFormat("0%");
    }
}

void ProgressWidget::updateETA()
{
    qint64 eta = calculateETA();
    
    if (eta > 0 && m_status == Active && m_transferSpeed > 0) {
        m_etaLabel->setText(formatDuration(eta));
    } else {
        m_etaLabel->setText("--:--");
    }
}

void ProgressWidget::updateButtonStates()
{
    m_pauseBtn->setVisible(m_status == Active);
    m_resumeBtn->setVisible(m_status == Paused);
    m_cancelBtn->setVisible(m_status == Active || m_status == Paused || m_status == Pending);
    m_retryBtn->setVisible(m_status == Failed);
    m_removeBtn->setVisible(m_status == Completed || m_status == Failed || m_status == Cancelled);
    
    m_pauseBtn->setEnabled(m_status == Active);
    m_resumeBtn->setEnabled(m_status == Paused);
    m_cancelBtn->setEnabled(m_status == Active || m_status == Paused || m_status == Pending);
    m_retryBtn->setEnabled(m_status == Failed);
    m_removeBtn->setEnabled(m_status == Completed || m_status == Failed || m_status == Cancelled);
}

void ProgressWidget::updateStatusDisplay()
{
    m_statusLabel->setText(getStatusText());
    m_statusIconLabel->setPixmap(getStatusIcon().pixmap(16, 16));
    
    // Update widget style based on status
    QString styleSheet;
    switch (m_status) {
        case Active:
            styleSheet = "QFrame { border: 1px solid #4CAF50; background-color: #E8F5E8; }";
            break;
        case Completed:
            styleSheet = "QFrame { border: 1px solid #2196F3; background-color: #E3F2FD; }";
            break;
        case Failed:
            styleSheet = "QFrame { border: 1px solid #F44336; background-color: #FFEBEE; }";
            break;
        case Paused:
            styleSheet = "QFrame { border: 1px solid #FF9800; background-color: #FFF3E0; }";
            break;
        case Cancelled:
            styleSheet = "QFrame { border: 1px solid #9E9E9E; background-color: #F5F5F5; }";
            break;
        default:
            styleSheet = "QFrame { border: 1px solid #E0E0E0; background-color: #FAFAFA; }";
            break;
    }
    setStyleSheet(styleSheet);
}

QString ProgressWidget::formatFileSize(qint64 bytes) const
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

QString ProgressWidget::formatSpeed(qint64 bytesPerSecond) const
{
    return formatFileSize(bytesPerSecond) + "/s";
}

QString ProgressWidget::formatDuration(qint64 seconds) const
{
    if (seconds < 60) {
        return QString("%1s").arg(seconds);
    } else if (seconds < 3600) {
        return QString("%1:%2").arg(seconds / 60).arg(seconds % 60, 2, 10, QChar('0'));
    } else {
        qint64 hours = seconds / 3600;
        qint64 minutes = (seconds % 3600) / 60;
        return QString("%1:%2:%3")
               .arg(hours)
               .arg(minutes, 2, 10, QChar('0'))
               .arg(seconds % 60, 2, 10, QChar('0'));
    }
}

QIcon ProgressWidget::getStatusIcon() const
{
    switch (m_status) {
        case Active:
            return QIcon(":/icons/transfer_active.png");
        case Completed:
            return QIcon(":/icons/transfer_completed.png");
        case Failed:
            return QIcon(":/icons/transfer_failed.png");
        case Paused:
            return QIcon(":/icons/transfer_paused.png");
        case Cancelled:
            return QIcon(":/icons/transfer_cancelled.png");
        default:
            return QIcon(":/icons/transfer_pending.png");
    }
}

QString ProgressWidget::getStatusText() const
{
    switch (m_status) {
        case Pending:
            return tr("Pending");
        case Active:
            return tr("Transferring");
        case Paused:
            return tr("Paused");
        case Completed:
            return tr("Completed");
        case Failed:
            return tr("Failed: %1").arg(m_errorMessage);
        case Cancelled:
            return tr("Cancelled");
        default:
            return tr("Unknown");
    }
}

qint64 ProgressWidget::calculateETA() const
{
    if (m_transferSpeed <= 0 || m_status != Active) {
        return -1;
    }
    
    qint64 remainingBytes = m_totalSize - m_bytesTransferred;
    if (remainingBytes <= 0) {
        return 0;
    }
    
    return remainingBytes / m_transferSpeed;
}

QIcon ProgressWidget::getFileIcon() const
{
    QFileInfo fileInfo(m_filePath);
    QString suffix = fileInfo.suffix().toLower();
    
    // Return appropriate icon based on file extension
    if (suffix == "txt" || suffix == "log") {
        return QIcon(":/icons/file_text.png");
    } else if (suffix == "pdf") {
        return QIcon(":/icons/file_pdf.png");
    } else if (suffix == "doc" || suffix == "docx") {
        return QIcon(":/icons/file_word.png");
    } else if (suffix == "xls" || suffix == "xlsx") {
        return QIcon(":/icons/file_excel.png");
    } else if (suffix == "ppt" || suffix == "pptx") {
        return QIcon(":/icons/file_powerpoint.png");
    } else if (suffix == "jpg" || suffix == "jpeg" || suffix == "png" || suffix == "gif" || suffix == "bmp") {
        return QIcon(":/icons/file_image.png");
    } else if (suffix == "mp3" || suffix == "wav" || suffix == "flac" || suffix == "ogg") {
        return QIcon(":/icons/file_audio.png");
    } else if (suffix == "mp4" || suffix == "avi" || suffix == "mkv" || suffix == "mov") {
        return QIcon(":/icons/file_video.png");
    } else if (suffix == "zip" || suffix == "rar" || suffix == "7z" || suffix == "tar" || suffix == "gz") {
        return QIcon(":/icons/file_archive.png");
    } else {
        return QIcon(":/icons/file_generic.png");
    }
}

void ProgressWidget::mouseDoubleClickEvent(QMouseEvent *event)
{
    Q_UNUSED(event)
    
    // Open file location on double-click if completed
    if (m_status == Completed) {
        QFileInfo fileInfo(m_filePath);
        if (fileInfo.exists()) {
            QDesktopServices::openUrl(QUrl::fromLocalFile(fileInfo.absolutePath()));
        }
    }
}

void ProgressWidget::contextMenuEvent(QContextMenuEvent *event)
{
    QMenu contextMenu(this);
    
    // Copy file path
    QAction *copyPathAction = contextMenu.addAction(QIcon(":/icons/copy.png"), tr("Copy File Path"));
    connect(copyPathAction, &QAction::triggered, [this]() {
        QClipboard *clipboard = QApplication::clipboard();
        clipboard->setText(m_filePath);
    });
    
    // Open file location
    if (m_status == Completed) {
        QAction *openLocationAction = contextMenu.addAction(QIcon(":/icons/folder.png"), tr("Open File Location"));
        connect(openLocationAction, &QAction::triggered, [this]() {
            QFileInfo fileInfo(m_filePath);
            if (fileInfo.exists()) {
                QDesktopServices::openUrl(QUrl::fromLocalFile(fileInfo.absolutePath()));
            }
        });
    }
    
    contextMenu.addSeparator();
    
    // Transfer actions based on status
    if (m_status == Active) {
        QAction *pauseAction = contextMenu.addAction(QIcon(":/icons/pause.png"), tr("Pause"));
        connect(pauseAction, &QAction::triggered, this, &ProgressWidget::onPauseClicked);
    } else if (m_status == Paused) {
        QAction *resumeAction = contextMenu.addAction(QIcon(":/icons/resume.png"), tr("Resume"));
        connect(resumeAction, &QAction::triggered, this, &ProgressWidget::onResumeClicked);
    } else if (m_status == Failed) {
        QAction *retryAction = contextMenu.addAction(QIcon(":/icons/retry.png"), tr("Retry"));
        connect(retryAction, &QAction::triggered, this, &ProgressWidget::onRetryClicked);
    }
    
    if (m_status == Active || m_status == Paused || m_status == Pending) {
        QAction *cancelAction = contextMenu.addAction(QIcon(":/icons/stop.png"), tr("Cancel"));
        connect(cancelAction, &QAction::triggered, this, &ProgressWidget::onCancelClicked);
    }
    
    if (m_status == Completed || m_status == Failed || m_status == Cancelled) {
        QAction *removeAction = contextMenu.addAction(QIcon(":/icons/remove.png"), tr("Remove from List"));
        connect(removeAction, &QAction::triggered, this, &ProgressWidget::onRemoveClicked);
    }
    
    contextMenu.exec(event->globalPos());
}

#include "progress_widget.moc"