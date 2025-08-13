#ifndef TRANSFER_DIALOG_H
#define TRANSFER_DIALOG_H

#include <QDialog>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QPushButton>
#include <QProgressBar>
#include <QListWidget>
#include <QFileDialog>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMimeData>
#include <QUrl>
#include <QTimer>
#include <QGroupBox>
#include <QTextEdit>
#include <QCheckBox>
#include <QSpinBox>
#include <QComboBox>
#include <QSplitter>
#include "FileTransferManager.h"

class ProgressWidget;
class QScrollArea;

class TransferDialog : public QDialog
{
    Q_OBJECT

public:
    explicit TransferDialog(FileTransferManager *manager, const QString &sessionId, 
                           const QString &technician, QWidget *parent = nullptr);
    ~TransferDialog();

    // File selection methods
    void addFiles(const QStringList &filePaths);
    void addFile(const QString &filePath);
    void removeSelectedFiles();
    void clearAllFiles();
    
    // Transfer control
    void startSelectedTransfers();
    void pauseSelectedTransfers();
    void resumeSelectedTransfers();
    void cancelSelectedTransfers();
    
    // Settings
    void setChunkSize(int size);
    void setMaxConcurrentTransfers(int count);
    void setEncryptionEnabled(bool enabled);
    
    // UI state
    void updateTransferProgress(const QString &transferId, const FileTransferProgress &progress);
    void updateTransferStatus(const QString &transferId, TransferStatus status);
    
protected:
    // Drag and drop support
    void dragEnterEvent(QDragEnterEvent *event) override;
    void dragMoveEvent(QDragMoveEvent *event) override;
    void dropEvent(QDropEvent *event) override;
    
    // Window events
    void closeEvent(QCloseEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;

private slots:
    // File operations
    void onBrowseFiles();
    void onBrowseFolder();
    void onRemoveSelected();
    void onClearAll();
    
    // Transfer operations
    void onStartTransfers();
    void onPauseAll();
    void onResumeAll();
    void onCancelAll();
    
    // Transfer manager signals
    void onTransferRequested(const QString &transferId, const FileTransferRequest &request);
    void onTransferApproved(const QString &transferId);
    void onTransferRejected(const QString &transferId, const QString &reason);
    void onTransferProgress(const QString &transferId, const FileTransferProgress &progress);
    void onTransferCompleted(const QString &transferId, const QString &filePath);
    void onTransferFailed(const QString &transferId, const QString &error);
    
    // Settings
    void onSettingsChanged();
    void onChunkSizeChanged(int size);
    void onMaxConcurrentChanged(int count);
    void onEncryptionToggled(bool enabled);
    
    // UI updates
    void updateUI();
    void updateStatistics();
    void updateButtonStates();
    
    // Approval dialog
    void showApprovalDialog(const QString &transferId, const FileTransferRequest &request);

private:
    void setupUI();
    void setupFileListArea();
    void setupProgressArea();
    void setupControlsArea();
    void setupSettingsArea();
    void setupStatusArea();
    
    void connectSignals();
    void loadSettings();
    void saveSettings();
    
    bool validateFiles(const QStringList &filePaths, QStringList &validFiles, QStringList &errors);
    QString formatFileSize(qint64 bytes) const;
    QString formatDuration(qint64 seconds) const;
    QString formatSpeed(qint64 bytesPerSecond) const;
    
    void addFileToList(const QString &filePath);
    void removeFileFromList(const QString &filePath);
    ProgressWidget* findProgressWidget(const QString &transferId);
    
    // Member variables
    FileTransferManager *m_manager;
    QString m_sessionId;
    QString m_technician;
    
    // UI components
    QVBoxLayout *m_mainLayout;
    QSplitter *m_mainSplitter;
    
    // File selection area
    QGroupBox *m_fileGroup;
    QListWidget *m_fileList;
    QPushButton *m_browseFilesBtn;
    QPushButton *m_browseFolderBtn;
    QPushButton *m_removeBtn;
    QPushButton *m_clearBtn;
    QLabel *m_dropLabel;
    
    // Progress area
    QGroupBox *m_progressGroup;
    QScrollArea *m_progressScrollArea;
    QWidget *m_progressContainer;
    QVBoxLayout *m_progressLayout;
    
    // Controls area
    QGroupBox *m_controlsGroup;
    QPushButton *m_startBtn;
    QPushButton *m_pauseAllBtn;
    QPushButton *m_resumeAllBtn;
    QPushButton *m_cancelAllBtn;
    
    // Settings area
    QGroupBox *m_settingsGroup;
    QSpinBox *m_chunkSizeSpinBox;
    QSpinBox *m_maxConcurrentSpinBox;
    QCheckBox *m_encryptionCheckBox;
    QComboBox *m_compressionComboBox;
    
    // Status area
    QGroupBox *m_statusGroup;
    QLabel *m_totalFilesLabel;
    QLabel *m_totalSizeLabel;
    QLabel *m_activeTransfersLabel;
    QLabel *m_completedTransfersLabel;
    QLabel *m_overallSpeedLabel;
    QProgressBar *m_overallProgressBar;
    
    // Data
    QStringList m_selectedFiles;
    QMap<QString, ProgressWidget*> m_progressWidgets;
    QMap<QString, FileTransferRequest> m_transferRequests;
    
    // Statistics
    int m_totalFiles;
    qint64 m_totalSize;
    int m_activeTransfers;
    int m_completedTransfers;
    int m_failedTransfers;
    qint64 m_totalBytesTransferred;
    qint64 m_overallSpeed;
    
    // Settings
    int m_chunkSize;
    int m_maxConcurrentTransfers;
    bool m_encryptionEnabled;
    bool m_compressionEnabled;
    
    // State
    bool m_isTransferring;
    QTimer *m_updateTimer;
};

#endif // TRANSFER_DIALOG_H