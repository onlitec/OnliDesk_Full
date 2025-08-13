import { useState, useEffect, useRef, useCallback } from 'react';
import FileTransferService from '../services/FileTransferService';

/**
 * Custom hook for managing file transfers with WebSocket communication
 * @param {string} sessionId - The session ID
 * @param {string} role - User role (client/technician)
 * @returns {Object} File transfer state and methods
 */
export const useFileTransfer = (sessionId, role = 'technician') => {
  // State management
  const [transfers, setTransfers] = useState([]);
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState(null);
  const [settings, setSettings] = useState(null);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Refs for WebSocket and cleanup
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const heartbeatIntervalRef = useRef(null);
  const reconnectAttemptsRef = useRef(0);
  const maxReconnectAttempts = 5;
  const reconnectDelay = 3000;

  /**
   * Initialize WebSocket connection
   */
  const connectWebSocket = useCallback(() => {
    if (!sessionId) {
      console.warn('Cannot connect WebSocket: sessionId is required');
      return;
    }

    try {
      const wsUrl = FileTransferService.getWebSocketUrl(sessionId, role);
      console.log('Connecting to WebSocket:', wsUrl);
      
      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        console.log('WebSocket connected');
        setIsConnected(true);
        setConnectionError(null);
        reconnectAttemptsRef.current = 0;

        // Send registration message
        const registrationMessage = {
          type: 'register',
          session_id: sessionId,
          role: role,
          timestamp: new Date().toISOString()
        };
        
        wsRef.current.send(JSON.stringify(registrationMessage));

        // Start heartbeat
        startHeartbeat();
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          handleWebSocketMessage(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      wsRef.current.onclose = (event) => {
        console.log('WebSocket disconnected:', event.code, event.reason);
        setIsConnected(false);
        stopHeartbeat();

        // Attempt to reconnect if not a clean close
        if (event.code !== 1000 && reconnectAttemptsRef.current < maxReconnectAttempts) {
          scheduleReconnect();
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('WebSocket error:', error);
        setConnectionError('WebSocket connection failed');
      };

    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
      setConnectionError('Failed to create WebSocket connection');
    }
  }, [sessionId, role]);

  /**
   * Handle incoming WebSocket messages
   */
  const handleWebSocketMessage = useCallback((message) => {
    console.log('Received WebSocket message:', message);

    switch (message.type) {
      case 'transfer_request':
        handleTransferRequest(message.data);
        break;
      case 'transfer_approved':
        handleTransferApproved(message.data);
        break;
      case 'transfer_rejected':
        handleTransferRejected(message.data);
        break;
      case 'transfer_progress':
        handleTransferProgress(message.data);
        break;
      case 'transfer_completed':
        handleTransferCompleted(message.data);
        break;
      case 'transfer_failed':
        handleTransferFailed(message.data);
        break;
      case 'transfer_cancelled':
        handleTransferCancelled(message.data);
        break;
      case 'transfer_paused':
        handleTransferPaused(message.data);
        break;
      case 'transfer_resumed':
        handleTransferResumed(message.data);
        break;
      case 'pong':
        // Heartbeat response
        break;
      case 'error':
        handleWebSocketError(message.data);
        break;
      default:
        console.warn('Unknown message type:', message.type);
    }
  }, []);

  /**
   * Handle transfer request
   */
  const handleTransferRequest = useCallback((data) => {
    setTransfers(prev => {
      const existingIndex = prev.findIndex(t => t.id === data.id);
      if (existingIndex >= 0) {
        const updated = [...prev];
        updated[existingIndex] = { ...updated[existingIndex], ...data };
        return updated;
      } else {
        return [...prev, data];
      }
    });
  }, []);

  /**
   * Handle transfer approval
   */
  const handleTransferApproved = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { ...transfer, status: 'approved', ...data }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer rejection
   */
  const handleTransferRejected = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { ...transfer, status: 'rejected', ...data }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer progress updates
   */
  const handleTransferProgress = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'in_progress',
              progress: data.progress,
              speed: data.speed,
              eta: data.eta,
              transferred_bytes: data.transferred_bytes,
              last_updated: new Date().toISOString()
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer completion
   */
  const handleTransferCompleted = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'completed',
              progress: 100,
              completed_at: new Date().toISOString(),
              ...data
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer failure
   */
  const handleTransferFailed = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'failed',
              error: data.error,
              failed_at: new Date().toISOString()
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer cancellation
   */
  const handleTransferCancelled = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'cancelled',
              cancelled_at: new Date().toISOString()
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer pause
   */
  const handleTransferPaused = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'paused',
              paused_at: new Date().toISOString()
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle transfer resume
   */
  const handleTransferResumed = useCallback((data) => {
    setTransfers(prev => 
      prev.map(transfer => 
        transfer.id === data.id 
          ? { 
              ...transfer, 
              status: 'in_progress',
              resumed_at: new Date().toISOString()
            }
          : transfer
      )
    );
  }, []);

  /**
   * Handle WebSocket errors
   */
  const handleWebSocketError = useCallback((data) => {
    console.error('WebSocket error:', data);
    setError(data.message || 'WebSocket error occurred');
  }, []);

  /**
   * Start heartbeat to keep connection alive
   */
  const startHeartbeat = useCallback(() => {
    heartbeatIntervalRef.current = setInterval(() => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000); // Send ping every 30 seconds
  }, []);

  /**
   * Stop heartbeat
   */
  const stopHeartbeat = useCallback(() => {
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
      heartbeatIntervalRef.current = null;
    }
  }, []);

  /**
   * Schedule reconnection attempt
   */
  const scheduleReconnect = useCallback(() => {
    reconnectAttemptsRef.current += 1;
    console.log(`Scheduling reconnect attempt ${reconnectAttemptsRef.current}/${maxReconnectAttempts}`);
    
    reconnectTimeoutRef.current = setTimeout(() => {
      connectWebSocket();
    }, reconnectDelay * reconnectAttemptsRef.current);
  }, [connectWebSocket]);

  /**
   * Disconnect WebSocket
   */
  const disconnectWebSocket = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    
    stopHeartbeat();
    
    if (wsRef.current) {
      wsRef.current.close(1000, 'Component unmounting');
      wsRef.current = null;
    }
    
    setIsConnected(false);
  }, [stopHeartbeat]);

  /**
   * Send WebSocket message
   */
  const sendMessage = useCallback((message) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
      return true;
    } else {
      console.warn('WebSocket is not connected');
      return false;
    }
  }, []);

  /**
   * Upload file
   */
  const uploadFile = useCallback(async (file, options = {}) => {
    try {
      setLoading(true);
      setError(null);

      // Validate file first
      const validationResponse = await FileTransferService.validateFile(file);
      if (!validationResponse.data.valid) {
        throw new Error(validationResponse.data.message || 'File validation failed');
      }

      // Create form data
      const formData = new FormData();
      formData.append('file', file);
      formData.append('session_id', sessionId);
      formData.append('type', 'upload');
      
      if (options.description) {
        formData.append('description', options.description);
      }
      
      if (options.technician) {
        formData.append('technician', options.technician);
      }

      // Upload file
      const response = await FileTransferService.uploadFile(formData);
      
      // Add to transfers list
      const transferData = response.data;
      setTransfers(prev => [...prev, transferData]);
      
      return transferData;
    } catch (error) {
      console.error('Upload failed:', error);
      setError(error.message || 'Upload failed');
      throw error;
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  /**
   * Request file download
   */
  const requestDownload = useCallback(async (filename, options = {}) => {
    try {
      setLoading(true);
      setError(null);

      const downloadRequest = {
        filename,
        session_id: sessionId,
        type: 'download',
        technician: options.technician || 'Unknown',
        description: options.description || ''
      };

      const response = await FileTransferService.requestDownload(downloadRequest);
      
      // Add to transfers list
      const transferData = response.data;
      setTransfers(prev => [...prev, transferData]);
      
      return transferData;
    } catch (error) {
      console.error('Download request failed:', error);
      setError(error.message || 'Download request failed');
      throw error;
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  /**
   * Approve transfer
   */
  const approveTransfer = useCallback(async (transferId, message = '') => {
    try {
      await FileTransferService.approveTransfer(transferId, true, message);
      
      // Send WebSocket message for real-time update
      sendMessage({
        type: 'approve_transfer',
        transfer_id: transferId,
        approved: true,
        message
      });
    } catch (error) {
      console.error('Approve transfer failed:', error);
      setError(error.message || 'Approve transfer failed');
      throw error;
    }
  }, [sendMessage]);

  /**
   * Reject transfer
   */
  const rejectTransfer = useCallback(async (transferId, message = '') => {
    try {
      await FileTransferService.approveTransfer(transferId, false, message);
      
      // Send WebSocket message for real-time update
      sendMessage({
        type: 'approve_transfer',
        transfer_id: transferId,
        approved: false,
        message
      });
    } catch (error) {
      console.error('Reject transfer failed:', error);
      setError(error.message || 'Reject transfer failed');
      throw error;
    }
  }, [sendMessage]);

  /**
   * Pause transfer
   */
  const pauseTransfer = useCallback(async (transferId) => {
    try {
      await FileTransferService.pauseTransfer(transferId);
      
      // Send WebSocket message for real-time update
      sendMessage({
        type: 'pause_transfer',
        transfer_id: transferId
      });
    } catch (error) {
      console.error('Pause transfer failed:', error);
      setError(error.message || 'Pause transfer failed');
      throw error;
    }
  }, [sendMessage]);

  /**
   * Resume transfer
   */
  const resumeTransfer = useCallback(async (transferId) => {
    try {
      await FileTransferService.resumeTransfer(transferId);
      
      // Send WebSocket message for real-time update
      sendMessage({
        type: 'resume_transfer',
        transfer_id: transferId
      });
    } catch (error) {
      console.error('Resume transfer failed:', error);
      setError(error.message || 'Resume transfer failed');
      throw error;
    }
  }, [sendMessage]);

  /**
   * Cancel transfer
   */
  const cancelTransfer = useCallback(async (transferId) => {
    try {
      await FileTransferService.cancelTransfer(transferId);
      
      // Send WebSocket message for real-time update
      sendMessage({
        type: 'cancel_transfer',
        transfer_id: transferId
      });
    } catch (error) {
      console.error('Cancel transfer failed:', error);
      setError(error.message || 'Cancel transfer failed');
      throw error;
    }
  }, [sendMessage]);

  /**
   * Delete transfer
   */
  const deleteTransfer = useCallback(async (transferId) => {
    try {
      await FileTransferService.deleteTransfer(transferId);
      
      // Remove from local state
      setTransfers(prev => prev.filter(t => t.id !== transferId));
    } catch (error) {
      console.error('Delete transfer failed:', error);
      setError(error.message || 'Delete transfer failed');
      throw error;
    }
  }, []);

  /**
   * Download completed file
   */
  const downloadFile = useCallback(async (transferId, filename) => {
    try {
      const response = await FileTransferService.downloadFile(transferId);
      
      // Create download link
      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', filename);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Download file failed:', error);
      setError(error.message || 'Download file failed');
      throw error;
    }
  }, []);

  /**
   * Load transfers from API
   */
  const loadTransfers = useCallback(async () => {
    try {
      setLoading(true);
      const response = await FileTransferService.getTransfers(sessionId);
      setTransfers(response.data);
    } catch (error) {
      console.error('Load transfers failed:', error);
      setError(error.message || 'Load transfers failed');
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  /**
   * Load settings
   */
  const loadSettings = useCallback(async () => {
    try {
      const response = await FileTransferService.getSettings();
      setSettings(response.data);
    } catch (error) {
      console.error('Load settings failed:', error);
    }
  }, []);

  /**
   * Load statistics
   */
  const loadStats = useCallback(async () => {
    try {
      const response = await FileTransferService.getStats(sessionId);
      setStats(response.data);
    } catch (error) {
      console.error('Load stats failed:', error);
    }
  }, [sessionId]);

  /**
   * Clear error
   */
  const clearError = useCallback(() => {
    setError(null);
  }, []);

  /**
   * Refresh data
   */
  const refresh = useCallback(async () => {
    await Promise.all([
      loadTransfers(),
      loadSettings(),
      loadStats()
    ]);
  }, [loadTransfers, loadSettings, loadStats]);

  // Initialize connection and load data
  useEffect(() => {
    if (sessionId) {
      connectWebSocket();
      loadTransfers();
      loadSettings();
      loadStats();
    }

    return () => {
      disconnectWebSocket();
    };
  }, [sessionId, connectWebSocket, disconnectWebSocket, loadTransfers, loadSettings, loadStats]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      disconnectWebSocket();
    };
  }, [disconnectWebSocket]);

  return {
    // State
    transfers,
    isConnected,
    connectionError,
    settings,
    stats,
    loading,
    error,
    
    // Actions
    uploadFile,
    requestDownload,
    approveTransfer,
    rejectTransfer,
    pauseTransfer,
    resumeTransfer,
    cancelTransfer,
    deleteTransfer,
    downloadFile,
    
    // Utilities
    refresh,
    clearError,
    sendMessage,
    
    // Connection management
    connectWebSocket,
    disconnectWebSocket
  };
};

export default useFileTransfer;