#include "ApprovalDialog.h"
#include <QApplication>
#include <QScreen>
#include <QDesktopServices>
#include <QUrl>
#include <QCloseEvent>
#include <QKeyEvent>
#include <QMessageBox>
#include <QMimeDatabase>
#include <QMimeType>
#include <QFileIconProvider>
#include <QStyle>
#include <QStyleOption>
#include <QPainter>
#include <QFont>
#include <QFontMetrics>
#include <QDateTime>
#include <QDebug>

// Define dangerous file extensions
const QStringList ApprovalDialog::DANGEROUS_EXTENSIONS = {
    "exe", "bat", "cmd", "com", "scr", "pif", "vbs", "vbe", "js", "jse",
    "wsf", "wsh", "msi", "msp", "hta", "cpl", "jar", "app", "deb", "rpm",
    "dmg", "pkg", "run", "bin", "sh", "ps1", "psm1", "psd1", "ps1xml"
};

ApprovalDialog::ApprovalDialog(const FileTransferRequest &request, QWidget *parent)
    : QDialog(parent)
    , m_request(request)
    , m_approved(false)
    , m_timeoutTimer(new QTimer(this))
    , m_displayTimer(new QTimer(this))
    , m_timeoutSeconds(DEFAULT_TIMEOUT)
    , m_remainingSeconds(DEFAULT_TIMEOUT)
{
    setWindowTitle(tr("File Transfer Request"));
    setWindowIcon(QIcon(":/icons/file_transfer.png"));
    setModal(true);
    setMinimumSize(MIN_DIALOG_WIDTH, MIN_DIALOG_HEIGHT);
    
    // Setup UI
    setupUI();
    connectSignals();
    
    // Apply security styling if needed
    if (isFileTypeDangerous(m_request.filename)) {
        applySecurityStyling();
    }
    
    // Center on screen
    QScreen *screen = QApplication::primaryScreen();
    if (screen) {
        QRect screenGeometry = screen->availableGeometry();
        move((screenGeometry.width() - width()) / 2,
             (screenGeometry.height() - height()) / 2);
    }
    
    // Set focus to reject button by default for security
    m_rejectButton->setFocus();
}

ApprovalDialog::~ApprovalDialog()
{
    if (m_timeoutTimer) {
        m_timeoutTimer->stop();
    }
    if (m_displayTimer) {
        m_displayTimer->stop();
    }
}

void ApprovalDialog::setAutoTimeout(int seconds)
{
    m_timeoutSeconds = seconds;
    m_remainingSeconds = seconds;
    
    if (seconds > 0) {
        m_timeoutTimer->setInterval(seconds * 1000);
        m_timeoutTimer->setSingleShot(true);
        m_timeoutTimer->start();
        
        m_displayTimer->setInterval(1000); // Update every second
        m_displayTimer->start();
        
        updateTimeoutDisplay();
        m_timeoutLabel->setVisible(true);
    } else {
        m_timeoutTimer->stop();
        m_displayTimer->stop();
        m_timeoutLabel->setVisible(false);
    }
}

void ApprovalDialog::setRememberOptionEnabled(bool enabled)
{
    m_rememberCheckBox->setVisible(enabled);
}

bool ApprovalDialog::shouldRememberDecision() const
{
    return m_rememberCheckBox->isChecked();
}

void ApprovalDialog::closeEvent(QCloseEvent *event)
{
    // Treat close as rejection
    m_approved = false;
    m_message = tr("Request cancelled by user");
    event->accept();
}

void ApprovalDialog::keyPressEvent(QKeyEvent *event)
{
    switch (event->key()) {
    case Qt::Key_Escape:
        onRejectClicked();
        break;
    case Qt::Key_Return:
    case Qt::Key_Enter:
        if (event->modifiers() & Qt::ControlModifier) {
            onApproveClicked();
        } else {
            // Default to reject for security
            onRejectClicked();
        }
        break;
    default:
        QDialog::keyPressEvent(event);
        break;
    }
}

void ApprovalDialog::onApproveClicked()
{
    m_approved = true;
    m_message = m_messageEdit->toPlainText().trimmed();
    
    if (m_message.isEmpty()) {
        m_message = tr("Approved by user");
    }
    
    accept();
}

void ApprovalDialog::onRejectClicked()
{
    m_approved = false;
    m_message = m_messageEdit->toPlainText().trimmed();
    
    if (m_message.isEmpty()) {
        m_message = tr("Rejected by user");
    }
    
    reject();
}

