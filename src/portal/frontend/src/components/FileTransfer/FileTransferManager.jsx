import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Button,
  IconButton,
  LinearProgress,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Snackbar,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Tooltip,
  Menu,
  MenuItem,
  Divider,
  Grid,
  FormControl,
  InputLabel,
  Select,
  Switch,
  FormControlLabel
} from '@mui/material';
import {
  Upload as UploadIcon,
  Download as DownloadIcon,
  Pause as PauseIcon,
  PlayArrow as ResumeIcon,
  Stop as CancelIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
  MoreVert as MoreIcon,
  CheckCircle as CompleteIcon,
  Error as ErrorIcon,
  Schedule as PendingIcon,
  CloudUpload as CloudUploadIcon,
  CloudDownload as CloudDownloadIcon,
  Security as SecurityIcon,
  Speed as SpeedIcon,
  Timer as TimerIcon,
  Storage as StorageIcon
} from '@mui/icons-material';
import { useWebSocket } from '../../hooks/useWebSocket';
import { formatBytes, formatDuration, formatSpeed } from '../../utils/formatters';
import { FileTransferService } from '../../services/FileTransferService';

const TRANSFER_STATUS = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
  IN_PROGRESS: 'in_progress',
  PAUSED: 'paused',
  COMPLETED: 'completed',
  FAILED: 'failed',
  CANCELLED: 'cancelled'
};

const STATUS_COLORS = {
  [TRANSFER_STATUS.PENDING]: 'warning',
  [TRANSFER_STATUS.APPROVED]: 'info',
  [TRANSFER_STATUS.REJECTED]: 'error',
  [TRANSFER_STATUS.IN_PROGRESS]: 'primary',
  [TRANSFER_STATUS.PAUSED]: 'default',
  [TRANSFER_STATUS.COMPLETED]: 'success',
  [TRANSFER_STATUS.FAILED]: 'error',
  [TRANSFER_STATUS.CANCELLED]: 'default'
};

const STATUS_ICONS = {
  [TRANSFER_STATUS.PENDING]: PendingIcon,
  [TRANSFER_STATUS.APPROVED]: PendingIcon,
  [TRANSFER_STATUS.REJECTED]: ErrorIcon,
  [TRANSFER_STATUS.IN_PROGRESS]: SpeedIcon,
  [TRANSFER_STATUS.PAUSED]: PauseIcon,
  [TRANSFER_STATUS.COMPLETED]: CompleteIcon,
  [TRANSFER_STATUS.FAILED]: ErrorIcon,
  [TRANSFER_STATUS.CANCELLED]: CancelIcon
};

