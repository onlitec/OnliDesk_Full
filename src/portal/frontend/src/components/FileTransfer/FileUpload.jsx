import React, { useState, useRef, useCallback } from 'react';
import {
  Box,
  Button,
  Typography,
  LinearProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Chip,
  IconButton,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Paper,
  Divider
} from '@mui/material';
import {
  CloudUpload as UploadIcon,
  Delete as DeleteIcon,
  InsertDriveFile as FileIcon,
  Warning as WarningIcon,
  CheckCircle as CheckIcon
} from '@mui/icons-material';
import { styled } from '@mui/material/styles';
import FileTransferService from '../../services/FileTransferService';

const DropZone = styled(Paper)(({ theme, isDragOver, hasError }) => ({
  border: `2px dashed ${
    hasError 
      ? theme.palette.error.main 
      : isDragOver 
        ? theme.palette.primary.main 
        : theme.palette.grey[300]
  }`,
  borderRadius: theme.spacing(1),
  padding: theme.spacing(4),
  textAlign: 'center',
  cursor: 'pointer',
  transition: 'all 0.3s ease',
  backgroundColor: isDragOver 
    ? theme.palette.action.hover 
    : hasError 
      ? theme.palette.error.light + '20'
      : 'transparent',
  '&:hover': {
    borderColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.hover,
  },
}));

const HiddenInput = styled('input')({
  display: 'none',
});

