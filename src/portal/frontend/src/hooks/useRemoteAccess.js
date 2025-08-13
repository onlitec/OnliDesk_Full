import { useState, useEffect, useRef, useCallback } from 'react';
import { RemoteAccessService } from '../services/RemoteAccessService';

/**
 * Custom hook for managing remote access functionality
 * @param {Object} options - Configuration options
 * @returns {Object} Remote access state and methods
 */
export const useRemoteAccess = (options = {}) => {
  const {
    autoConnect = false,
    reconnectAttempts = 3,
    reconnectDelay = 5000,
    onConnectionChange = null,
    onError = null,
    onMessage = null,
  } = options;

  // State management
  const [sessionId, setSessionId] = useState('');
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState('disconnected'); // disconnected, connecting, connected, error
  const [sessionInfo, setSessionInfo] = useState(null);
  const [clientInfo, setClientInfo] = useState(null);
  const [error, setError] = useState(null);
  const [messages, setMessages] = useState([]);
  const [activeSessions, setActiveSessions] = useState([]);
  const [sessionStats, setSessionStats] = useState(null);
  const [privilegeRequests, setPrivilegeRequests] = useState([]);

  // Refs for WebSocket and reconnection
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const reconnectCountRef = useRef(0);
  const isManualDisconnectRef = useRef(false);

  /**
   * Clear error state
   */
  const clearError = useCallback(() => {
    setError(null);
  }, []);

  /**
   * Add message to messages array
   */
  const addMessage = useCallback((message) => {
    setMessages(prev => [...prev, {
      ...message,
      timestamp: Date.now(),
      id: Math.random().toString(36).substr(2, 9)
    }]);
  }, []);

  /**
   * Clear all messages
   */
  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  /**
   * Handle WebSocket connection
   */
  const connectWebSocket = useCallback((sessionId, role = 'technician') => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    setIsConnecting(true);
    setConnectionStatus('connecting');
    clearError();

    try {
      const wsUrl = RemoteAccessService.getWebSocketUrl(sessionId, role);
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('WebSocket connected for remote access');
        setIsConnected(true);
        setIsConnecting(false);
        setConnectionStatus('connected');
        reconnectCountRef.current = 0;
        
        // Send initial authentication
        ws.send(JSON.stringify({
          type: 'auth',
          sessionId,
          role,
          timestamp: Date.now()
        }));

        onConnectionChange?.(true);
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          console.log('WebSocket message received:', data);

          switch (data.type) {
            case 'session_info':
              setSessionInfo(data.session);
              break;
            case 'client_info':
              setClientInfo(data.client);
              break;
            case 'privilege_request':
              setPrivilegeRequests(prev => [...prev, data.request]);
              break;
            case 'privilege_response':
              // Handle privilege escalation response
              addMessage({
                type: 'privilege_response',
                content: `Privilege request ${data.approved ? 'approved' : 'denied'}`,
                level: data.approved ? 'success' : 'warning'
              });
              break;
            case 'session_terminated':
              addMessage({
                type: 'session_terminated',
                content: `Session terminated: ${data.reason}`,
                level: 'info'
              });
              disconnect();
              break;
            case 'error':
              setError(data.message);
              addMessage({
                type: 'error',
                content: data.message,
                level: 'error'
              });
              break;
            case 'control_response':
              addMessage({
                type: 'control_response',
                content: `Control command ${data.success ? 'executed' : 'failed'}`,
                level: data.success ? 'success' : 'error'
              });
              break;
            default:
              addMessage({
                type: data.type || 'message',
                content: data.message || JSON.stringify(data),
                level: 'info'
              });
          }

          onMessage?.(data);
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
          addMessage({
            type: 'error',
            content: 'Failed to parse message from server',
            level: 'error'
          });
        }
      };

      ws.onclose = (event) => {
        console.log('WebSocket disconnected:', event.code, event.reason);
        setIsConnected(false);
        setIsConnecting(false);
        setConnectionStatus('disconnected');
        onConnectionChange?.(false);

        // Attempt reconnection if not manually disconnected
        if (!isManualDisconnectRef.current && reconnectCountRef.current < reconnectAttempts) {
          reconnectCountRef.current++;
          addMessage({
            type: 'reconnect',
            content: `Attempting to reconnect (${reconnectCountRef.current}/${reconnectAttempts})...`,
            level: 'warning'
          });
          
          reconnectTimeoutRef.current = setTimeout(() => {
            connectWebSocket(sessionId, role);
          }, reconnectDelay);
        } else if (reconnectCountRef.current >= reconnectAttempts) {
          setConnectionStatus('error');
          setError('Failed to reconnect after multiple attempts');
          addMessage({
            type: 'error',
            content: 'Connection lost and failed to reconnect',
            level: 'error'
          });
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        setError('WebSocket connection error');
        setConnectionStatus('error');
        onError?.(error);
      };

    } catch (err) {
      console.error('Failed to create WebSocket connection:', err);
      setError('Failed to create WebSocket connection');
      setIsConnecting(false);
      setConnectionStatus('error');
      onError?.(err);
    }
  }, [reconnectAttempts, reconnectDelay, onConnectionChange, onError, onMessage, addMessage, clearError]);

  /**
   * Disconnect WebSocket
   */
  const disconnect = useCallback(() => {
    isManualDisconnectRef.current = true;
    
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.close(1000, 'Manual disconnect');
      wsRef.current = null;
    }

    setIsConnected(false);
    setIsConnecting(false);
    setConnectionStatus('disconnected');
    setSessionInfo(null);
    setClientInfo(null);
    reconnectCountRef.current = 0;
  }, []);

  /**
   * Connect to a session
   */
  const connectToSession = useCallback(async (sessionId, role = 'technician') => {
    try {
      setError(null);
      
      // Validate session ID format
      if (!RemoteAccessService.validateSessionIdFormat(sessionId)) {
        throw new Error('Invalid session ID format');
      }

      // Validate session with server
      const response = await RemoteAccessService.validateSessionId(sessionId);
      
      if (response.data.valid) {
        setSessionId(sessionId);
        isManualDisconnectRef.current = false;
        connectWebSocket(sessionId, role);
        
        addMessage({
          type: 'connection',
          content: `Connecting to session ${sessionId}...`,
          level: 'info'
        });
      } else {
        throw new Error(response.data.message || 'Invalid session ID');
      }
    } catch (err) {
      console.error('Failed to connect to session:', err);
      const errorMessage = err.response?.data?.message || err.message || 'Failed to connect to session';
      setError(errorMessage);
      addMessage({
        type: 'error',
        content: errorMessage,
        level: 'error'
      });
      onError?.(err);
    }
  }, [connectWebSocket, addMessage, onError]);

  /**
   * Send message through WebSocket
   */
  const sendMessage = useCallback((message) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
      return true;
    } else {
      setError('WebSocket is not connected');
      return false;
    }
  }, []);

  /**
   * Request privilege escalation
   */
  const requestPrivilegeEscalation = useCallback(async (request) => {
    try {
      if (!sessionId) {
        throw new Error('No active session');
      }

      const response = await RemoteAccessService.requestPrivilegeEscalation(sessionId, request);
      
      addMessage({
        type: 'privilege_request',
        content: `Privilege escalation requested: ${request.type}`,
        level: 'info'
      });

      return response.data;
    } catch (err) {
      console.error('Failed to request privilege escalation:', err);
      const errorMessage = err.response?.data?.message || err.message || 'Failed to request privilege escalation';
      setError(errorMessage);
      addMessage({
        type: 'error',
        content: errorMessage,
        level: 'error'
      });
      throw err;
    }
  }, [sessionId, addMessage]);

  /**
   * Send control command
   */
  const sendControlCommand = useCallback(async (command) => {
    try {
      if (!sessionId) {
        throw new Error('No active session');
      }

      const response = await RemoteAccessService.sendControlCommand(sessionId, command);
      
      addMessage({
        type: 'control_command',
        content: `Control command sent: ${command.type}`,
        level: 'info'
      });

      return response.data;
    } catch (err) {
      console.error('Failed to send control command:', err);
      const errorMessage = err.response?.data?.message || err.message || 'Failed to send control command';
      setError(errorMessage);
      addMessage({
        type: 'error',
        content: errorMessage,
        level: 'error'
      });
      throw err;
    }
  }, [sessionId, addMessage]);

  /**
   * Terminate session
   */
  const terminateSession = useCallback(async (reason = 'Terminated by technician') => {
    try {
      if (!sessionId) {
        throw new Error('No active session');
      }

      await RemoteAccessService.terminateSession(sessionId, reason);
      
      addMessage({
        type: 'session_terminated',
        content: `Session terminated: ${reason}`,
        level: 'info'
      });

      disconnect();
    } catch (err) {
      console.error('Failed to terminate session:', err);
      const errorMessage = err.response?.data?.message || err.message || 'Failed to terminate session';
      setError(errorMessage);
      addMessage({
        type: 'error',
        content: errorMessage,
        level: 'error'
      });
    }
  }, [sessionId, addMessage, disconnect]);

  /**
   * Load active sessions
   */
  const loadActiveSessions = useCallback(async () => {
    try {
      const response = await RemoteAccessService.getActiveSessions();
      setActiveSessions(response.data.sessions || []);
    } catch (err) {
      console.error('Failed to load active sessions:', err);
      setError('Failed to load active sessions');
    }
  }, []);

  /**
   * Load session statistics
   */
  const loadSessionStats = useCallback(async (sessionId = null) => {
    try {
      const response = await RemoteAccessService.getSessionStats(sessionId);
      setSessionStats(response.data);
    } catch (err) {
      console.error('Failed to load session stats:', err);
      setError('Failed to load session statistics');
    }
  }, []);

  // Auto-connect effect
  useEffect(() => {
    if (autoConnect && sessionId && !isConnected && !isConnecting) {
      connectToSession(sessionId);
    }
  }, [autoConnect, sessionId, isConnected, isConnecting, connectToSession]);

  // Cleanup effect
  useEffect(() => {
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  return {
    // State
    sessionId,
    isConnected,
    isConnecting,
    connectionStatus,
    sessionInfo,
    clientInfo,
    error,
    messages,
    activeSessions,
    sessionStats,
    privilegeRequests,

    // Actions
    connectToSession,
    disconnect,
    sendMessage,
    requestPrivilegeEscalation,
    sendControlCommand,
    terminateSession,
    loadActiveSessions,
    loadSessionStats,
    clearError,
    clearMessages,
    addMessage,

    // Utilities
    setSessionId,
  };
};

export default useRemoteAccess;