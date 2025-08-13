import React, { useState, useEffect, useCallback } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  TextField,
  Button,
  Alert,
  CircularProgress,
  Chip,
  Grid,
  Paper,
  Divider,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Tooltip
} from '@mui/material';
import {
  Computer as ComputerIcon,
  Security as SecurityIcon,
  VpnKey as VpnKeyIcon,
  CheckCircle as CheckCircleIcon,
  Error as ErrorIcon,
  Refresh as RefreshIcon,
  Settings as SettingsIcon,
  Info as InfoIcon
} from '@mui/icons-material';
import { useWebSocket } from '../../hooks/useWebSocket';
import FileTransferManager from '../FileTransfer/FileTransferManager';

const RemoteAccessPortal = () => {
  // State management
  const [sessionId, setSessionId] = useState('');
  const [isConnecting, setIsConnecting] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState(null);
  const [sessionInfo, setSessionInfo] = useState(null);
  const [showFileTransfer, setShowFileTransfer] = useState(false);
  const [showSessionDetails, setShowSessionDetails] = useState(false);
  const [technicianInfo, setTechnicianInfo] = useState({
    name: 'Técnico',
    email: 'tecnico@onlidesk.com'
  });

  // WebSocket connection for remote access
  const { isConnected: wsConnected, sendMessage, lastMessage } = useWebSocket(
    sessionId ? `ws://localhost:8080/ws/remote-access?session_id=${sessionId}&role=technician` : null,
    {
      onConnect: handleWebSocketConnect,
      onDisconnect: handleWebSocketDisconnect,
      onError: handleWebSocketError,
      onMessage: handleWebSocketMessage
    }
  );

  // WebSocket event handlers
  function handleWebSocketConnect() {
    console.log('Connected to remote access WebSocket');
    setIsConnected(true);
    setConnectionError(null);
    
    // Register technician session
    sendMessage({
      type: 'technician_register',
      session_id: sessionId,
      technician: technicianInfo
    });
  }

  function handleWebSocketDisconnect() {
    console.log('Disconnected from remote access WebSocket');
    setIsConnected(false);
    setSessionInfo(null);
  }

  function handleWebSocketError(error) {
    console.error('WebSocket error:', error);
    setConnectionError('Erro de conexão WebSocket');
    setIsConnecting(false);
    setIsConnected(false);
  }

  function handleWebSocketMessage(message) {
    try {
      const data = JSON.parse(message.data);
      
      switch (data.type) {
        case 'session_info':
          setSessionInfo(data.session);
          break;
        case 'client_connected':
          setSessionInfo(prev => ({
            ...prev,
            clientConnected: true,
            clientInfo: data.client
          }));
          break;
        case 'client_disconnected':
          setSessionInfo(prev => ({
            ...prev,
            clientConnected: false
          }));
          break;
        case 'privilege_escalation_request':
          handlePrivilegeEscalationRequest(data);
          break;
        case 'session_terminated':
          handleSessionTerminated(data);
          break;
        case 'error':
          setConnectionError(data.message);
          break;
        default:
          console.log('Unknown message type:', data.type);
      }
    } catch (error) {
      console.error('Error parsing WebSocket message:', error);
    }
  }

  // Handle privilege escalation request
  function handlePrivilegeEscalationRequest(data) {
    // Show privilege escalation dialog
    console.log('Privilege escalation requested:', data);
    // Implementation for privilege escalation UI
  }

  // Handle session termination
  function handleSessionTerminated(data) {
    setIsConnected(false);
    setSessionInfo(null);
    setSessionId('');
    console.log('Session terminated:', data.reason);
  }

  // Connect to remote session
  const handleConnect = useCallback(async () => {
    if (!sessionId.trim()) {
      setConnectionError('Por favor, insira um ID de sessão válido');
      return;
    }

    setIsConnecting(true);
    setConnectionError(null);

    try {
      // Validate session ID format
      if (!/^[A-Z0-9]{6,12}$/.test(sessionId.trim())) {
        throw new Error('ID de sessão deve conter 6-12 caracteres alfanuméricos');
      }

      // The WebSocket connection will be established automatically
      // when sessionId changes due to the useWebSocket dependency
      
    } catch (error) {
      setConnectionError(error.message);
      setIsConnecting(false);
    }
  }, [sessionId]);

  // Disconnect from session
  const handleDisconnect = useCallback(() => {
    if (wsConnected) {
      sendMessage({
        type: 'technician_disconnect',
        session_id: sessionId
      });
    }
    
    setIsConnected(false);
    setSessionInfo(null);
    setSessionId('');
  }, [wsConnected, sendMessage, sessionId]);

  // Request privilege escalation
  const handlePrivilegeEscalation = useCallback(() => {
    if (wsConnected) {
      sendMessage({
        type: 'privilege_escalation_request',
        session_id: sessionId,
        technician: technicianInfo
      });
    }
  }, [wsConnected, sendMessage, sessionId, technicianInfo]);

  // Terminate session
  const handleTerminateSession = useCallback(() => {
    if (wsConnected) {
      sendMessage({
        type: 'terminate_session',
        session_id: sessionId,
        reason: 'Terminated by technician'
      });
    }
  }, [wsConnected, sendMessage, sessionId]);

  // Update connecting state when WebSocket connection changes
  useEffect(() => {
    if (wsConnected) {
      setIsConnecting(false);
    }
  }, [wsConnected]);

  return (
    <Box sx={{ p: 3, maxWidth: 1200, mx: 'auto' }}>
      <Typography variant="h4" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <ComputerIcon color="primary" />
        Portal de Acesso Remoto
      </Typography>

      <Grid container spacing={3}>
        {/* Connection Panel */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <VpnKeyIcon />
                Conectar à Sessão
              </Typography>
              
              <Box sx={{ mt: 2 }}>
                <TextField
                  fullWidth
                  label="ID da Sessão"
                  value={sessionId}
                  onChange={(e) => setSessionId(e.target.value.toUpperCase())}
                  placeholder="Ex: ABC123DEF"
                  disabled={isConnected}
                  helperText="Insira o ID de sessão fornecido pelo cliente"
                  sx={{ mb: 2 }}
                />
                
                {connectionError && (
                  <Alert severity="error" sx={{ mb: 2 }}>
                    {connectionError}
                  </Alert>
                )}
                
                <Box sx={{ display: 'flex', gap: 1 }}>
                  {!isConnected ? (
                    <Button
                      variant="contained"
                      onClick={handleConnect}
                      disabled={isConnecting || !sessionId.trim()}
                      startIcon={isConnecting ? <CircularProgress size={20} /> : <VpnKeyIcon />}
                    >
                      {isConnecting ? 'Conectando...' : 'Conectar'}
                    </Button>
                  ) : (
                    <Button
                      variant="outlined"
                      color="error"
                      onClick={handleDisconnect}
                    >
                      Desconectar
                    </Button>
                  )}
                </Box>
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Session Status */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <SecurityIcon />
                Status da Sessão
              </Typography>
              
              <Box sx={{ mt: 2 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                  <Typography variant="body2">Conexão:</Typography>
                  <Chip
                    label={isConnected ? 'Conectado' : 'Desconectado'}
                    color={isConnected ? 'success' : 'default'}
                    size="small"
                    icon={isConnected ? <CheckCircleIcon /> : <ErrorIcon />}
                  />
                </Box>
                
                {sessionInfo && (
                  <>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                      <Typography variant="body2">Cliente:</Typography>
                      <Chip
                        label={sessionInfo.clientConnected ? 'Online' : 'Offline'}
                        color={sessionInfo.clientConnected ? 'success' : 'warning'}
                        size="small"
                      />
                    </Box>
                    
                    <Button
                      size="small"
                      startIcon={<InfoIcon />}
                      onClick={() => setShowSessionDetails(true)}
                    >
                      Ver Detalhes
                    </Button>
                  </>
                )}
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Remote Access Controls */}
        {isConnected && sessionInfo && (
          <Grid item xs={12}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Controles de Acesso Remoto
                </Typography>
                
                <Grid container spacing={2}>
                  <Grid item>
                    <Button
                      variant="contained"
                      startIcon={<SecurityIcon />}
                      onClick={handlePrivilegeEscalation}
                      disabled={!sessionInfo.clientConnected}
                    >
                      Solicitar Privilégios
                    </Button>
                  </Grid>
                  
                  <Grid item>
                    <Button
                      variant="outlined"
                      startIcon={<RefreshIcon />}
                      onClick={() => setShowFileTransfer(true)}
                      disabled={!sessionInfo.clientConnected}
                    >
                      Transferência de Arquivos
                    </Button>
                  </Grid>
                  
                  <Grid item>
                    <Button
                      variant="outlined"
                      color="error"
                      onClick={handleTerminateSession}
                    >
                      Encerrar Sessão
                    </Button>
                  </Grid>
                </Grid>
              </CardContent>
            </Card>
          </Grid>
        )}
      </Grid>

      {/* File Transfer Dialog */}
      <Dialog
        open={showFileTransfer}
        onClose={() => setShowFileTransfer(false)}
        maxWidth="lg"
        fullWidth
      >
        <DialogTitle>Transferência de Arquivos</DialogTitle>
        <DialogContent>
          {sessionId && (
            <FileTransferManager
              sessionId={sessionId}
              userRole="technician"
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowFileTransfer(false)}>Fechar</Button>
        </DialogActions>
      </Dialog>

      {/* Session Details Dialog */}
      <Dialog
        open={showSessionDetails}
        onClose={() => setShowSessionDetails(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Detalhes da Sessão</DialogTitle>
        <DialogContent>
          {sessionInfo && (
            <List>
              <ListItem>
                <ListItemIcon><VpnKeyIcon /></ListItemIcon>
                <ListItemText
                  primary="ID da Sessão"
                  secondary={sessionId}
                />
              </ListItem>
              
              <ListItem>
                <ListItemIcon><ComputerIcon /></ListItemIcon>
                <ListItemText
                  primary="Status do Cliente"
                  secondary={sessionInfo.clientConnected ? 'Conectado' : 'Desconectado'}
                />
              </ListItem>
              
              {sessionInfo.clientInfo && (
                <>
                  <ListItem>
                    <ListItemText
                      primary="Informações do Cliente"
                      secondary={`OS: ${sessionInfo.clientInfo.os || 'N/A'}, IP: ${sessionInfo.clientInfo.ip || 'N/A'}`}
                    />
                  </ListItem>
                </>
              )}
              
              <ListItem>
                <ListItemText
                  primary="Horário de Conexão"
                  secondary={sessionInfo.connectedAt ? new Date(sessionInfo.connectedAt).toLocaleString() : 'N/A'}
                />
              </ListItem>
            </List>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowSessionDetails(false)}>Fechar</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default RemoteAccessPortal;