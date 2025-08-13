import React, { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Chip,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Tooltip,
  Grid,
  CircularProgress,
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Add as AddIcon,
  MoreVert as MoreVertIcon,
  Visibility as ViewIcon,
  Stop as StopIcon,
  Delete as DeleteIcon,
  Info as InfoIcon,
  Computer as ComputerIcon,
  Person as PersonIcon,
  Schedule as ScheduleIcon,
} from '@mui/icons-material';
import { RemoteAccessService } from '../../services/RemoteAccessService';
import { useRemoteAccess } from '../../hooks/useRemoteAccess';

const SessionManager = ({ onSessionSelect }) => {
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [terminateDialogOpen, setTerminateDialogOpen] = useState(false);
  const [selectedSession, setSelectedSession] = useState(null);
  const [terminationReason, setTerminationReason] = useState('');
  const [menuAnchor, setMenuAnchor] = useState(null);
  const [menuSession, setMenuSession] = useState(null);
  const [stats, setStats] = useState(null);

  const { loadActiveSessions, loadSessionStats } = useRemoteAccess();

  /**
   * Load sessions from server
   */
  const loadSessions = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await RemoteAccessService.getActiveSessions();
      setSessions(response.data.sessions || []);
      
      // Load overall statistics
      const statsResponse = await RemoteAccessService.getSessionStats();
      setStats(statsResponse.data);
    } catch (err) {
      console.error('Failed to load sessions:', err);
      setError(err.response?.data?.message || 'Failed to load sessions');
    } finally {
      setLoading(false);
    }
  };

  /**
   * Generate new session ID
   */
  const generateNewSession = async () => {
    try {
      const response = await RemoteAccessService.generateSessionId();
      const newSession = response.data;
      
      setSessions(prev => [newSession, ...prev]);
      setCreateDialogOpen(false);
      
      // Optionally select the new session
      if (onSessionSelect) {
        onSessionSelect(newSession.sessionId);
      }
    } catch (err) {
      console.error('Failed to generate session:', err);
      setError(err.response?.data?.message || 'Failed to generate new session');
    }
  };

  /**
   * Terminate session
   */
  const terminateSession = async () => {
    if (!selectedSession) return;
    
    try {
      await RemoteAccessService.terminateSession(
        selectedSession.sessionId,
        terminationReason || 'Terminated by administrator'
      );
      
      // Remove from local state
      setSessions(prev => prev.filter(s => s.sessionId !== selectedSession.sessionId));
      
      setTerminateDialogOpen(false);
      setSelectedSession(null);
      setTerminationReason('');
    } catch (err) {
      console.error('Failed to terminate session:', err);
      setError(err.response?.data?.message || 'Failed to terminate session');
    }
  };

  /**
   * Get session status color
   */
  const getStatusColor = (status) => {
    return RemoteAccessService.getSessionStatusColor(status);
  };

  /**
   * Format session duration
   */
  const formatDuration = (startTime, endTime) => {
    return RemoteAccessService.formatSessionDuration(startTime, endTime);
  };

  /**
   * Handle menu open
   */
  const handleMenuOpen = (event, session) => {
    setMenuAnchor(event.currentTarget);
    setMenuSession(session);
  };

  /**
   * Handle menu close
   */
  const handleMenuClose = () => {
    setMenuAnchor(null);
    setMenuSession(null);
  };

  /**
   * Handle session view
   */
  const handleViewSession = (session) => {
    if (onSessionSelect) {
      onSessionSelect(session.sessionId);
    }
    handleMenuClose();
  };

  /**
   * Handle session termination
   */
  const handleTerminateSession = (session) => {
    setSelectedSession(session);
    setTerminateDialogOpen(true);
    handleMenuClose();
  };

  // Load sessions on component mount
  useEffect(() => {
    loadSessions();
    
    // Set up auto-refresh every 30 seconds
    const interval = setInterval(loadSessions, 30000);
    
    return () => clearInterval(interval);
  }, []);

  return (
    <Box>
      {/* Header */}
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h5" component="h2">
          🖥️ Gerenciador de Sessões
        </Typography>
        <Box>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={loadSessions}
            disabled={loading}
            sx={{ mr: 1 }}
          >
            Atualizar
          </Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => setCreateDialogOpen(true)}
          >
            Nova Sessão
          </Button>
        </Box>
      </Box>

      {/* Statistics Cards */}
      {stats && (
        <Grid container spacing={2} mb={3}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center">
                  <ComputerIcon color="primary" sx={{ mr: 1 }} />
                  <Box>
                    <Typography variant="h6">{stats.totalSessions || 0}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Total de Sessões
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center">
                  <PersonIcon color="success" sx={{ mr: 1 }} />
                  <Box>
                    <Typography variant="h6">{stats.activeSessions || 0}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Sessões Ativas
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center">
                  <ScheduleIcon color="warning" sx={{ mr: 1 }} />
                  <Box>
                    <Typography variant="h6">{stats.averageDuration || '0m'}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Duração Média
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center">
                  <InfoIcon color="info" sx={{ mr: 1 }} />
                  <Box>
                    <Typography variant="h6">{stats.todaySessions || 0}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      Sessões Hoje
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}

      {/* Error Alert */}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Sessions Table */}
      <Card>
        <CardContent>
          {loading ? (
            <Box display="flex" justifyContent="center" p={3}>
              <CircularProgress />
            </Box>
          ) : sessions.length === 0 ? (
            <Box textAlign="center" p={3}>
              <Typography variant="body1" color="text.secondary">
                Nenhuma sessão ativa encontrada
              </Typography>
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>ID da Sessão</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell>Cliente</TableCell>
                    <TableCell>Técnico</TableCell>
                    <TableCell>Iniciado</TableCell>
                    <TableCell>Duração</TableCell>
                    <TableCell align="right">Ações</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {sessions.map((session) => (
                    <TableRow key={session.sessionId} hover>
                      <TableCell>
                        <Typography variant="body2" fontFamily="monospace">
                          {session.sessionId}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={session.status}
                          color={getStatusColor(session.status)}
                          size="small"
                        />
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2">
                            {session.clientInfo?.computerName || 'N/A'}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {session.clientInfo?.ipAddress || 'N/A'}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2">
                            {session.technicianInfo?.name || 'N/A'}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {session.technicianInfo?.email || 'N/A'}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {new Date(session.startTime).toLocaleString()}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatDuration(session.startTime, session.endTime)}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={(e) => handleMenuOpen(e, session)}
                        >
                          <MoreVertIcon />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </CardContent>
      </Card>

      {/* Action Menu */}
      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor)}
        onClose={handleMenuClose}
      >
        <MenuItem onClick={() => handleViewSession(menuSession)}>
          <ListItemIcon>
            <ViewIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Visualizar Sessão</ListItemText>
        </MenuItem>
        <MenuItem onClick={() => handleTerminateSession(menuSession)}>
          <ListItemIcon>
            <StopIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>Encerrar Sessão</ListItemText>
        </MenuItem>
      </Menu>

      {/* Create Session Dialog */}
      <Dialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)}>
        <DialogTitle>Criar Nova Sessão</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" mb={2}>
            Uma nova sessão será criada com um ID único. O cliente poderá usar este ID para conectar-se.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateDialogOpen(false)}>Cancelar</Button>
          <Button onClick={generateNewSession} variant="contained">
            Criar Sessão
          </Button>
        </DialogActions>
      </Dialog>

      {/* Terminate Session Dialog */}
      <Dialog open={terminateDialogOpen} onClose={() => setTerminateDialogOpen(false)}>
        <DialogTitle>Encerrar Sessão</DialogTitle>
        <DialogContent>
          <Typography variant="body2" mb={2}>
            Tem certeza de que deseja encerrar a sessão {selectedSession?.sessionId}?
          </Typography>
          <TextField
            fullWidth
            label="Motivo do encerramento (opcional)"
            value={terminationReason}
            onChange={(e) => setTerminationReason(e.target.value)}
            multiline
            rows={3}
            placeholder="Ex: Suporte concluído, problema resolvido..."
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setTerminateDialogOpen(false)}>Cancelar</Button>
          <Button onClick={terminateSession} variant="contained" color="error">
            Encerrar Sessão
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default SessionManager;