import React, { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Alert,
  Chip,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  IconButton,
  Divider,
  Grid,
  Paper,
  Tooltip,
  Badge,
} from '@mui/material';
import {
  Security as SecurityIcon,
  AdminPanelSettings as AdminIcon,
  Warning as WarningIcon,
  Check as CheckIcon,
  Close as CloseIcon,
  Info as InfoIcon,
  Schedule as ScheduleIcon,
  Computer as ComputerIcon,
} from '@mui/icons-material';
import { useRemoteAccess } from '../../hooks/useRemoteAccess';

const PrivilegeEscalation = ({ sessionId, onPrivilegeChange }) => {
  const [requestDialogOpen, setRequestDialogOpen] = useState(false);
  const [requestType, setRequestType] = useState('');
  const [requestReason, setRequestReason] = useState('');
  const [requestDuration, setRequestDuration] = useState(30); // minutes
  const [pendingRequests, setPendingRequests] = useState([]);
  const [approvedRequests, setApprovedRequests] = useState([]);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);

  const {
    requestPrivilegeEscalation,
    sendMessage,
    privilegeRequests,
    isConnected
  } = useRemoteAccess();

  // Privilege types
  const privilegeTypes = [
    {
      value: 'admin',
      label: 'Administrador',
      description: 'Acesso completo ao sistema',
      icon: <AdminIcon />,
      color: 'error'
    },
    {
      value: 'elevated',
      label: 'Privilégios Elevados',
      description: 'Acesso a recursos do sistema',
      icon: <SecurityIcon />,
      color: 'warning'
    },
    {
      value: 'registry',
      label: 'Acesso ao Registro',
      description: 'Modificar configurações do registro',
      icon: <ComputerIcon />,
      color: 'info'
    },
    {
      value: 'services',
      label: 'Gerenciar Serviços',
      description: 'Iniciar/parar serviços do sistema',
      icon: <SecurityIcon />,
      color: 'warning'
    }
  ];

  /**
   * Get privilege type info
   */
  const getPrivilegeTypeInfo = (type) => {
    return privilegeTypes.find(p => p.value === type) || {
      value: type,
      label: type,
      description: 'Tipo de privilégio personalizado',
      icon: <InfoIcon />,
      color: 'default'
    };
  };

  /**
   * Handle privilege request
   */
  const handlePrivilegeRequest = async () => {
    if (!requestType || !requestReason.trim()) {
      setError('Tipo de privilégio e justificativa são obrigatórios');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const request = {
        type: requestType,
        reason: requestReason.trim(),
        duration: requestDuration,
        timestamp: Date.now(),
        sessionId
      };

      await requestPrivilegeEscalation(request);
      
      // Add to pending requests
      setPendingRequests(prev => [{
        ...request,
        id: Math.random().toString(36).substr(2, 9),
        status: 'pending'
      }, ...prev]);

      // Reset form
      setRequestType('');
      setRequestReason('');
      setRequestDuration(30);
      setRequestDialogOpen(false);

      onPrivilegeChange?.('requested', request);
    } catch (err) {
      console.error('Failed to request privilege escalation:', err);
      setError(err.message || 'Falha ao solicitar escalonamento de privilégios');
    } finally {
      setLoading(false);
    }
  };

  /**
   * Handle privilege response from client
   */
  const handlePrivilegeResponse = (requestId, approved, reason = '') => {
    const request = pendingRequests.find(r => r.id === requestId);
    if (!request) return;

    // Remove from pending
    setPendingRequests(prev => prev.filter(r => r.id !== requestId));

    if (approved) {
      // Add to approved
      setApprovedRequests(prev => [{
        ...request,
        status: 'approved',
        approvedAt: Date.now(),
        approvalReason: reason
      }, ...prev]);

      onPrivilegeChange?.('approved', request);
    } else {
      onPrivilegeChange?.('denied', request);
    }

    // Send response to client
    sendMessage({
      type: 'privilege_response',
      requestId,
      approved,
      reason,
      timestamp: Date.now()
    });
  };

  /**
   * Revoke privilege
   */
  const revokePrivilege = (requestId) => {
    setApprovedRequests(prev => prev.filter(r => r.id !== requestId));
    
    sendMessage({
      type: 'privilege_revoke',
      requestId,
      timestamp: Date.now()
    });

    onPrivilegeChange?.('revoked', { id: requestId });
  };

  /**
   * Format duration
   */
  const formatDuration = (minutes) => {
    if (minutes < 60) {
      return `${minutes} min`;
    }
    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}min` : `${hours}h`;
  };

  /**
   * Check if privilege is expired
   */
  const isPrivilegeExpired = (request) => {
    if (!request.approvedAt || !request.duration) return false;
    const expirationTime = request.approvedAt + (request.duration * 60 * 1000);
    return Date.now() > expirationTime;
  };

  /**
   * Get remaining time for privilege
   */
  const getRemainingTime = (request) => {
    if (!request.approvedAt || !request.duration) return null;
    const expirationTime = request.approvedAt + (request.duration * 60 * 1000);
    const remaining = expirationTime - Date.now();
    if (remaining <= 0) return 'Expirado';
    
    const minutes = Math.floor(remaining / (1000 * 60));
    const seconds = Math.floor((remaining % (1000 * 60)) / 1000);
    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  };

  // Update privilege requests from hook
  useEffect(() => {
    if (privilegeRequests.length > 0) {
      const newRequests = privilegeRequests.filter(
        req => !pendingRequests.some(p => p.id === req.id)
      );
      if (newRequests.length > 0) {
        setPendingRequests(prev => [...newRequests, ...prev]);
      }
    }
  }, [privilegeRequests, pendingRequests]);

  // Auto-remove expired privileges
  useEffect(() => {
    const interval = setInterval(() => {
      setApprovedRequests(prev => {
        const active = prev.filter(req => !isPrivilegeExpired(req));
        const expired = prev.filter(req => isPrivilegeExpired(req));
        
        // Notify about expired privileges
        expired.forEach(req => {
          onPrivilegeChange?.('expired', req);
          sendMessage({
            type: 'privilege_expired',
            requestId: req.id,
            timestamp: Date.now()
          });
        });
        
        return active;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [onPrivilegeChange, sendMessage]);

  return (
    <Box>
      {/* Header */}
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h6" component="h3">
          🔐 Escalonamento de Privilégios
        </Typography>
        <Button
          variant="contained"
          startIcon={<SecurityIcon />}
          onClick={() => setRequestDialogOpen(true)}
          disabled={!isConnected}
        >
          Solicitar Privilégio
        </Button>
      </Box>

      {/* Error Alert */}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Connection Warning */}
      {!isConnected && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Conecte-se a uma sessão para gerenciar privilégios
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* Pending Requests */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center" mb={2}>
                <Badge badgeContent={pendingRequests.length} color="warning">
                  <ScheduleIcon color="warning" />
                </Badge>
                <Typography variant="h6" sx={{ ml: 1 }}>
                  Solicitações Pendentes
                </Typography>
              </Box>
              
              {pendingRequests.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  Nenhuma solicitação pendente
                </Typography>
              ) : (
                <List>
                  {pendingRequests.map((request, index) => {
                    const typeInfo = getPrivilegeTypeInfo(request.type);
                    return (
                      <React.Fragment key={request.id}>
                        <ListItem>
                          <Box display="flex" alignItems="center" mr={2}>
                            {typeInfo.icon}
                          </Box>
                          <ListItemText
                            primary={
                              <Box display="flex" alignItems="center" gap={1}>
                                <Typography variant="body1">
                                  {typeInfo.label}
                                </Typography>
                                <Chip
                                  label={formatDuration(request.duration)}
                                  size="small"
                                  color={typeInfo.color}
                                />
                              </Box>
                            }
                            secondary={
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  {request.reason}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                  {new Date(request.timestamp).toLocaleString()}
                                </Typography>
                              </Box>
                            }
                          />
                          <ListItemSecondaryAction>
                            <Tooltip title="Aprovar">
                              <IconButton
                                color="success"
                                onClick={() => handlePrivilegeResponse(request.id, true)}
                              >
                                <CheckIcon />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="Negar">
                              <IconButton
                                color="error"
                                onClick={() => handlePrivilegeResponse(request.id, false)}
                              >
                                <CloseIcon />
                              </IconButton>
                            </Tooltip>
                          </ListItemSecondaryAction>
                        </ListItem>
                        {index < pendingRequests.length - 1 && <Divider />}
                      </React.Fragment>
                    );
                  })}
                </List>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Active Privileges */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center" mb={2}>
                <Badge badgeContent={approvedRequests.length} color="success">
                  <CheckIcon color="success" />
                </Badge>
                <Typography variant="h6" sx={{ ml: 1 }}>
                  Privilégios Ativos
                </Typography>
              </Box>
              
              {approvedRequests.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  Nenhum privilégio ativo
                </Typography>
              ) : (
                <List>
                  {approvedRequests.map((request, index) => {
                    const typeInfo = getPrivilegeTypeInfo(request.type);
                    const remainingTime = getRemainingTime(request);
                    const isExpired = isPrivilegeExpired(request);
                    
                    return (
                      <React.Fragment key={request.id}>
                        <ListItem>
                          <Box display="flex" alignItems="center" mr={2}>
                            {typeInfo.icon}
                          </Box>
                          <ListItemText
                            primary={
                              <Box display="flex" alignItems="center" gap={1}>
                                <Typography variant="body1">
                                  {typeInfo.label}
                                </Typography>
                                <Chip
                                  label={remainingTime}
                                  size="small"
                                  color={isExpired ? 'error' : 'success'}
                                />
                              </Box>
                            }
                            secondary={
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  {request.reason}
                                </Typography>
                                <Typography variant="caption" color="text.secondary">
                                  Aprovado em {new Date(request.approvedAt).toLocaleString()}
                                </Typography>
                              </Box>
                            }
                          />
                          <ListItemSecondaryAction>
                            <Tooltip title="Revogar">
                              <IconButton
                                color="error"
                                onClick={() => revokePrivilege(request.id)}
                              >
                                <CloseIcon />
                              </IconButton>
                            </Tooltip>
                          </ListItemSecondaryAction>
                        </ListItem>
                        {index < approvedRequests.length - 1 && <Divider />}
                      </React.Fragment>
                    );
                  })}
                </List>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Request Dialog */}
      <Dialog
        open={requestDialogOpen}
        onClose={() => setRequestDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          <Box display="flex" alignItems="center">
            <SecurityIcon sx={{ mr: 1 }} />
            Solicitar Escalonamento de Privilégios
          </Box>
        </DialogTitle>
        <DialogContent>
          <Box mt={2}>
            <FormControl fullWidth margin="normal">
              <InputLabel>Tipo de Privilégio</InputLabel>
              <Select
                value={requestType}
                onChange={(e) => setRequestType(e.target.value)}
                label="Tipo de Privilégio"
              >
                {privilegeTypes.map((type) => (
                  <MenuItem key={type.value} value={type.value}>
                    <Box display="flex" alignItems="center">
                      <Box mr={1}>{type.icon}</Box>
                      <Box>
                        <Typography variant="body1">{type.label}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {type.description}
                        </Typography>
                      </Box>
                    </Box>
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <TextField
              fullWidth
              margin="normal"
              label="Justificativa"
              value={requestReason}
              onChange={(e) => setRequestReason(e.target.value)}
              multiline
              rows={3}
              placeholder="Explique por que você precisa deste privilégio..."
              required
            />

            <FormControl fullWidth margin="normal">
              <InputLabel>Duração</InputLabel>
              <Select
                value={requestDuration}
                onChange={(e) => setRequestDuration(e.target.value)}
                label="Duração"
              >
                <MenuItem value={15}>15 minutos</MenuItem>
                <MenuItem value={30}>30 minutos</MenuItem>
                <MenuItem value={60}>1 hora</MenuItem>
                <MenuItem value={120}>2 horas</MenuItem>
                <MenuItem value={240}>4 horas</MenuItem>
              </Select>
            </FormControl>

            <Alert severity="warning" sx={{ mt: 2 }}>
              <Typography variant="body2">
                O cliente precisará aprovar esta solicitação antes que os privilégios sejam concedidos.
              </Typography>
            </Alert>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRequestDialogOpen(false)}>Cancelar</Button>
          <Button
            onClick={handlePrivilegeRequest}
            variant="contained"
            disabled={loading || !requestType || !requestReason.trim()}
          >
            {loading ? 'Solicitando...' : 'Solicitar'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default PrivilegeEscalation;