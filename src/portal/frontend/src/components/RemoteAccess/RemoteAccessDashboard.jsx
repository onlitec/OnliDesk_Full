import React, { useState, useEffect } from 'react';
import {
  Box,
  Container,
  Typography,
  Tabs,
  Tab,
  Paper,
  Alert,
  Snackbar,
  Breadcrumbs,
  Link,
  Chip,
  Grid,
  Card,
  CardContent,
  IconButton,
  Tooltip,
} from '@mui/material';
import {
  Dashboard as DashboardIcon,
  Computer as ComputerIcon,
  Security as SecurityIcon,
  Settings as SettingsIcon,
  Home as HomeIcon,
  Refresh as RefreshIcon,
  Fullscreen as FullscreenIcon,
  FullscreenExit as FullscreenExitIcon,
} from '@mui/icons-material';
import { useRemoteAccess } from '../../hooks/useRemoteAccess';
import RemoteAccessPortal from './RemoteAccessPortal';
import SessionManager from './SessionManager';
import PrivilegeEscalation from './PrivilegeEscalation';

const RemoteAccessDashboard = () => {
  const [activeTab, setActiveTab] = useState(0);
  const [selectedSessionId, setSelectedSessionId] = useState('');
  const [notification, setNotification] = useState(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [lastActivity, setLastActivity] = useState(Date.now());

  const {
    sessionId,
    isConnected,
    connectionStatus,
    sessionInfo,
    clientInfo,
    error,
    messages,
    clearError,
    connectToSession,
    disconnect,
  } = useRemoteAccess({
    onConnectionChange: (connected) => {
      if (connected) {
        showNotification('Conectado com sucesso à sessão', 'success');
        setActiveTab(1); // Switch to portal tab
      } else {
        showNotification('Desconectado da sessão', 'info');
      }
    },
    onError: (error) => {
      showNotification(error.message || 'Erro de conexão', 'error');
    },
    onMessage: (message) => {
      setLastActivity(Date.now());
      
      // Handle specific message types
      switch (message.type) {
        case 'privilege_request':
          showNotification('Nova solicitação de privilégio recebida', 'warning');
          break;
        case 'client_connected':
          showNotification('Cliente conectado à sessão', 'success');
          break;
        case 'client_disconnected':
          showNotification('Cliente desconectado da sessão', 'warning');
          break;
        default:
          break;
      }
    }
  });

  /**
   * Show notification
   */
  const showNotification = (message, severity = 'info') => {
    setNotification({ message, severity });
  };

  /**
   * Close notification
   */
  const closeNotification = () => {
    setNotification(null);
  };

  /**
   * Handle tab change
   */
  const handleTabChange = (event, newValue) => {
    setActiveTab(newValue);
  };

  /**
   * Handle session selection from manager
   */
  const handleSessionSelect = (sessionId) => {
    setSelectedSessionId(sessionId);
    if (sessionId && sessionId !== sessionId) {
      connectToSession(sessionId);
    }
  };

  /**
   * Handle privilege change
   */
  const handlePrivilegeChange = (action, request) => {
    switch (action) {
      case 'requested':
        showNotification(`Privilégio ${request.type} solicitado`, 'info');
        break;
      case 'approved':
        showNotification(`Privilégio ${request.type} aprovado`, 'success');
        break;
      case 'denied':
        showNotification(`Privilégio ${request.type} negado`, 'warning');
        break;
      case 'revoked':
        showNotification('Privilégio revogado', 'info');
        break;
      case 'expired':
        showNotification(`Privilégio ${request.type} expirado`, 'warning');
        break;
      default:
        break;
    }
  };

  /**
   * Toggle fullscreen mode
   */
  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
      setIsFullscreen(true);
    } else {
      document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  /**
   * Get connection status color
   */
  const getConnectionStatusColor = () => {
    switch (connectionStatus) {
      case 'connected':
        return 'success';
      case 'connecting':
        return 'warning';
      case 'error':
        return 'error';
      default:
        return 'default';
    }
  };

  /**
   * Get connection status text
   */
  const getConnectionStatusText = () => {
    switch (connectionStatus) {
      case 'connected':
        return 'Conectado';
      case 'connecting':
        return 'Conectando...';
      case 'error':
        return 'Erro de Conexão';
      default:
        return 'Desconectado';
    }
  };

  /**
   * Format last activity time
   */
  const formatLastActivity = () => {
    const diff = Date.now() - lastActivity;
    const minutes = Math.floor(diff / (1000 * 60));
    const seconds = Math.floor((diff % (1000 * 60)) / 1000);
    
    if (minutes > 0) {
      return `${minutes}m ${seconds}s atrás`;
    }
    return `${seconds}s atrás`;
  };

  // Listen for fullscreen changes
  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };

    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  }, []);

  // Auto-refresh last activity
  useEffect(() => {
    const interval = setInterval(() => {
      // Force re-render to update "last activity" display
      setLastActivity(prev => prev);
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  const tabs = [
    {
      label: 'Dashboard',
      icon: <DashboardIcon />,
      component: <SessionManager onSessionSelect={handleSessionSelect} />
    },
    {
      label: 'Portal de Acesso',
      icon: <ComputerIcon />,
      component: (
        <RemoteAccessPortal
          sessionId={selectedSessionId}
          onSessionIdChange={setSelectedSessionId}
        />
      )
    },
    {
      label: 'Privilégios',
      icon: <SecurityIcon />,
      component: (
        <PrivilegeEscalation
          sessionId={sessionId}
          onPrivilegeChange={handlePrivilegeChange}
        />
      )
    }
  ];

  return (
    <Container maxWidth={false} sx={{ py: 3 }}>
      {/* Header */}
      <Box mb={3}>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
          <Box>
            <Breadcrumbs>
              <Link color="inherit" href="/" sx={{ display: 'flex', alignItems: 'center' }}>
                <HomeIcon sx={{ mr: 0.5 }} fontSize="inherit" />
                Início
              </Link>
              <Typography color="text.primary" sx={{ display: 'flex', alignItems: 'center' }}>
                <ComputerIcon sx={{ mr: 0.5 }} fontSize="inherit" />
                Acesso Remoto
              </Typography>
            </Breadcrumbs>
            <Typography variant="h4" component="h1" sx={{ mt: 1 }}>
              🖥️ Acesso Remoto Onlidesk
            </Typography>
          </Box>
          
          <Box display="flex" alignItems="center" gap={2}>
            {/* Connection Status */}
            <Chip
              label={getConnectionStatusText()}
              color={getConnectionStatusColor()}
              variant="outlined"
            />
            
            {/* Fullscreen Toggle */}
            <Tooltip title={isFullscreen ? 'Sair da Tela Cheia' : 'Tela Cheia'}>
              <IconButton onClick={toggleFullscreen}>
                {isFullscreen ? <FullscreenExitIcon /> : <FullscreenIcon />}
              </IconButton>
            </Tooltip>
          </Box>
        </Box>

        {/* Session Info */}
        {isConnected && sessionInfo && (
          <Grid container spacing={2} mb={2}>
            <Grid item xs={12} md={4}>
              <Card variant="outlined">
                <CardContent sx={{ py: 1.5 }}>
                  <Typography variant="body2" color="text.secondary">
                    Sessão Ativa
                  </Typography>
                  <Typography variant="h6" fontFamily="monospace">
                    {sessionId}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
            
            {clientInfo && (
              <Grid item xs={12} md={4}>
                <Card variant="outlined">
                  <CardContent sx={{ py: 1.5 }}>
                    <Typography variant="body2" color="text.secondary">
                      Cliente
                    </Typography>
                    <Typography variant="h6">
                      {clientInfo.computerName || 'N/A'}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {clientInfo.ipAddress || 'N/A'}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
            )}
            
            <Grid item xs={12} md={4}>
              <Card variant="outlined">
                <CardContent sx={{ py: 1.5 }}>
                  <Typography variant="body2" color="text.secondary">
                    Última Atividade
                  </Typography>
                  <Typography variant="h6">
                    {formatLastActivity()}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        )}
      </Box>

      {/* Error Alert */}
      {error && (
        <Alert
          severity="error"
          sx={{ mb: 3 }}
          onClose={clearError}
          action={
            <IconButton size="small" onClick={() => window.location.reload()}>
              <RefreshIcon />
            </IconButton>
          }
        >
          {error}
        </Alert>
      )}

      {/* Main Content */}
      <Paper sx={{ width: '100%' }}>
        {/* Tabs */}
        <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
          <Tabs
            value={activeTab}
            onChange={handleTabChange}
            variant="fullWidth"
            sx={{ minHeight: 64 }}
          >
            {tabs.map((tab, index) => (
              <Tab
                key={index}
                label={tab.label}
                icon={tab.icon}
                iconPosition="start"
                sx={{ minHeight: 64 }}
              />
            ))}
          </Tabs>
        </Box>

        {/* Tab Content */}
        <Box sx={{ p: 3 }}>
          {tabs[activeTab]?.component}
        </Box>
      </Paper>

      {/* Notification Snackbar */}
      <Snackbar
        open={!!notification}
        autoHideDuration={6000}
        onClose={closeNotification}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        {notification && (
          <Alert
            onClose={closeNotification}
            severity={notification.severity}
            variant="filled"
            sx={{ width: '100%' }}
          >
            {notification.message}
          </Alert>
        )}
      </Snackbar>
    </Container>
  );
};

export default RemoteAccessDashboard;