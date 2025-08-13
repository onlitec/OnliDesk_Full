#ifndef APPROVALDIALOG_H
#define APPROVALDIALOG_H

#include <QDialog>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QGridLayout>
#include <QLabel>
#include <QPushButton>
#include <QTextEdit>
#include <QGroupBox>
#include <QFrame>
#include <QTimer>
#include <QProgressBar>
#include <QCheckBox>
#include <QScrollArea>
#include <QPixmap>
#include <QIcon>
#include <QFileInfo>
#include <QMimeDatabase>
#include <QDateTime>
#include "FileTransferManager.h"

/**
 * @brief Dialog for approving/rejecting file transfer requests
 * 
 * This dialog provides a user-friendly interface for clients to review
 * and approve or reject file transfer requests from technicians.
 * Features:
 * - File information display (name, size, type, checksum)
 * - Technician information
 * - Security warnings for potentially dangerous files
 * - Auto-timeout option
 * - Detailed transfer metadata
 */
class ApprovalDialog : public QDialog
{
    Q_OBJECT

public:
    /**
     * @brief Constructor
     * @param request File transfer request to approve/reject
     * @param parent Parent widget
     */
    explicit ApprovalDialog(const FileTransferRequest &request, QWidget *parent = nullptr);
    
    /**
     * @brief Destructor
     */
    ~ApprovalDialog();
    
    /**
     * @brief Get the approval result
     * @return true if approved, false if rejected
     */
    bool isApproved() const { return m_approved; }
    
    /**
     * @brief Get the approval message
     * @return User's message for the approval/rejection
     */
    QString getMessage() const { return m_message; }
    
    /**
     * @brief Set auto-timeout for the dialog
     * @param seconds Timeout in seconds (0 to disable)
     */
    void setAutoTimeout(int seconds);
    
    /**
     * @brief Set whether to remember the decision for this session
     * @param enabled Whether to show the "remember" option
     */
    void setRememberOptionEnabled(bool enabled);
    
    /**
     * @brief Check if user wants to remember the decision
     * @return true if remember option is checked
     */
    bool shouldRememberDecision() const;

protected:
    /**
     * @brief Handle close event
     */
    void closeEvent(QCloseEvent *event) override;
    
    /**
     * @brief Handle key press events
     */
    void keyPressEvent(QKeyEvent *event) override;

private slots:
    /**
     * @brief Handle approve button click
     */
    void onApproveClicked();
    
    /**
     * @brief Handle reject button click
     */
    void onRejectClicked();
    
    /**
     * @brief Handle timeout timer
     */
    void onTimeout();
    
    /**
     * @brief Update timeout display
     */
    void updateTimeoutDisplay();
    
    /**
     * @brief Handle message text changes
     */
    void onMessageChanged();

private:
    /**
     * @brief Setup the user interface
     */
    void setupUI();
    
    /**
     * @brief Setup file information section
     */
    void setupFileInfoSection();
    
    /**
     * @brief Setup technician information section
     */
    void setupTechnicianInfoSection();
    
    /**
     * @brief Setup security warning section
     */
    void setupSecuritySection();
    
    /**
     * @brief Setup message input section
     */
    void setupMessageSection();
    
    /**
     * @brief Setup action buttons
     */
    void setupButtonSection();
    
    /**
     * @brief Connect signals and slots
     */
    void connectSignals();
    
    /**
     * @brief Get file icon based on file type
     * @param filePath Path to the file
     * @return QIcon for the file type
     */
    QIcon getFileIcon(const QString &filePath) const;
    
    /**
     * @brief Check if file type is potentially dangerous
     * @param filePath Path to the file
     * @return true if file type requires extra caution
     */
    bool isFileTypeDangerous(const QString &filePath) const;
    
    /**
     * @brief Format file size for display
     * @param bytes File size in bytes
     * @return Formatted string
     */
    QString formatFileSize(qint64 bytes) const;
    
    /**
     * @brief Get transfer type display string
     * @param type Transfer type
     * @return Human-readable string
     */
    QString getTransferTypeString(TransferType type) const;
    
    /**
     * @brief Apply security styling to dangerous files
     */
    void applySecurityStyling();

private:
    // Request data
    FileTransferRequest m_request;
    bool m_approved;
    QString m_message;
    
    // Auto-timeout
    QTimer *m_timeoutTimer;
    QTimer *m_displayTimer;
    int m_timeoutSeconds;
    int m_remainingSeconds;
    
    // UI Components
    QVBoxLayout *m_mainLayout;
    QScrollArea *m_scrollArea;
    QWidget *m_contentWidget;
    QVBoxLayout *m_contentLayout;
    
    // File information section
    QGroupBox *m_fileInfoGroup;
    QGridLayout *m_fileInfoLayout;
    QLabel *m_fileIconLabel;
    QLabel *m_fileNameLabel;
    QLabel *m_fileSizeLabel;
    QLabel *m_fileTypeLabel;
    QLabel *m_checksumLabel;
    QLabel *m_transferTypeLabel;
    
    // Technician information section
    QGroupBox *m_technicianGroup;
    QGridLayout *m_technicianLayout;
    QLabel *m_technicianNameLabel;
    QLabel *m_sessionIdLabel;
    QLabel *m_requestTimeLabel;
    
    // Security section
    QGroupBox *m_securityGroup;
    QVBoxLayout *m_securityLayout;
    QLabel *m_securityWarningLabel;
    QLabel *m_securityIconLabel;
    QFrame *m_securityFrame;
    
    // Message section
    QGroupBox *m_messageGroup;
    QVBoxLayout *m_messageLayout;
    QTextEdit *m_messageEdit;
    QLabel *m_messageHintLabel;
    
    // Options section
    QCheckBox *m_rememberCheckBox;
    QLabel *m_timeoutLabel;
    
    // Buttons
    QHBoxLayout *m_buttonLayout;
    QPushButton *m_approveButton;
    QPushButton *m_rejectButton;
    QPushButton *m_detailsButton;
    
    // Constants
    static const int DEFAULT_TIMEOUT = 60; // seconds
    static const int MIN_DIALOG_WIDTH = 500;
    static const int MIN_DIALOG_HEIGHT = 400;
    static const QStringList DANGEROUS_EXTENSIONS;
};

#endif // APPROVALDIALOG_H