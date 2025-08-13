import React, { useState, useMemo } from 'react';
import {
  Box,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  Paper,
  Chip,
  IconButton,
  Button,
  LinearProgress,
  Tooltip,
  Menu,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Collapse,
  Card,
  CardContent,
  Divider,
  Stack
} from '@mui/material';
import {
  MoreVert as MoreIcon,
  PlayArrow as PlayIcon,
  Pause as PauseIcon,
  Stop as StopIcon,
  Delete as DeleteIcon,
  Download as DownloadIcon,
  Upload as UploadIcon,
  CheckCircle as ApproveIcon,
  Cancel as RejectIcon,
  Refresh as RefreshIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
  Info as InfoIcon,
  Warning as WarningIcon,
  Error as ErrorIcon
} from '@mui/icons-material';
import { styled } from '@mui/material/styles';
import FileTransferService from '../../services/FileTransferService';

const StyledTableRow = styled(TableRow)(({ theme }) => ({
  '&:nth-of-type(odd)': {
    backgroundColor: theme.palette.action.hover,
  },
  '&:hover': {
    backgroundColor: theme.palette.action.selected,
  },
}));

const TransferList = ({ 
  transfers = [], 
  onApprove,
  onReject, 
  onPause, 
  onResume, 
  onCancel, 
  onDelete,
  onDownload,
  onRefresh,
  loading = false,
  role = 'technician'
}) => {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [selectedTransfer, setSelectedTransfer] = useState(null);
  const [actionMenuAnchor, setActionMenuAnchor] = useState(null);
  const [showApprovalDialog, setShowApprovalDialog] = useState(false);
  const [approvalAction, setApprovalAction] = useState(null);
  const [approvalMessage, setApprovalMessage] = useState('');
  const [expandedRows, setExpandedRows] = useState(new Set());
  const [sortBy, setSortBy] = useState('created_at');
  const [sortOrder, setSortOrder] = useState('desc');

  /**
   * Sort and paginate transfers
   */
  const sortedTransfers = useMemo(() => {
    const sorted = [...transfers].sort((a, b) => {
      let aValue = a[sortBy];
      let bValue = b[sortBy];
      
      // Handle date sorting
      if (sortBy.includes('_at')) {
        aValue = new Date(aValue || 0).getTime();
        bValue = new Date(bValue || 0).getTime();
      }
      
      // Handle string sorting
      if (typeof aValue === 'string') {
        aValue = aValue.toLowerCase();
        bValue = bValue.toLowerCase();
      }
      
      if (sortOrder === 'asc') {
        return aValue > bValue ? 1 : -1;
      } else {
        return aValue < bValue ? 1 : -1;
      }
    });
    
    return sorted;
  }, [transfers, sortBy, sortOrder]);

  const paginatedTransfers = useMemo(() => {
    const startIndex = page * rowsPerPage;
    return sortedTransfers.slice(startIndex, startIndex + rowsPerPage);
  }, [sortedTransfers, page, rowsPerPage]);

  /**
   * Handle page change
   */
  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  /**
   * Handle rows per page change
   */
  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  /**
   * Handle action menu
   */
  const handleActionMenuOpen = (event, transfer) => {
    setActionMenuAnchor(event.currentTarget);
    setSelectedTransfer(transfer);
  };

  const handleActionMenuClose = () => {
    setActionMenuAnchor(null);
    setSelectedTransfer(null);
  };

  /**
   * Handle approval dialog
   */
  const handleApprovalDialog = (action) => {
    setApprovalAction(action);
    setShowApprovalDialog(true);
    handleActionMenuClose();
  };

  const handleApprovalSubmit = async () => {
    if (!selectedTransfer || !approvalAction) return;

    try {
      if (approvalAction === 'approve') {
        await onApprove(selectedTransfer.id, approvalMessage);
      } else {
        await onReject(selectedTransfer.id, approvalMessage);
      }
      
      setShowApprovalDialog(false);
      setApprovalMessage('');
      setApprovalAction(null);
      setSelectedTransfer(null);
    } catch (error) {
      console.error('Approval action failed:', error);
    }
  };

  /**
   * Handle row expansion
   */
  const toggleRowExpansion = (transferId) => {
    const newExpanded = new Set(expandedRows);
    if (newExpanded.has(transferId)) {
      newExpanded.delete(transferId);
    } else {
      newExpanded.add(transferId);
    }
    setExpandedRows(newExpanded);
  };

  /**
   * Get status color
   */
  const getStatusColor = (status) => {
    return FileTransferService.getStatusColor(status);
  };

  /**
   * Get status icon
   */
  const getStatusIcon = (status) => {
    switch (status) {
      case 'completed':
        return <ApproveIcon fontSize="small" />;
      case 'failed':
      case 'rejected':
        return <ErrorIcon fontSize="small" />;
      case 'in_progress':
        return <PlayIcon fontSize="small" />;
      case 'paused':
        return <PauseIcon fontSize="small" />;
      case 'pending':
        return <InfoIcon fontSize="small" />;
      default:
        return <WarningIcon fontSize="small" />;
    }
  };

  /**
   * Get transfer type icon
   */
  const getTypeIcon = (type) => {
    return type === 'upload' ? <UploadIcon fontSize="small" /> : <DownloadIcon fontSize="small" />;
  };

  /**
   * Format date
   */
  const formatDate = (dateString) => {
    if (!dateString) return 'N/A';
    return new Date(dateString).toLocaleString();
  };

  /**
   * Format duration
   */
  const formatDuration = (startTime, endTime) => {
    if (!startTime) return 'N/A';
    
    const start = new Date(startTime);
    const end = endTime ? new Date(endTime) : new Date();
    const duration = Math.floor((end - start) / 1000);
    
    return FileTransferService.formatDuration(duration);
  };

  /**
   * Check if action is available
   */
  const isActionAvailable = (transfer, action) => {
    const status = transfer.status;
    const isActive = FileTransferService.isActiveStatus(status);
    const isFinal = FileTransferService.isFinalStatus(status);
    
    switch (action) {
      case 'approve':
      case 'reject':
        return status === 'pending' && role === 'technician';
      case 'pause':
        return status === 'in_progress';
      case 'resume':
        return status === 'paused';
      case 'cancel':
        return isActive && !isFinal;
      case 'delete':
        return isFinal;
      case 'download':
        return status === 'completed' && transfer.type === 'download';
      default:
        return false;
    }
  };

  /**
   * Render transfer details
   */
  const renderTransferDetails = (transfer) => {
    return (
      <Card variant="outlined" sx={{ mt: 1, mb: 1 }}>
        <CardContent>
          <Typography variant="subtitle2" gutterBottom>
            Transfer Details
          </Typography>
          
          <Stack spacing={1}>
            {transfer.description && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Description:
                </Typography>
                <Typography variant="body2">
                  {transfer.description}
                </Typography>
              </Box>
            )}
            
            {transfer.technician && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Technician:
                </Typography>
                <Typography variant="body2">
                  {transfer.technician}
                </Typography>
              </Box>
            )}
            
            {transfer.checksum && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Checksum:
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                  {transfer.checksum}
                </Typography>
              </Box>
            )}
            
            {transfer.error && (
              <Alert severity="error" size="small">
                <Typography variant="body2">
                  {transfer.error}
                </Typography>
              </Alert>
            )}
            
            <Divider />
            
            <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Created:
                </Typography>
                <Typography variant="body2">
                  {formatDate(transfer.created_at)}
                </Typography>
              </Box>
              
              {transfer.started_at && (
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    Started:
                  </Typography>
                  <Typography variant="body2">
                    {formatDate(transfer.started_at)}
                  </Typography>
                </Box>
              )}
              
              {transfer.completed_at && (
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    Completed:
                  </Typography>
                  <Typography variant="body2">
                    {formatDate(transfer.completed_at)}
                  </Typography>
                </Box>
              )}
              
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Duration:
                </Typography>
                <Typography variant="body2">
                  {formatDuration(transfer.started_at, transfer.completed_at)}
                </Typography>
              </Box>
            </Box>
          </Stack>
        </CardContent>
      </Card>
    );
  };

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h6">
          File Transfers ({transfers.length})
        </Typography>
        <Button
          startIcon={<RefreshIcon />}
          onClick={onRefresh}
          disabled={loading}
          size="small"
        >
          Refresh
        </Button>
      </Box>

      {/* Transfers Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell width={40}></TableCell>
              <TableCell>File</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Progress</TableCell>
              <TableCell>Size</TableCell>
              <TableCell>Speed</TableCell>
              <TableCell>ETA</TableCell>
              <TableCell>Created</TableCell>
              <TableCell width={60}>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {paginatedTransfers.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} align="center" sx={{ py: 4 }}>
                  <Typography color="text.secondary">
                    {loading ? 'Loading transfers...' : 'No transfers found'}
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              paginatedTransfers.map((transfer) => (
                <React.Fragment key={transfer.id}>
                  <StyledTableRow>
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={() => toggleRowExpansion(transfer.id)}
                      >
                        {expandedRows.has(transfer.id) ? <ExpandLessIcon /> : <ExpandMoreIcon />}
                      </IconButton>
                    </TableCell>
                    
                    <TableCell>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        {getTypeIcon(transfer.type)}
                        <Box>
                          <Typography variant="body2" noWrap>
                            {transfer.filename}
                          </Typography>
                          {transfer.path && (
                            <Typography variant="caption" color="text.secondary" noWrap>
                              {transfer.path}
                            </Typography>
                          )}
                        </Box>
                      </Box>
                    </TableCell>
                    
                    <TableCell>
                      <Chip
                        label={transfer.type}
                        size="small"
                        variant="outlined"
                        color={transfer.type === 'upload' ? 'primary' : 'secondary'}
                      />
                    </TableCell>
                    
                    <TableCell>
                      <Chip
                        icon={getStatusIcon(transfer.status)}
                        label={transfer.status}
                        size="small"
                        color={getStatusColor(transfer.status)}
                        variant="outlined"
                      />
                    </TableCell>
                    
                    <TableCell>
                      <Box sx={{ minWidth: 120 }}>
                        {transfer.status === 'in_progress' && transfer.progress !== undefined ? (
                          <Box>
                            <LinearProgress 
                              variant="determinate" 
                              value={transfer.progress} 
                              sx={{ mb: 0.5 }}
                            />
                            <Typography variant="caption">
                              {transfer.progress.toFixed(1)}%
                            </Typography>
                          </Box>
                        ) : (
                          <Typography variant="body2" color="text.secondary">
                            {transfer.progress !== undefined ? `${transfer.progress.toFixed(1)}%` : 'N/A'}
                          </Typography>
                        )}
                      </Box>
                    </TableCell>
                    
                    <TableCell>
                      <Typography variant="body2">
                        {transfer.size ? FileTransferService.formatFileSize(transfer.size) : 'N/A'}
                      </Typography>
                    </TableCell>
                    
                    <TableCell>
                      <Typography variant="body2">
                        {transfer.speed ? FileTransferService.formatSpeed(transfer.speed) : 'N/A'}
                      </Typography>
                    </TableCell>
                    
                    <TableCell>
                      <Typography variant="body2">
                        {transfer.eta ? FileTransferService.formatDuration(transfer.eta) : 'N/A'}
                      </Typography>
                    </TableCell>
                    
                    <TableCell>
                      <Typography variant="body2">
                        {formatDate(transfer.created_at)}
                      </Typography>
                    </TableCell>
                    
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={(e) => handleActionMenuOpen(e, transfer)}
                      >
                        <MoreIcon />
                      </IconButton>
                    </TableCell>
                  </StyledTableRow>
                  
                  {/* Expanded Row */}
                  <TableRow>
                    <TableCell colSpan={10} sx={{ py: 0 }}>
                      <Collapse in={expandedRows.has(transfer.id)} timeout="auto" unmountOnExit>
                        <Box sx={{ py: 2 }}>
                          {renderTransferDetails(transfer)}
                        </Box>
                      </Collapse>
                    </TableCell>
                  </TableRow>
                </React.Fragment>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Pagination */}
      {transfers.length > 0 && (
        <TablePagination
          component="div"
          count={transfers.length}
          page={page}
          onPageChange={handleChangePage}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={[5, 10, 25, 50]}
        />
      )}

      {/* Action Menu */}
      <Menu
        anchorEl={actionMenuAnchor}
        open={Boolean(actionMenuAnchor)}
        onClose={handleActionMenuClose}
      >
        {selectedTransfer && [
          isActionAvailable(selectedTransfer, 'approve') && (
            <MenuItem key="approve" onClick={() => handleApprovalDialog('approve')}>
              <ApproveIcon sx={{ mr: 1 }} />
              Approve
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'reject') && (
            <MenuItem key="reject" onClick={() => handleApprovalDialog('reject')}>
              <RejectIcon sx={{ mr: 1 }} />
              Reject
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'pause') && (
            <MenuItem key="pause" onClick={() => { onPause(selectedTransfer.id); handleActionMenuClose(); }}>
              <PauseIcon sx={{ mr: 1 }} />
              Pause
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'resume') && (
            <MenuItem key="resume" onClick={() => { onResume(selectedTransfer.id); handleActionMenuClose(); }}>
              <PlayIcon sx={{ mr: 1 }} />
              Resume
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'cancel') && (
            <MenuItem key="cancel" onClick={() => { onCancel(selectedTransfer.id); handleActionMenuClose(); }}>
              <StopIcon sx={{ mr: 1 }} />
              Cancel
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'download') && (
            <MenuItem key="download" onClick={() => { onDownload(selectedTransfer.id, selectedTransfer.filename); handleActionMenuClose(); }}>
              <DownloadIcon sx={{ mr: 1 }} />
              Download
            </MenuItem>
          ),
          isActionAvailable(selectedTransfer, 'delete') && (
            <MenuItem key="delete" onClick={() => { onDelete(selectedTransfer.id); handleActionMenuClose(); }}>
              <DeleteIcon sx={{ mr: 1 }} />
              Delete
            </MenuItem>
          ),
        ].filter(Boolean)}
      </Menu>

      {/* Approval Dialog */}
      <Dialog
        open={showApprovalDialog}
        onClose={() => setShowApprovalDialog(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          {approvalAction === 'approve' ? 'Approve Transfer' : 'Reject Transfer'}
        </DialogTitle>
        <DialogContent>
          {selectedTransfer && (
            <Box>
              <Typography gutterBottom>
                {approvalAction === 'approve' ? 'Approve' : 'Reject'} transfer for:
              </Typography>
              
              <Paper variant="outlined" sx={{ p: 2, mb: 2 }}>
                <Typography variant="subtitle2">
                  {selectedTransfer.filename}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {selectedTransfer.type === 'upload' ? 'Upload' : 'Download'} • 
                  {selectedTransfer.size ? FileTransferService.formatFileSize(selectedTransfer.size) : 'Unknown size'}
                </Typography>
              </Paper>

              <TextField
                fullWidth
                label={`${approvalAction === 'approve' ? 'Approval' : 'Rejection'} message (optional)`}
                value={approvalMessage}
                onChange={(e) => setApprovalMessage(e.target.value)}
                multiline
                rows={3}
                placeholder={`Add a message for this ${approvalAction}...`}
              />
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowApprovalDialog(false)}>
            Cancel
          </Button>
          <Button 
            onClick={handleApprovalSubmit} 
            variant="contained"
            color={approvalAction === 'approve' ? 'primary' : 'error'}
          >
            {approvalAction === 'approve' ? 'Approve' : 'Reject'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default TransferList;