void ApprovalDialog::onTimeout()
{
    // Auto-reject on timeout for security
    m_approved = false;
    m_message = tr("Request timed out");
    reject();
}

void ApprovalDialog::updateTimeoutDisplay()
{
    m_remainingSeconds--;
    
    if (m_remainingSeconds <= 0) {
        m_timeoutLabel->setText(tr("Request timed out"));
        m_displayTimer->stop();
    } else {
        m_timeoutLabel->setText(tr("Auto-reject in %1 seconds").arg(m_remainingSeconds));
    }
}

void ApprovalDialog::onMessageChanged()
{
    // Enable/disable buttons based on message content if needed
    // Currently no restrictions
}

void ApprovalDialog::setupUI()
{
    m_mainLayout = new QVBoxLayout(this);
    m_mainLayout->setSpacing(12);
    m_mainLayout->setContentsMargins(16, 16, 16, 16);
    
    // Create scroll area for content
    m_scrollArea = new QScrollArea(this);
    m_scrollArea->setWidgetResizable(true);
    m_scrollArea->setFrameShape(QFrame::NoFrame);
    
    m_contentWidget = new QWidget();
    m_contentLayout = new QVBoxLayout(m_contentWidget);
    m_contentLayout->setSpacing(12);
    
    // Setup sections
    setupFileInfoSection();
    setupTechnicianInfoSection();
    setupSecuritySection();
    setupMessageSection();
    
    // Add content to scroll area
    m_scrollArea->setWidget(m_contentWidget);
    m_mainLayout->addWidget(m_scrollArea);
    
    // Setup timeout label
    m_timeoutLabel = new QLabel(this);
    m_timeoutLabel->setAlignment(Qt::AlignCenter);
    m_timeoutLabel->setStyleSheet("QLabel { color: #d32f2f; font-weight: bold; }");
    m_timeoutLabel->setVisible(false);
    m_mainLayout->addWidget(m_timeoutLabel);
    
    // Setup remember option
    m_rememberCheckBox = new QCheckBox(tr("Remember my decision for this session"), this);
    m_rememberCheckBox->setVisible(false); // Hidden by default
    m_mainLayout->addWidget(m_rememberCheckBox);
    
    // Setup buttons
    setupButtonSection();
}

void ApprovalDialog::setupFileInfoSection()
{
    m_fileInfoGroup = new QGroupBox(tr("File Information"), m_contentWidget);
    m_fileInfoLayout = new QGridLayout(m_fileInfoGroup);
    m_fileInfoLayout->setSpacing(8);
    
    // File icon
    m_fileIconLabel = new QLabel();
    QIcon fileIcon = getFileIcon(m_request.filename);
    m_fileIconLabel->setPixmap(fileIcon.pixmap(48, 48));
    m_fileIconLabel->setAlignment(Qt::AlignCenter);
    m_fileInfoLayout->addWidget(m_fileIconLabel, 0, 0, 3, 1);
    
    // File details
    int row = 0;
    
    // File name
    m_fileInfoLayout->addWidget(new QLabel(tr("<b>File Name:</b>")), row, 1);
    m_fileNameLabel = new QLabel(m_request.filename);
    m_fileNameLabel->setWordWrap(true);
    m_fileNameLabel->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_fileInfoLayout->addWidget(m_fileNameLabel, row++, 2);
    
    // File size
    m_fileInfoLayout->addWidget(new QLabel(tr("<b>File Size:</b>")), row, 1);
    m_fileSizeLabel = new QLabel(formatFileSize(m_request.fileSize));
    m_fileInfoLayout->addWidget(m_fileSizeLabel, row++, 2);
    
    // Transfer type
    m_fileInfoLayout->addWidget(new QLabel(tr("<b>Transfer Type:</b>")), row, 1);
    m_transferTypeLabel = new QLabel(getTransferTypeString(m_request.type));
    m_fileInfoLayout->addWidget(m_transferTypeLabel, row++, 2);
    
    // File type
    QMimeDatabase mimeDb;
    QMimeType mimeType = mimeDb.mimeTypeForFile(m_request.filename);
    m_fileInfoLayout->addWidget(new QLabel(tr("<b>File Type:</b>")), row, 1);
    m_fileTypeLabel = new QLabel(mimeType.comment());
    m_fileInfoLayout->addWidget(m_fileTypeLabel, row++, 2);
    
    // Checksum (if available)
    if (!m_request.checksum.isEmpty()) {
        m_fileInfoLayout->addWidget(new QLabel(tr("<b>Checksum:</b>")), row, 1);
        m_checksumLabel = new QLabel(m_request.checksum);
        m_checksumLabel->setFont(QFont("monospace"));
        m_checksumLabel->setTextInteractionFlags(Qt::TextSelectableByMouse);
        m_fileInfoLayout->addWidget(m_checksumLabel, row++, 2);
    }
    
    m_contentLayout->addWidget(m_fileInfoGroup);
}