const FileUpload = ({ 
  onUpload, 
  settings = {}, 
  disabled = false,
  multiple = true,
  technician = 'Unknown'
}) => {
  const [selectedFiles, setSelectedFiles] = useState([]);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState({});
  const [errors, setErrors] = useState([]);
  const [isDragOver, setIsDragOver] = useState(false);
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [uploadDescription, setUploadDescription] = useState('');
  const fileInputRef = useRef(null);

  // Default settings
  const defaultSettings = {
    maxFileSize: 100 * 1024 * 1024, // 100MB
    allowedExtensions: ['.txt', '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.jpg', '.jpeg', '.png', '.gif'],
    blockedExtensions: ['.exe', '.bat', '.cmd', '.scr', '.com', '.pif'],
    maxFiles: 10,
    requireApproval: true,
    allowCompression: true,
    ...settings
  };

  /**
   * Validate a single file
   */
  const validateFile = useCallback((file) => {
    const errors = [];

    // Check file size
    if (file.size > defaultSettings.maxFileSize) {
      errors.push(`File size exceeds limit (${FileTransferService.formatFileSize(defaultSettings.maxFileSize)})`);
    }

    // Check file extension
    const extension = '.' + file.name.split('.').pop().toLowerCase();
    
    if (defaultSettings.blockedExtensions.includes(extension)) {
      errors.push(`File type '${extension}' is not allowed`);
    }
    
    if (defaultSettings.allowedExtensions.length > 0 && 
        !defaultSettings.allowedExtensions.includes(extension)) {
      errors.push(`File type '${extension}' is not supported`);
    }

    // Check filename
    if (file.name.length > 255) {
      errors.push('Filename is too long (max 255 characters)');
    }

    if (!/^[a-zA-Z0-9._\-\s]+$/.test(file.name)) {
      errors.push('Filename contains invalid characters');
    }

    return {
      valid: errors.length === 0,
      errors
    };
  }, [defaultSettings]);

  /**
   * Handle file selection
   */
  const handleFileSelect = useCallback((files) => {
    const fileArray = Array.from(files);
    const validFiles = [];
    const newErrors = [];

    // Check total file count
    if (selectedFiles.length + fileArray.length > defaultSettings.maxFiles) {
      newErrors.push(`Maximum ${defaultSettings.maxFiles} files allowed`);
      setErrors(newErrors);
      return;
    }

    // Validate each file
    fileArray.forEach((file, index) => {
      const validation = validateFile(file);
      
      if (validation.valid) {
        // Check for duplicates
        const isDuplicate = selectedFiles.some(f => 
          f.name === file.name && f.size === file.size
        );
        
        if (!isDuplicate) {
          validFiles.push({
            file,
            id: `${file.name}_${file.size}_${Date.now()}_${index}`,
            name: file.name,
            size: file.size,
            type: file.type,
            status: 'selected',
            progress: 0,
            errors: []
          });
        } else {
          newErrors.push(`Duplicate file: ${file.name}`);
        }
      } else {
        newErrors.push(`${file.name}: ${validation.errors.join(', ')}`);
      }
    });

    if (validFiles.length > 0) {
      setSelectedFiles(prev => [...prev, ...validFiles]);
    }

    if (newErrors.length > 0) {
      setErrors(prev => [...prev, ...newErrors]);
    }
  }, [selectedFiles, validateFile, defaultSettings.maxFiles]);

  /**
   * Handle drag and drop events
   */
  const handleDragOver = useCallback((e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
  }, []);

  const handleDrop = useCallback((e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFileSelect(files);
    }
  }, [handleFileSelect]);

  /**
   * Handle file input change
   */
  const handleFileInputChange = useCallback((e) => {
    const files = e.target.files;
    if (files.length > 0) {
      handleFileSelect(files);
    }
    // Reset input value to allow selecting the same file again
    e.target.value = '';
  }, [handleFileSelect]);

  /**
   * Remove selected file
   */
  const removeFile = useCallback((fileId) => {
    setSelectedFiles(prev => prev.filter(f => f.id !== fileId));
    setUploadProgress(prev => {
      const updated = { ...prev };
      delete updated[fileId];
      return updated;
    });
  }, []);

  /**
   * Clear all files
   */
  const clearFiles = useCallback(() => {
    setSelectedFiles([]);
    setUploadProgress({});
    setErrors([]);
  }, []);

  /**
   * Clear errors
   */
  const clearErrors = useCallback(() => {
    setErrors([]);
  }, []);

  /**
   * Start upload process
   */
  const startUpload = useCallback(() => {
    if (selectedFiles.length === 0) {
      setErrors(['No files selected']);
      return;
    }

    if (defaultSettings.requireApproval) {
      setShowConfirmDialog(true);
    } else {
      performUpload();
    }
  }, [selectedFiles, defaultSettings.requireApproval]);

  /**
   * Perform the actual upload
   */
  const performUpload = useCallback(async () => {
    setUploading(true);
    setErrors([]);
    setShowConfirmDialog(false);

    const uploadPromises = selectedFiles.map(async (fileData) => {
      try {
        // Update file status
        setSelectedFiles(prev => 
          prev.map(f => 
            f.id === fileData.id 
              ? { ...f, status: 'uploading' }
              : f
          )
        );

        // Prepare upload options
        const options = {
          technician,
          description: uploadDescription || `File uploaded by ${technician}`,
          onProgress: (progress) => {
            setUploadProgress(prev => ({
              ...prev,
              [fileData.id]: progress
            }));
          }
        };

        // Upload file
        const result = await onUpload(fileData.file, options);

        // Update file status
        setSelectedFiles(prev => 
          prev.map(f => 
            f.id === fileData.id 
              ? { ...f, status: 'completed', transferId: result.id }
              : f
          )
        );

        return { success: true, fileId: fileData.id, result };
      } catch (error) {
        console.error(`Upload failed for ${fileData.name}:`, error);
        
        // Update file status
        setSelectedFiles(prev => 
          prev.map(f => 
            f.id === fileData.id 
              ? { ...f, status: 'failed', error: error.message }
              : f
          )
        );

        return { success: false, fileId: fileData.id, error: error.message };
      }
    });

    try {
      const results = await Promise.allSettled(uploadPromises);
      
      const failures = results
        .filter(result => result.status === 'rejected' || !result.value.success)
        .map(result => 
          result.status === 'rejected' 
            ? result.reason.message 
            : result.value.error
        );

      if (failures.length > 0) {
        setErrors(failures);
      }

      // Clear successful uploads after a delay
      setTimeout(() => {
        setSelectedFiles(prev => prev.filter(f => f.status !== 'completed'));
      }, 3000);

    } catch (error) {
      console.error('Upload process failed:', error);
      setErrors([error.message || 'Upload process failed']);
    } finally {
      setUploading(false);
      setUploadProgress({});
    }
  }, [selectedFiles, onUpload, technician, uploadDescription]);

  /**
   * Get file status icon
   */
  const getFileStatusIcon = (status) => {
    switch (status) {
      case 'completed':
        return <CheckIcon color="success" />;
      case 'failed':
        return <WarningIcon color="error" />;
      case 'uploading':
        return null; // Progress bar will be shown
      default:
        return <FileIcon />;
    }
  };

  /**
   * Get file status color
   */
  const getFileStatusColor = (status) => {
    switch (status) {
      case 'completed':
        return 'success';
      case 'failed':
        return 'error';
      case 'uploading':
        return 'primary';
      default:
        return 'default';
    }
  };

  return (
    <Box>
      {/* Drop Zone */}
      <DropZone
        isDragOver={isDragOver}
        hasError={errors.length > 0}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
        elevation={isDragOver ? 4 : 1}
      >
        <UploadIcon 
          sx={{ 
            fontSize: 48, 
            color: errors.length > 0 ? 'error.main' : 'text.secondary',
            mb: 2 
          }} 
        />
        <Typography variant="h6" gutterBottom>
          {isDragOver ? 'Drop files here' : 'Drag & drop files here'}
        </Typography>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          or click to browse files
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Max {defaultSettings.maxFiles} files, {FileTransferService.formatFileSize(defaultSettings.maxFileSize)} each
        </Typography>
        
        {/* Allowed file types */}
        {defaultSettings.allowedExtensions.length > 0 && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="caption" color="text.secondary">
              Allowed types:
            </Typography>
            <Box sx={{ mt: 1 }}>
              {defaultSettings.allowedExtensions.map((ext) => (
                <Chip
                  key={ext}
                  label={ext}
                  size="small"
                  variant="outlined"
                  sx={{ mr: 0.5, mb: 0.5 }}
                />
              ))}
            </Box>
          </Box>
        )}
      </DropZone>

      <HiddenInput
        ref={fileInputRef}
        type="file"
        multiple={multiple}
        onChange={handleFileInputChange}
        disabled={disabled || uploading}
      />

      {/* Error Messages */}
      {errors.length > 0 && (
        <Box sx={{ mt: 2 }}>
          {errors.map((error, index) => (
            <Alert 
              key={index} 
              severity="error" 
              onClose={clearErrors}
              sx={{ mb: 1 }}
            >
              {error}
            </Alert>
          ))}
        </Box>
      )}

      {/* Selected Files List */}
      {selectedFiles.length > 0 && (
        <Box sx={{ mt: 3 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
            <Typography variant="h6">
              Selected Files ({selectedFiles.length})
            </Typography>
            <Button
              size="small"
              onClick={clearFiles}
              disabled={uploading}
            >
              Clear All
            </Button>
          </Box>
          
          <Paper variant="outlined">
            <List>
              {selectedFiles.map((fileData, index) => (
                <React.Fragment key={fileData.id}>
                  <ListItem>
                    <Box sx={{ display: 'flex', alignItems: 'center', mr: 2 }}>
                      {getFileStatusIcon(fileData.status)}
                    </Box>
                    <ListItemText
                      primary={
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Typography variant="body2">
                            {fileData.name}
                          </Typography>
                          <Chip
                            label={fileData.status}
                            size="small"
                            color={getFileStatusColor(fileData.status)}
                            variant="outlined"
                          />
                        </Box>
                      }
                      secondary={
                        <Box>
                          <Typography variant="caption" color="text.secondary">
                            {FileTransferService.formatFileSize(fileData.size)}
                          </Typography>
                          {fileData.error && (
                            <Typography variant="caption" color="error" sx={{ display: 'block' }}>
                              Error: {fileData.error}
                            </Typography>
                          )}
                          {fileData.status === 'uploading' && uploadProgress[fileData.id] && (
                            <Box sx={{ mt: 1 }}>
                              <LinearProgress 
                                variant="determinate" 
                                value={uploadProgress[fileData.id]} 
                              />
                              <Typography variant="caption" color="text.secondary">
                                {uploadProgress[fileData.id].toFixed(1)}%
                              </Typography>
                            </Box>
                          )}
                        </Box>
                      }
                    />
                    <ListItemSecondaryAction>
                      <IconButton
                        edge="end"
                        onClick={() => removeFile(fileData.id)}
                        disabled={uploading && fileData.status === 'uploading'}
                        size="small"
                      >
                        <DeleteIcon />
                      </IconButton>
                    </ListItemSecondaryAction>
                  </ListItem>
                  {index < selectedFiles.length - 1 && <Divider />}
                </React.Fragment>
              ))}
            </List>
          </Paper>
        </Box>
      )}

      {/* Upload Button */}
      {selectedFiles.length > 0 && (
        <Box sx={{ mt: 3, display: 'flex', justifyContent: 'center' }}>
          <Button
            variant="contained"
            size="large"
            onClick={startUpload}
            disabled={disabled || uploading || selectedFiles.every(f => f.status === 'completed')}
            startIcon={<UploadIcon />}
          >
            {uploading ? 'Uploading...' : `Upload ${selectedFiles.length} File${selectedFiles.length > 1 ? 's' : ''}`}
          </Button>
        </Box>
      )}

      {/* Confirmation Dialog */}
      <Dialog
        open={showConfirmDialog}
        onClose={() => setShowConfirmDialog(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          Confirm File Upload
        </DialogTitle>
        <DialogContent>
          <Typography gutterBottom>
            You are about to upload {selectedFiles.length} file{selectedFiles.length > 1 ? 's' : ''}:
          </Typography>
          
          <List dense>
            {selectedFiles.map((fileData) => (
              <ListItem key={fileData.id}>
                <ListItemText
                  primary={fileData.name}
                  secondary={FileTransferService.formatFileSize(fileData.size)}
                />
              </ListItem>
            ))}
          </List>

          <TextField
            fullWidth
            label="Description (optional)"
            value={uploadDescription}
            onChange={(e) => setUploadDescription(e.target.value)}
            multiline
            rows={3}
            sx={{ mt: 2 }}
            placeholder="Add a description for this upload..."
          />

          {defaultSettings.requireApproval && (
            <Alert severity="info" sx={{ mt: 2 }}>
              These files will require approval before transfer begins.
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowConfirmDialog(false)}>
            Cancel
          </Button>
          <Button 
            onClick={performUpload} 
            variant="contained"
            disabled={uploading}
          >
            Upload
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default FileUpload;