const FileTransferManager = ({ sessionId, userRole = 'technician' }) => {
  // State management
  const [transfers, setTransfers] = useState([]);
  const [selectedTransfer, setSelectedTransfer] = useState(null);
  const [showApprovalDialog, setShowApprovalDialog] = useState(false);
  const [showUploadDialog, setShowUploadDialog] = useState(false);
  const [showDownloadDialog, setShowDownloadDialog] = useState(false);
  const [showSettingsDialog, setShowSettingsDialog] = useState(false);
  const [pendingApproval, setPendingApproval] = useState(null);
  const [notification, setNotification] = useState({ open: false, message: '', severity: 'info' });
  const [menuAnchor, setMenuAnchor] = useState(null);
  const [selectedFile, setSelectedFile] = useState(null);
  const [downloadPath, setDownloadPath] = useState('');
  const [settings, setSettings] = useState({
    autoApprove: false,
    maxFileSize: 100, // MB
    allowedExtensions: ['.txt', '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.zip', '.jpg', '.png'],
    encryptionEnabled: true,
    compressionEnabled: false
  });
  const [stats, setStats] = useState({
    totalTransfers: 0,
    activeTransfers: 0,
    completedTransfers: 0,
    failedTransfers: 0,
    totalBytesTransferred: 0,
    averageSpeed: 0
  });
  
  // Refs
  const fileInputRef = useRef(null);
  const wsRef = useRef(null);
  
  // WebSocket connection
  const { isConnected, sendMessage } = useWebSocket(
    `ws://localhost:8080/ws/filetransfer?session_id=${sessionId}&role=${userRole}`,
    {
      onMessage: handleWebSocketMessage,
      onConnect: handleWebSocketConnect,
      onDisconnect: handleWebSocketDisconnect,
      onError: handleWebSocketError
    }
  );
  
  // Effects
  useEffect(() => {
    loadTransfers();
    loadSettings();
    loadStats();
    
    // Set up periodic refresh
    const interval = setInterval(() => {
      loadStats();
    }, 5000);
    
    return () => clearInterval(interval);
  }, [sessionId]);
  
  // WebSocket event handlers
  function handleWebSocketConnect() {
    console.log('Connected to file transfer WebSocket');
    
    // Register session
    sendMessage({
      type: 'session_register',
      session_id: sessionId,
      role: userRole
    });
    
    showNotification('Connected to file transfer service', 'success');
  }
  
  function handleWebSocketDisconnect() {
    console.log('Disconnected from file transfer WebSocket');
    showNotification('Disconnected from file transfer service', 'warning');
  }
  
  function handleWebSocketError(error) {
    console.error('WebSocket error:', error);
    showNotification('Connection error: ' + error.message, 'error');
  }
  
  function handleWebSocketMessage(message) {
    try {
      const data = JSON.parse(message.data);
      
      switch (data.type) {
        case 'file_transfer_request':
          handleTransferRequest(data);
          break;
        case 'file_transfer_response':
          handleTransferResponse(data);
          break;
        case 'transfer_status_update':
          handleTransferStatusUpdate(data);
          break;
        case 'transfer_progress':
          handleTransferProgress(data);
          break;
        case 'chunk_ack':
          handleChunkAcknowledgment(data);
          break;
        case 'error':
          handleError(data);
          break;
        default:
          console.log('Unknown message type:', data.type);
      }
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error);
    }
  }
  
  // Message handlers
  function handleTransferRequest(data) {
    console.log('Transfer request received:', data);
    
    // Add to transfers list
    const transfer = {
      id: data.id,
      sessionId: data.session_id,
      filename: data.filename,
      fileSize: data.file_size,
      checksum: data.checksum,
      type: data.type,
      technician: data.technician,
      status: TRANSFER_STATUS.PENDING,
      progress: 0,
      speed: 0,
      remainingTime: 0,
      startTime: new Date().toISOString(),
      error: null
    };
    
    setTransfers(prev => [transfer, ...prev]);
    
    // Show approval dialog for technicians
    if (userRole === 'technician' && !settings.autoApprove) {
      setPendingApproval(transfer);
      setShowApprovalDialog(true);
    } else if (settings.autoApprove) {
      approveTransfer(transfer.id, true, 'Auto-approved');
    }
    
    showNotification(`New ${data.type} request: ${data.filename}`, 'info');
  }
  
  function handleTransferResponse(data) {
    console.log('Transfer response received:', data);
    
    updateTransfer(data.transfer_id, {
      status: data.status,
      message: data.message
    });
    
    if (data.status === TRANSFER_STATUS.APPROVED) {
      showNotification('Transfer approved', 'success');
    } else if (data.status === TRANSFER_STATUS.REJECTED) {
      showNotification(`Transfer rejected: ${data.message}`, 'error');
    }
  }
  
  function handleTransferStatusUpdate(data) {
    console.log('Transfer status update:', data);
    
    updateTransfer(data.transfer_id, {
      status: data.status,
      message: data.message
    });
  }
  
  function handleTransferProgress(data) {
    const progress = data.progress;
    
    updateTransfer(progress.transfer_id, {
      progress: progress.percentage,
      bytesTransferred: progress.bytes_transferred,
      speed: progress.speed,
      remainingTime: progress.remaining_time
    });
  }
  
  function handleChunkAcknowledgment(data) {
    console.log('Chunk acknowledged:', data.transfer_id, data.chunk_index);
  }
  
  function handleError(data) {
    console.error('Server error:', data);
    showNotification(`Error: ${data.message}`, 'error');
  }
  
  // API functions
  async function loadTransfers() {
    try {
      const response = await FileTransferService.getTransfers(sessionId);
      setTransfers(response.data.transfers || []);
    } catch (error) {
      console.error('Failed to load transfers:', error);
      showNotification('Failed to load transfers', 'error');
    }
  }
  
  async function loadSettings() {
    try {
      const response = await FileTransferService.getSettings();
      setSettings(prev => ({ ...prev, ...response.data }));
    } catch (error) {
      console.error('Failed to load settings:', error);
    }
  }
  
  async function loadStats() {
    try {
      const response = await FileTransferService.getStats(sessionId);
      setStats(response.data);
    } catch (error) {
      console.error('Failed to load stats:', error);
    }
  }
  
  async function saveSettings() {
    try {
      await FileTransferService.updateSettings(settings);
      showNotification('Settings saved', 'success');
      setShowSettingsDialog(false);
    } catch (error) {
      console.error('Failed to save settings:', error);
      showNotification('Failed to save settings', 'error');
    }
  }
  
  // Transfer actions
  async function approveTransfer(transferId, approved, message = '') {
    try {
      await FileTransferService.approveTransfer(transferId, approved, message);
      
      // Send WebSocket message
      sendMessage({
        type: 'transfer_approval',
        transfer_id: transferId,
        approved: approved,
        message: message
      });
      
      updateTransfer(transferId, {
        status: approved ? TRANSFER_STATUS.APPROVED : TRANSFER_STATUS.REJECTED,
        message: message
      });
      
      setShowApprovalDialog(false);
      setPendingApproval(null);
      
    } catch (error) {
      console.error('Failed to approve transfer:', error);
      showNotification('Failed to process approval', 'error');
    }
  }
  
  async function pauseTransfer(transferId) {
    try {
      await FileTransferService.pauseTransfer(transferId);
      
      sendMessage({
        type: 'transfer_control',
        transfer_id: transferId,
        action: 'pause'
      });
      
      updateTransfer(transferId, { status: TRANSFER_STATUS.PAUSED });
      showNotification('Transfer paused', 'info');
      
    } catch (error) {
      console.error('Failed to pause transfer:', error);
      showNotification('Failed to pause transfer', 'error');
    }
  }
  
  async function resumeTransfer(transferId) {
    try {
      await FileTransferService.resumeTransfer(transferId);
      
      sendMessage({
        type: 'transfer_control',
        transfer_id: transferId,
        action: 'resume'
      });
      
      updateTransfer(transferId, { status: TRANSFER_STATUS.IN_PROGRESS });
      showNotification('Transfer resumed', 'info');
      
    } catch (error) {
      console.error('Failed to resume transfer:', error);
      showNotification('Failed to resume transfer', 'error');
    }
  }
  
  async function cancelTransfer(transferId) {
    try {
      await FileTransferService.cancelTransfer(transferId);
      
      sendMessage({
        type: 'transfer_control',
        transfer_id: transferId,
        action: 'cancel'
      });
      
      updateTransfer(transferId, { status: TRANSFER_STATUS.CANCELLED });
      showNotification('Transfer cancelled', 'info');
      
    } catch (error) {
      console.error('Failed to cancel transfer:', error);
      showNotification('Failed to cancel transfer', 'error');
    }
  }
  
  async function deleteTransfer(transferId) {
    try {
      await FileTransferService.deleteTransfer(transferId);
      setTransfers(prev => prev.filter(t => t.id !== transferId));
      showNotification('Transfer deleted', 'info');
      
    } catch (error) {
      console.error('Failed to delete transfer:', error);
      showNotification('Failed to delete transfer', 'error');
    }
  }
  
  async function downloadFile(transferId) {
    try {
      const response = await FileTransferService.downloadFile(transferId);
      
      // Create download link
      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', getTransferById(transferId)?.filename || 'download');
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      
      showNotification('File downloaded', 'success');
      
    } catch (error) {
      console.error('Failed to download file:', error);
      showNotification('Failed to download file', 'error');
    }
  }
  
  // File upload
  async function handleFileUpload() {
    if (!selectedFile) {
      showNotification('Please select a file', 'warning');
      return;
    }
    
    try {
      const formData = new FormData();
      formData.append('file', selectedFile);
      formData.append('session_id', sessionId);
      formData.append('technician', userRole);
      
      const response = await FileTransferService.uploadFile(formData);
      
      showNotification('Upload started', 'success');
      setShowUploadDialog(false);
      setSelectedFile(null);
      
      // Refresh transfers
      loadTransfers();
      
    } catch (error) {
      console.error('Failed to start upload:', error);
      showNotification('Failed to start upload', 'error');
    }
  }
  
  // File download request
  async function handleFileDownload() {
    if (!downloadPath.trim()) {
      showNotification('Please enter a filename', 'warning');
      return;
    }
    
    try {
      const response = await FileTransferService.requestDownload({
        filename: downloadPath,
        session_id: sessionId,
        technician: userRole
      });
      
      showNotification('Download requested', 'success');
      setShowDownloadDialog(false);
      setDownloadPath('');
      
      // Refresh transfers
      loadTransfers();
      
    } catch (error) {
      console.error('Failed to request download:', error);
      showNotification('Failed to request download', 'error');
    }
  }
  
  // Utility functions
  function updateTransfer(transferId, updates) {
    setTransfers(prev => prev.map(transfer => 
      transfer.id === transferId 
        ? { ...transfer, ...updates }
        : transfer
    ));
  }
  
  function getTransferById(transferId) {
    return transfers.find(t => t.id === transferId);
  }
  
  function showNotification(message, severity = 'info') {
    setNotification({ open: true, message, severity });
  }
  
  function getStatusIcon(status) {
    const IconComponent = STATUS_ICONS[status] || PendingIcon;
    return <IconComponent />;
  }
  
  function canPauseResume(status) {
    return [TRANSFER_STATUS.IN_PROGRESS, TRANSFER_STATUS.PAUSED].includes(status);
  }
  
  function canCancel(status) {
    return ![TRANSFER_STATUS.COMPLETED, TRANSFER_STATUS.FAILED, TRANSFER_STATUS.CANCELLED].includes(status);
  }
  
  function canDownload(transfer) {
    return transfer.status === TRANSFER_STATUS.COMPLETED && transfer.type === 'upload';
  }
  
  function canDelete(status) {
    return [TRANSFER_STATUS.COMPLETED, TRANSFER_STATUS.FAILED, TRANSFER_STATUS.CANCELLED].includes(status);
  }
  
  // Render functions
  function renderTransferRow(transfer) {
    const StatusIcon = STATUS_ICONS[transfer.status] || PendingIcon;
    
    return (
      <TableRow key={transfer.id} hover>
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            {transfer.type === 'upload' ? <UploadIcon fontSize="small" /> : <DownloadIcon fontSize="small" />}
            <Typography variant="body2" noWrap sx={{ maxWidth: 200 }}>
              {transfer.filename}
            </Typography>
          </Box>
        </TableCell>
        
        <TableCell>
          <Typography variant="body2">
            {formatBytes(transfer.fileSize)}
          </Typography>
        </TableCell>
        
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <StatusIcon fontSize="small" />
            <Chip 
              label={transfer.status.replace('_', ' ').toUpperCase()}
              color={STATUS_COLORS[transfer.status]}
              size="small"
            />
          </Box>
        </TableCell>
        
        <TableCell>
          {transfer.status === TRANSFER_STATUS.IN_PROGRESS ? (
            <Box sx={{ width: '100%' }}>
              <LinearProgress 
                variant="determinate" 
                value={transfer.progress || 0}
                sx={{ mb: 0.5 }}
              />
              <Typography variant="caption" color="text.secondary">
                {Math.round(transfer.progress || 0)}% • {formatSpeed(transfer.speed || 0)}
              </Typography>
            </Box>
          ) : (
            <Typography variant="body2" color="text.secondary">
              {transfer.progress ? `${Math.round(transfer.progress)}%` : '-'}
            </Typography>
          )}
        </TableCell>
        
        <TableCell>
          <Typography variant="body2" color="text.secondary">
            {transfer.technician || '-'}
          </Typography>
        </TableCell>
        
        <TableCell>
          <Typography variant="body2" color="text.secondary">
            {transfer.remainingTime ? formatDuration(transfer.remainingTime) : '-'}
          </Typography>
        </TableCell>
        
        <TableCell>
          <Box sx={{ display: 'flex', gap: 0.5 }}>
            {canPauseResume(transfer.status) && (
              <>
                {transfer.status === TRANSFER_STATUS.IN_PROGRESS ? (
                  <Tooltip title="Pause">
                    <IconButton size="small" onClick={() => pauseTransfer(transfer.id)}>
                      <PauseIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                ) : (
                  <Tooltip title="Resume">
                    <IconButton size="small" onClick={() => resumeTransfer(transfer.id)}>
                      <ResumeIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                )}
              </>
            )}
            
            {canCancel(transfer.status) && (
              <Tooltip title="Cancel">
                <IconButton size="small" onClick={() => cancelTransfer(transfer.id)}>
                  <CancelIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
            
            {canDownload(transfer) && (
              <Tooltip title="Download">
                <IconButton size="small" onClick={() => downloadFile(transfer.id)}>
                  <DownloadIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
            
            {canDelete(transfer.status) && (
              <Tooltip title="Delete">
                <IconButton size="small" onClick={() => deleteTransfer(transfer.id)}>
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            )}
            
            <IconButton 
              size="small" 
              onClick={(e) => {
                setSelectedTransfer(transfer);
                setMenuAnchor(e.currentTarget);
              }}
            >
              <MoreIcon fontSize="small" />
            </IconButton>
          </Box>
        </TableCell>
      </TableRow>
    );
  }
  
  function renderStatsCards() {
    return (
      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ textAlign: 'center' }}>
              <StorageIcon color="primary" sx={{ fontSize: 40, mb: 1 }} />
              <Typography variant="h6">{stats.totalTransfers}</Typography>
              <Typography variant="body2" color="text.secondary">Total Transfers</Typography>
            </CardContent>
          </Card>
        </Grid>
        
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ textAlign: 'center' }}>
              <SpeedIcon color="info" sx={{ fontSize: 40, mb: 1 }} />
              <Typography variant="h6">{stats.activeTransfers}</Typography>
              <Typography variant="body2" color="text.secondary">Active</Typography>
            </CardContent>
          </Card>
        </Grid>
        
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ textAlign: 'center' }}>
              <CompleteIcon color="success" sx={{ fontSize: 40, mb: 1 }} />
              <Typography variant="h6">{stats.completedTransfers}</Typography>
              <Typography variant="body2" color="text.secondary">Completed</Typography>
            </CardContent>
          </Card>
        </Grid>
        
        <Grid item xs={12} sm={6} md={3}>
          <Card>
            <CardContent sx={{ textAlign: 'center' }}>
              <CloudUploadIcon color="secondary" sx={{ fontSize: 40, mb: 1 }} />
              <Typography variant="h6">{formatBytes(stats.totalBytesTransferred)}</Typography>
              <Typography variant="body2" color="text.secondary">Total Data</Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    );
  }
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Typography variant="h4" component="h1">
          File Transfer Manager
        </Typography>
        
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button
            variant="contained"
            startIcon={<UploadIcon />}
            onClick={() => setShowUploadDialog(true)}
            disabled={!isConnected}
          >
            Upload File
          </Button>
          
          <Button
            variant="outlined"
            startIcon={<DownloadIcon />}
            onClick={() => setShowDownloadDialog(true)}
            disabled={!isConnected}
          >
            Request Download
          </Button>
          
          <IconButton onClick={loadTransfers}>
            <RefreshIcon />
          </IconButton>
          
          <IconButton onClick={() => setShowSettingsDialog(true)}>
            <SecurityIcon />
          </IconButton>
        </Box>
      </Box>
      
      {/* Connection Status */}
      {!isConnected && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Not connected to file transfer service. Some features may be unavailable.
        </Alert>
      )}
      
      {/* Statistics */}
      {renderStatsCards()}
      
      {/* Transfers Table */}
      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            File Transfers
          </Typography>
          
          <TableContainer component={Paper} variant="outlined">
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>File</TableCell>
                  <TableCell>Size</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Progress</TableCell>
                  <TableCell>Technician</TableCell>
                  <TableCell>ETA</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {transfers.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} align="center">
                      <Typography variant="body2" color="text.secondary">
                        No file transfers found
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  transfers.map(renderTransferRow)
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>
      
      {/* Approval Dialog */}
      <Dialog open={showApprovalDialog} onClose={() => setShowApprovalDialog(false)}>
        <DialogTitle>Approve File Transfer</DialogTitle>
        <DialogContent>
          {pendingApproval && (
            <Box>
              <Typography variant="body1" gutterBottom>
                <strong>File:</strong> {pendingApproval.filename}
              </Typography>
              <Typography variant="body1" gutterBottom>
                <strong>Size:</strong> {formatBytes(pendingApproval.fileSize)}
              </Typography>
              <Typography variant="body1" gutterBottom>
                <strong>Type:</strong> {pendingApproval.type}
              </Typography>
              <Typography variant="body1" gutterBottom>
                <strong>From:</strong> {pendingApproval.technician}
              </Typography>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => approveTransfer(pendingApproval?.id, false, 'Rejected by technician')}>
            Reject
          </Button>
          <Button 
            variant="contained" 
            onClick={() => approveTransfer(pendingApproval?.id, true, 'Approved by technician')}
          >
            Approve
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Upload Dialog */}
      <Dialog open={showUploadDialog} onClose={() => setShowUploadDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Upload File</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 2 }}>
            <input
              ref={fileInputRef}
              type="file"
              style={{ display: 'none' }}
              onChange={(e) => setSelectedFile(e.target.files[0])}
              accept={settings.allowedExtensions.join(',')}
            />
            
            <Button
              variant="outlined"
              fullWidth
              onClick={() => fileInputRef.current?.click()}
              sx={{ mb: 2, py: 2 }}
            >
              <CloudUploadIcon sx={{ mr: 1 }} />
              {selectedFile ? selectedFile.name : 'Choose File'}
            </Button>
            
            {selectedFile && (
              <Box sx={{ mt: 2 }}>
                <Typography variant="body2" color="text.secondary">
                  <strong>Size:</strong> {formatBytes(selectedFile.size)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  <strong>Type:</strong> {selectedFile.type || 'Unknown'}
                </Typography>
              </Box>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowUploadDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleFileUpload} disabled={!selectedFile}>
            Upload
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Download Dialog */}
      <Dialog open={showDownloadDialog} onClose={() => setShowDownloadDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Request File Download</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="Filename"
            fullWidth
            variant="outlined"
            value={downloadPath}
            onChange={(e) => setDownloadPath(e.target.value)}
            placeholder="Enter filename to download"
            sx={{ mt: 2 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowDownloadDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleFileDownload} disabled={!downloadPath.trim()}>
            Request
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Settings Dialog */}
      <Dialog open={showSettingsDialog} onClose={() => setShowSettingsDialog(false)} maxWidth="md" fullWidth>
        <DialogTitle>File Transfer Settings</DialogTitle>
        <DialogContent>
          <Box sx={{ mt: 2 }}>
            <FormControlLabel
              control={
                <Switch
                  checked={settings.autoApprove}
                  onChange={(e) => setSettings(prev => ({ ...prev, autoApprove: e.target.checked }))}
                />
              }
              label="Auto-approve transfers"
            />
            
            <TextField
              label="Max File Size (MB)"
              type="number"
              fullWidth
              margin="normal"
              value={settings.maxFileSize}
              onChange={(e) => setSettings(prev => ({ ...prev, maxFileSize: parseInt(e.target.value) }))}
            />
            
            <FormControlLabel
              control={
                <Switch
                  checked={settings.encryptionEnabled}
                  onChange={(e) => setSettings(prev => ({ ...prev, encryptionEnabled: e.target.checked }))}
                />
              }
              label="Enable encryption"
            />
            
            <FormControlLabel
              control={
                <Switch
                  checked={settings.compressionEnabled}
                  onChange={(e) => setSettings(prev => ({ ...prev, compressionEnabled: e.target.checked }))}
                />
              }
              label="Enable compression"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowSettingsDialog(false)}>Cancel</Button>
          <Button variant="contained" onClick={saveSettings}>
            Save
          </Button>
        </DialogActions>
      </Dialog>
      
      {/* Context Menu */}
      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor)}
        onClose={() => setMenuAnchor(null)}
      >
        {selectedTransfer && (
          [
            <MenuItem key="details" onClick={() => {
              console.log('Transfer details:', selectedTransfer);
              setMenuAnchor(null);
            }}>
              View Details
            </MenuItem>,
            
            canDownload(selectedTransfer) && (
              <MenuItem key="download" onClick={() => {
                downloadFile(selectedTransfer.id);
                setMenuAnchor(null);
              }}>
                Download File
              </MenuItem>
            ),
            
            <Divider key="divider" />,
            
            canDelete(selectedTransfer.status) && (
              <MenuItem key="delete" onClick={() => {
                deleteTransfer(selectedTransfer.id);
                setMenuAnchor(null);
              }}>
                Delete
              </MenuItem>
            )
          ].filter(Boolean)
        )}
      </Menu>
      
      {/* Notification Snackbar */}
      <Snackbar
        open={notification.open}
        autoHideDuration={6000}
        onClose={() => setNotification(prev => ({ ...prev, open: false }))}
      >
        <Alert 
          onClose={() => setNotification(prev => ({ ...prev, open: false }))} 
          severity={notification.severity}
        >
          {notification.message}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default FileTransferManager;