void ApprovalDialog::setupTechnicianInfoSection()
{
    m_technicianGroup = new QGroupBox(tr("Technician Information"), m_contentWidget);
    m_technicianLayout = new QGridLayout(m_technicianGroup);
    m_technicianLayout->setSpacing(8);
    
    int row = 0;
    
    // Technician name
    m_technicianLayout->addWidget(new QLabel(tr("<b>Technician:</b>")), row, 0);
    m_technicianNameLabel = new QLabel(m_request.technician);
    m_technicianLayout->addWidget(m_technicianNameLabel, row++, 1);
    
    // Session ID
    m_technicianLayout->addWidget(new QLabel(tr("<b>Session ID:</b>")), row, 0);
    m_sessionIdLabel = new QLabel(m_request.sessionId);
    m_sessionIdLabel->setFont(QFont("monospace"));
    m_sessionIdLabel->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_technicianLayout->addWidget(m_sessionIdLabel, row++, 1);
    
    // Request time
    m_technicianLayout->addWidget(new QLabel(tr("<b>Request Time:</b>")), row, 0);
    m_requestTimeLabel = new QLabel(QDateTime::currentDateTime().toString(Qt::DefaultLocaleLongDate));
    m_technicianLayout->addWidget(m_requestTimeLabel, row++, 1);
    
    m_contentLayout->addWidget(m_technicianGroup);
}

void ApprovalDialog::setupSecuritySection()
{
    m_securityGroup = new QGroupBox(tr("Security Information"), m_contentWidget);
    m_securityLayout = new QVBoxLayout(m_securityGroup);
    
    // Security frame
    m_securityFrame = new QFrame();
    m_securityFrame->setFrameStyle(QFrame::Box | QFrame::Raised);
    m_securityFrame->setLineWidth(2);
    
    QHBoxLayout *securityFrameLayout = new QHBoxLayout(m_securityFrame);
    
    // Security icon
    m_securityIconLabel = new QLabel();
    m_securityIconLabel->setPixmap(style()->standardIcon(QStyle::SP_MessageBoxInformation).pixmap(32, 32));
    securityFrameLayout->addWidget(m_securityIconLabel);
    
    // Security warning text
    m_securityWarningLabel = new QLabel();
    m_securityWarningLabel->setWordWrap(true);
    
    if (isFileTypeDangerous(m_request.filename)) {
        m_securityWarningLabel->setText(
            tr("<b>⚠️ SECURITY WARNING:</b><br/>"
               "This file type (%1) can potentially execute code on your computer. "
               "Only approve this transfer if you trust the technician and understand the risks.")
            .arg(QFileInfo(m_request.filename).suffix().toUpper())
        );
    } else {
        m_securityWarningLabel->setText(
            tr("<b>🔒 SECURE TRANSFER:</b><br/>"
               "This file transfer will be encrypted and verified with checksums. "
               "The file type appears to be safe for transfer.")
        );
    }
    
    securityFrameLayout->addWidget(m_securityWarningLabel, 1);
    m_securityLayout->addWidget(m_securityFrame);
    
    m_contentLayout->addWidget(m_securityGroup);
}

void ApprovalDialog::setupMessageSection()
{
    m_messageGroup = new QGroupBox(tr("Message (Optional)"), m_contentWidget);
    m_messageLayout = new QVBoxLayout(m_messageGroup);
    
    // Message hint
    m_messageHintLabel = new QLabel(tr("You can add a message to explain your decision:"));
    m_messageHintLabel->setStyleSheet("QLabel { color: #666; font-style: italic; }");
    m_messageLayout->addWidget(m_messageHintLabel);
    
    // Message edit
    m_messageEdit = new QTextEdit();
    m_messageEdit->setMaximumHeight(80);
    m_messageEdit->setPlaceholderText(tr("Enter your message here..."));
    m_messageLayout->addWidget(m_messageEdit);
    
    m_contentLayout->addWidget(m_messageGroup);
}

