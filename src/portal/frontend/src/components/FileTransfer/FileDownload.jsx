import React, { useState, useEffect, useCallback } from 'react';
import {
  Box,
  Button,
  Typography,
  TextField,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  ListItemSecondaryAction,
  IconButton,
  Chip,
  Paper,
  Divider,
  Autocomplete,
  CircularProgress,
  Tooltip
} from '@mui/material';
import {
  Download as DownloadIcon,
  Folder as FolderIcon,
  InsertDriveFile as FileIcon,
  Refresh as RefreshIcon,
  Search as SearchIcon,
  GetApp as GetAppIcon,
  Warning as WarningIcon,
  Info as InfoIcon
} from '@mui/icons-material';
import FileTransferService from '../../services/FileTransferService';

const FileDownload = ({ 
  onDownload, 
  sessionId,
  technician = 'Unknown',
  disabled = false 
}) => {
  const [availableFiles, setAvailableFiles] = useState([]);
  const [filteredFiles, setFilteredFiles] = useState([]);
  const [selectedFile, setSelectedFile] = useState('');
  const [customPath, setCustomPath] = useState('');
  const [searchTerm, setSearchTerm] = useState('');
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState(null);
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [downloadDescription, setDownloadDescription] = useState('');
  const [fileToDownload, setFileToDownload] = useState(null);

  /**
   * Load available files from the client
   */
  const loadAvailableFiles = useCallback(async () => {
    if (!sessionId) return;

    try {
      setRefreshing(true);
      setError(null);
      
      const response = await FileTransferService.getAvailableFiles(sessionId);
      const files = response.data || [];
      
      setAvailableFiles(files);
      setFilteredFiles(files);
    } catch (error) {
      console.error('Failed to load available files:', error);
      setError(error.message || 'Failed to load available files');
      setAvailableFiles([]);
      setFilteredFiles([]);
    } finally {
      setRefreshing(false);
    }
  }, [sessionId]);

  /**
   * Filter files based on search term
   */
  useEffect(() => {
    if (!searchTerm.trim()) {
      setFilteredFiles(availableFiles);
    } else {
      const filtered = availableFiles.filter(file => 
        file.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        file.path.toLowerCase().includes(searchTerm.toLowerCase())
      );
      setFilteredFiles(filtered);
    }
  }, [availableFiles, searchTerm]);

  /**
   * Handle file selection from list
   */
  const handleFileSelect = useCallback((file) => {
    setSelectedFile(file.path);
    setCustomPath(file.path);
  }, []);

  /**
   * Validate file path
   */
  const validateFilePath = useCallback((path) => {
    if (!path || path.trim() === '') {
      return { valid: false, error: 'File path is required' };
    }

    // Basic path validation
    const invalidChars = /[<>:"|?*]/;
    if (invalidChars.test(path)) {
      return { valid: false, error: 'File path contains invalid characters' };
    }

    // Check path length
    if (path.length > 260) {
      return { valid: false, error: 'File path is too long (max 260 characters)' };
    }

    return { valid: true };
  }, []);

  /**
   * Start download request
   */
  const startDownload = useCallback(() => {
    const path = customPath.trim();
    
    if (!path) {
      setError('Please enter a file path or select a file');
      return;
    }

    const validation = validateFilePath(path);
    if (!validation.valid) {
      setError(validation.error);
      return;
    }

    // Extract filename from path
    const filename = path.split(/[\\/]/).pop();
    
    setFileToDownload({
      path,
      filename,
      size: getFileSize(path)
    });
    setShowConfirmDialog(true);
  }, [customPath, validateFilePath]);

  /**
   * Get file size from available files list
   */
  const getFileSize = useCallback((path) => {
    const file = availableFiles.find(f => f.path === path);
    return file ? file.size : null;
  }, [availableFiles]);

  /**
   * Perform the actual download request
   */
  const performDownload = useCallback(async () => {
    if (!fileToDownload) return;

    try {
      setLoading(true);
      setError(null);
      setShowConfirmDialog(false);

      const options = {
        technician,
        description: downloadDescription || `File download requested by ${technician}`
      };

      await onDownload(fileToDownload.filename, options);
      
      // Clear form
      setCustomPath('');
      setSelectedFile('');
      setDownloadDescription('');
      setFileToDownload(null);
      
    } catch (error) {
      console.error('Download request failed:', error);
      setError(error.message || 'Download request failed');
    } finally {
      setLoading(false);
    }
  }, [fileToDownload, onDownload, technician, downloadDescription]);

  /**
   * Clear error
   */
  const clearError = useCallback(() => {
    setError(null);
  }, []);

  /**
   * Get file icon based on extension
   */
  const getFileIcon = useCallback((filename) => {
    if (!filename) return <FileIcon />;
    
    const extension = filename.split('.').pop()?.toLowerCase();
    
    // You can expand this with more specific icons
    const iconMap = {
      'folder': <FolderIcon />,
      'txt': <FileIcon />,
      'pdf': <FileIcon />,
      'doc': <FileIcon />,
      'docx': <FileIcon />,
      'xls': <FileIcon />,
      'xlsx': <FileIcon />,
      'jpg': <FileIcon />,
      'jpeg': <FileIcon />,
      'png': <FileIcon />,
      'gif': <FileIcon />,
    };
    
    return iconMap[extension] || <FileIcon />;
  }, []);

  /**
   * Format file path for display
   */
  const formatPath = useCallback((path) => {
    if (path.length <= 50) return path;
    return '...' + path.slice(-47);
  }, []);

  // Load files on component mount
  useEffect(() => {
    loadAvailableFiles();
  }, [loadAvailableFiles]);

  return (
    <Box>
      {/* Header with refresh button */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Typography variant="h6">
          Download Files
        </Typography>
        <Button
          startIcon={<RefreshIcon />}
          onClick={loadAvailableFiles}
          disabled={refreshing || disabled}
          size="small"
        >
          {refreshing ? 'Refreshing...' : 'Refresh'}
        </Button>
      </Box>

      {/* Error Alert */}
      {error && (
        <Alert 
          severity="error" 
          onClose={clearError}
          sx={{ mb: 2 }}
        >
          {error}
        </Alert>
      )}

      {/* Search and File Selection */}
      <Box sx={{ mb: 3 }}>
        {/* Search Field */}
        <TextField
          fullWidth
          label="Search files"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          InputProps={{
            startAdornment: <SearchIcon sx={{ mr: 1, color: 'text.secondary' }} />
          }}
          sx={{ mb: 2 }}
          disabled={disabled}
        />

        {/* Custom Path Input */}
        <Autocomplete
          freeSolo
          options={filteredFiles.map(file => file.path)}
          value={customPath}
          onInputChange={(event, newValue) => {
            setCustomPath(newValue || '');
          }}
          onChange={(event, newValue) => {
            setCustomPath(newValue || '');
          }}
          renderInput={(params) => (
            <TextField
              {...params}
              label="File path"
              placeholder="Enter file path or select from list below"
              helperText="Enter the full path to the file you want to download"
              disabled={disabled}
            />
          )}
          disabled={disabled}
        />
      </Box>

      {/* Available Files List */}
      {filteredFiles.length > 0 ? (
        <Paper variant="outlined" sx={{ mb: 3, maxHeight: 400, overflow: 'auto' }}>
          <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="subtitle2" color="text.secondary">
              Available Files ({filteredFiles.length})
            </Typography>
          </Box>
          <List dense>
            {filteredFiles.map((file, index) => (
              <React.Fragment key={file.path}>
                <ListItem
                  button
                  onClick={() => handleFileSelect(file)}
                  selected={selectedFile === file.path}
                  disabled={disabled}
                >
                  <ListItemIcon>
                    {getFileIcon(file.name)}
                  </ListItemIcon>
                  <ListItemText
                    primary={
                      <Typography variant="body2">
                        {file.name}
                      </Typography>
                    }
                    secondary={
                      <Box>
                        <Typography variant="caption" color="text.secondary">
                          {formatPath(file.path)}
                        </Typography>
                        {file.size && (
                          <Typography variant="caption" color="text.secondary" sx={{ ml: 2 }}>
                            {FileTransferService.formatFileSize(file.size)}
                          </Typography>
                        )}
                        {file.modified && (
                          <Typography variant="caption" color="text.secondary" sx={{ ml: 2 }}>
                            Modified: {new Date(file.modified).toLocaleDateString()}
                          </Typography>
                        )}
                      </Box>
                    }
                  />
                  <ListItemSecondaryAction>
                    <Tooltip title="Select this file">
                      <IconButton
                        edge="end"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleFileSelect(file);
                        }}
                        size="small"
                        disabled={disabled}
                      >
                        <GetAppIcon />
                      </IconButton>
                    </Tooltip>
                  </ListItemSecondaryAction>
                </ListItem>
                {index < filteredFiles.length - 1 && <Divider />}
              </React.Fragment>
            ))}
          </List>
        </Paper>
      ) : (
        <Paper variant="outlined" sx={{ p: 4, textAlign: 'center', mb: 3 }}>
          {refreshing ? (
            <Box>
              <CircularProgress size={24} sx={{ mb: 2 }} />
              <Typography color="text.secondary">
                Loading available files...
              </Typography>
            </Box>
          ) : (
            <Box>
              <InfoIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
              <Typography color="text.secondary">
                {searchTerm ? 'No files match your search' : 'No files available'}
              </Typography>
              {!searchTerm && (
                <Typography variant="caption" color="text.secondary">
                  Files will appear here when the client shares them
                </Typography>
              )}
            </Box>
          )}
        </Paper>
      )}

      {/* Download Button */}
      <Box sx={{ display: 'flex', justifyContent: 'center' }}>
        <Button
          variant="contained"
          size="large"
          onClick={startDownload}
          disabled={disabled || loading || !customPath.trim()}
          startIcon={loading ? <CircularProgress size={20} /> : <DownloadIcon />}
        >
          {loading ? 'Requesting...' : 'Request Download'}
        </Button>
      </Box>

      {/* Information */}
      <Alert severity="info" sx={{ mt: 3 }}>
        <Typography variant="body2">
          <strong>Note:</strong> Download requests require approval from the client. 
          Once approved, the file transfer will begin automatically.
        </Typography>
      </Alert>

      {/* Confirmation Dialog */}
      <Dialog
        open={showConfirmDialog}
        onClose={() => setShowConfirmDialog(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          Confirm Download Request
        </DialogTitle>
        <DialogContent>
          {fileToDownload && (
            <Box>
              <Typography gutterBottom>
                You are about to request download of:
              </Typography>
              
              <Paper variant="outlined" sx={{ p: 2, mb: 2 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  {getFileIcon(fileToDownload.filename)}
                  <Typography variant="subtitle1" sx={{ ml: 1 }}>
                    {fileToDownload.filename}
                  </Typography>
                </Box>
                <Typography variant="body2" color="text.secondary">
                  Path: {fileToDownload.path}
                </Typography>
                {fileToDownload.size && (
                  <Typography variant="body2" color="text.secondary">
                    Size: {FileTransferService.formatFileSize(fileToDownload.size)}
                  </Typography>
                )}
              </Paper>

              <TextField
                fullWidth
                label="Request description (optional)"
                value={downloadDescription}
                onChange={(e) => setDownloadDescription(e.target.value)}
                multiline
                rows={3}
                placeholder="Add a description for this download request..."
              />

              <Alert severity="warning" sx={{ mt: 2 }}>
                This request will be sent to the client for approval. 
                The client can approve or reject this download request.
              </Alert>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowConfirmDialog(false)}>
            Cancel
          </Button>
          <Button 
            onClick={performDownload} 
            variant="contained"
            disabled={loading}
          >
            Send Request
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default FileDownload;