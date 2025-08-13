#ifndef PROGRESS_WIDGET_H
#define PROGRESS_WIDGET_H

#include <QWidget>
#include <QProgressBar>
#include <QLabel>
#include <QPushButton>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QTimer>
#include <QFrame>
#include <QGroupBox>
#include <QGridLayout>
#include <QIcon>
#include <QPixmap>
#include <QFileInfo>
#include <QDateTime>
#include <QElapsedTimer>
#include "FileTransferManager.h"

/**
 * @brief Widget for displaying individual file transfer progress
 * 
 * This widget shows the progress of a single file transfer, including:
 * - File name and size
 * - Transfer progress bar
 * - Transfer speed and ETA
 * - Transfer status (pending, active, paused, completed, failed)
 * - Control buttons (pause, resume, cancel)
 */
class ProgressWidget : public QFrame
{
    Q_OBJECT

public:
    /**
     * @brief Transfer status enumeration
     */
    enum Status {
        Pending,
        Active,
        Paused,
        Completed,
        Failed,
        Cancelled
    };

    /**
     * @brief Constructor
     * @param transferId Unique transfer identifier
     * @param filePath Path to the file being transferred
     * @param parent Parent widget
     */
    explicit ProgressWidget(const QString &transferId, const QString &filePath, QWidget *parent = nullptr);

    /**
     * @brief Destructor
     */
    ~ProgressWidget();

    /**
     * @brief Get the transfer ID
     * @return Transfer identifier
     */
    QString transferId() const { return m_transferId; }

    /**
     * @brief Get the file path
     * @return File path
     */
    QString filePath() const { return m_filePath; }

    /**
     * @brief Get current status
     * @return Current transfer status
     */
    Status status() const { return m_status; }

    /**
     * @brief Get current progress percentage
     * @return Progress percentage (0-100)
     */
    int progressPercentage() const { return m_progressBar->value(); }

    /**
     * @brief Get bytes transferred
     * @return Number of bytes transferred
     */
    qint64 bytesTransferred() const { return m_bytesTransferred; }

    /**
     * @brief Get total file size
     * @return Total file size in bytes
     */
    qint64 totalSize() const { return m_totalSize; }

    /**
     * @brief Get current transfer speed
     * @return Transfer speed in bytes per second
     */
    qint64 transferSpeed() const { return m_transferSpeed; }

public slots:
    /**
     * @brief Update transfer progress
     * @param progress Progress information
     */
    void updateProgress(const FileTransferProgress &progress);

    /**
     * @brief Set transfer as completed
     */
    void setCompleted();

    /**
     * @brief Set transfer as failed
     * @param errorMessage Error message
     */
    void setFailed(const QString &errorMessage);

    /**
     * @brief Set transfer as cancelled
     */
    void setCancelled();

    /**
     * @brief Set transfer as paused
     */
    void setPaused();

    /**
     * @brief Set transfer as resumed/active
     */
    void setResumed();

    /**
     * @brief Reset the widget for a new transfer
     */
    void reset();

signals:
    /**
     * @brief Emitted when pause button is clicked
     * @param transferId Transfer identifier
     */
    void pauseRequested(const QString &transferId);

    /**
     * @brief Emitted when resume button is clicked
     * @param transferId Transfer identifier
     */
    void resumeRequested(const QString &transferId);

    /**
     * @brief Emitted when cancel button is clicked
     * @param transferId Transfer identifier
     */
    void cancelRequested(const QString &transferId);

    /**
     * @brief Emitted when retry button is clicked
     * @param transferId Transfer identifier
     */
    void retryRequested(const QString &transferId);

    /**
     * @brief Emitted when remove button is clicked
     * @param transferId Transfer identifier
     */
    void removeRequested(const QString &transferId);

protected:
    /**
     * @brief Handle mouse double-click events
     * @param event Mouse event
     */
    void mouseDoubleClickEvent(QMouseEvent *event) override;

    /**
     * @brief Handle context menu events
     * @param event Context menu event
     */
    void contextMenuEvent(QContextMenuEvent *event) override;

private slots:
    /**
     * @brief Handle pause button click
     */
    void onPauseClicked();

    /**
     * @brief Handle resume button click
     */
    void onResumeClicked();

    /**
     * @brief Handle cancel button click
     */
    void onCancelClicked();

    /**
     * @brief Handle retry button click
     */
    void onRetryClicked();

    /**
     * @brief Handle remove button click
     */
    void onRemoveClicked();

    /**
     * @brief Update display information
     */
    void updateDisplay();

    /**
     * @brief Update ETA calculation
     */
    void updateETA();

private:
    /**
     * @brief Setup the user interface
     */
    void setupUI();

    /**
     * @brief Connect signals and slots
     */
    void connectSignals();

    /**
     * @brief Update button states based on current status
     */
    void updateButtonStates();

    /**
     * @brief Update status display
     */
    void updateStatusDisplay();

    /**
     * @brief Format file size for display
     * @param bytes Size in bytes
     * @return Formatted size string
     */
    QString formatFileSize(qint64 bytes) const;

    /**
     * @brief Format transfer speed for display
     * @param bytesPerSecond Speed in bytes per second
     * @return Formatted speed string
     */
    QString formatSpeed(qint64 bytesPerSecond) const;

    /**
     * @brief Format duration for display
     * @param seconds Duration in seconds
     * @return Formatted duration string
     */
    QString formatDuration(qint64 seconds) const;

    /**
     * @brief Get status icon for current status
     * @return Status icon
     */
    QIcon getStatusIcon() const;

    /**
     * @brief Get status text for current status
     * @return Status text
     */
    QString getStatusText() const;

    /**
     * @brief Calculate estimated time of arrival
     * @return ETA in seconds, -1 if unknown
     */
    qint64 calculateETA() const;

    // Transfer information
    QString m_transferId;
    QString m_filePath;
    QString m_fileName;
    qint64 m_totalSize;
    qint64 m_bytesTransferred;
    qint64 m_transferSpeed;
    Status m_status;
    QString m_errorMessage;
    QDateTime m_startTime;
    QDateTime m_lastUpdateTime;
    QElapsedTimer m_elapsedTimer;
    
    // Speed calculation
    qint64 m_lastBytesTransferred;
    QDateTime m_lastSpeedUpdate;
    QList<qint64> m_speedHistory;
    static const int SPEED_HISTORY_SIZE = 10;
    
    // UI components
    QVBoxLayout *m_mainLayout;
    QHBoxLayout *m_topLayout;
    QHBoxLayout *m_bottomLayout;
    QGridLayout *m_infoLayout;
    
    // File information
    QLabel *m_fileIconLabel;
    QLabel *m_fileNameLabel;
    QLabel *m_fileSizeLabel;
    
    // Progress information
    QProgressBar *m_progressBar;
    QLabel *m_progressLabel;
    QLabel *m_speedLabel;
    QLabel *m_etaLabel;
    QLabel *m_statusLabel;
    QLabel *m_statusIconLabel;
    
    // Control buttons
    QPushButton *m_pauseBtn;
    QPushButton *m_resumeBtn;
    QPushButton *m_cancelBtn;
    QPushButton *m_retryBtn;
    QPushButton *m_removeBtn;
    
    // Update timer
    QTimer *m_updateTimer;
    
    // Constants
    static const int UPDATE_INTERVAL = 1000; // 1 second
    static const int PROGRESS_BAR_HEIGHT = 20;
    static const int BUTTON_SIZE = 24;
};

#endif // PROGRESS_WIDGET_H