void ApprovalDialog::setupButtonSection()
{
    m_buttonLayout = new QHBoxLayout();
    m_buttonLayout->setSpacing(12);
    
    // Details button (for future expansion)
    m_detailsButton = new QPushButton(tr("Details..."));
    m_detailsButton->setVisible(false); // Hidden for now
    m_buttonLayout->addWidget(m_detailsButton);
    
    m_buttonLayout->addStretch();
    
    // Reject button
    m_rejectButton = new QPushButton(tr("Reject"));
    m_rejectButton->setIcon(style()->standardIcon(QStyle::SP_DialogCancelButton));
    m_rejectButton->setStyleSheet(
        "QPushButton { "
        "  background-color: #f44336; "
        "  color: white; "
        "  border: none; "
        "  padding: 8px 16px; "
        "  border-radius: 4px; "
        "  font-weight: bold; "
        "} "
        "QPushButton:hover { "
        "  background-color: #d32f2f; "
        "} "
        "QPushButton:pressed { "
        "  background-color: #b71c1c; "
        "}"
    );
    m_buttonLayout->addWidget(m_rejectButton);
    
    // Approve button
    m_approveButton = new QPushButton(tr("Approve"));
    m_approveButton->setIcon(style()->standardIcon(QStyle::SP_DialogOkButton));
    m_approveButton->setStyleSheet(
        "QPushButton { "
        "  background-color: #4caf50; "
        "  color: white; "
        "  border: none; "
        "  padding: 8px 16px; "
        "  border-radius: 4px; "
        "  font-weight: bold; "
        "} "
        "QPushButton:hover { "
        "  background-color: #388e3c; "
        "} "
        "QPushButton:pressed { "
        "  background-color: #2e7d32; "
        "}"
    );
    m_buttonLayout->addWidget(m_approveButton);
    
    m_mainLayout->addLayout(m_buttonLayout);
}

void ApprovalDialog::connectSignals()
{
    connect(m_approveButton, &QPushButton::clicked, this, &ApprovalDialog::onApproveClicked);
    connect(m_rejectButton, &QPushButton::clicked, this, &ApprovalDialog::onRejectClicked);
    connect(m_timeoutTimer, &QTimer::timeout, this, &ApprovalDialog::onTimeout);
    connect(m_displayTimer, &QTimer::timeout, this, &ApprovalDialog::updateTimeoutDisplay);
    connect(m_messageEdit, &QTextEdit::textChanged, this, &ApprovalDialog::onMessageChanged);
}

QIcon ApprovalDialog::getFileIcon(const QString &filePath) const
{
    QFileIconProvider iconProvider;
    QFileInfo fileInfo(filePath);
    
    // Try to get icon from file info
    QIcon icon = iconProvider.icon(fileInfo);
    
    // Fallback to generic file icon
    if (icon.isNull()) {
        icon = style()->standardIcon(QStyle::SP_FileIcon);
    }
    
    return icon;
}

bool ApprovalDialog::isFileTypeDangerous(const QString &filePath) const
{
    QFileInfo fileInfo(filePath);
    QString extension = fileInfo.suffix().toLower();
    
    return DANGEROUS_EXTENSIONS.contains(extension);
}

QString ApprovalDialog::formatFileSize(qint64 bytes) const
{
    const qint64 KB = 1024;
    const qint64 MB = KB * 1024;
    const qint64 GB = MB * 1024;
    
    if (bytes >= GB) {
        return tr("%1 GB").arg(QString::number(bytes / (double)GB, 'f', 2));
    } else if (bytes >= MB) {
        return tr("%1 MB").arg(QString::number(bytes / (double)MB, 'f', 2));
    } else if (bytes >= KB) {
        return tr("%1 KB").arg(QString::number(bytes / (double)KB, 'f', 2));
    } else {
        return tr("%1 bytes").arg(bytes);
    }
}

QString ApprovalDialog::getTransferTypeString(TransferType type) const
{
    switch (type) {
    case TransferType::Upload:
        return tr("Upload (Technician → Your Computer)");
    case TransferType::Download:
        return tr("Download (Your Computer → Technician)");
    default:
        return tr("Unknown");
    }
}

void ApprovalDialog::applySecurityStyling()
{
    // Apply warning styling for dangerous files
    m_securityFrame->setStyleSheet(
        "QFrame { "
        "  border: 2px solid #ff9800; "
        "  background-color: #fff3e0; "
        "  border-radius: 4px; "
        "  padding: 8px; "
        "}"
    );
    
    m_securityIconLabel->setPixmap(style()->standardIcon(QStyle::SP_MessageBoxWarning).pixmap(32, 32));
    
    // Make reject button more prominent
    m_rejectButton->setDefault(true);
}