import axios from 'axios';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:3001';

// Create axios instance with default config
const apiClient = axios.create({
  baseURL: `${API_BASE_URL}/api`,
  timeout: 30000, // 30 seconds
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor for auth tokens
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('authToken');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    if (error.response?.status === 401) {
      // Handle unauthorized access
      localStorage.removeItem('authToken');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export class RemoteAccessService {
  /**
   * Generate a new session ID for remote access
   * @returns {Promise} API response with session ID
   */
  static async generateSessionId() {
    try {
      const response = await apiClient.post('/remote-access/session/generate');
      return response;
    } catch (error) {
      console.error('Failed to generate session ID:', error);
      throw error;
    }
  }

  /**
   * Validate a session ID
   * @param {string} sessionId - The session ID to validate
   * @returns {Promise} API response
   */
  static async validateSessionId(sessionId) {
    try {
      const response = await apiClient.get(`/remote-access/session/${sessionId}/validate`);
      return response;
    } catch (error) {
      console.error('Failed to validate session ID:', error);
      throw error;
    }
  }

  /**
   * Get session information
   * @param {string} sessionId - The session ID
   * @returns {Promise} API response
   */
  static async getSessionInfo(sessionId) {
    try {
      const response = await apiClient.get(`/remote-access/session/${sessionId}`);
      return response;
    } catch (error) {
      console.error('Failed to get session info:', error);
      throw error;
    }
  }

  /**
   * Register technician for a session
   * @param {string} sessionId - The session ID
   * @param {Object} technicianInfo - Technician information
   * @returns {Promise} API response
   */
  static async registerTechnician(sessionId, technicianInfo) {
    try {
      const response = await apiClient.post(`/remote-access/session/${sessionId}/technician`, {
        technician: technicianInfo
      });
      return response;
    } catch (error) {
      console.error('Failed to register technician:', error);
      throw error;
    }
  }

  /**
   * Request privilege escalation
   * @param {string} sessionId - The session ID
   * @param {Object} request - Privilege escalation request
   * @returns {Promise} API response
   */
  static async requestPrivilegeEscalation(sessionId, request) {
    try {
      const response = await apiClient.post(`/remote-access/session/${sessionId}/privilege-escalation`, request);
      return response;
    } catch (error) {
      console.error('Failed to request privilege escalation:', error);
      throw error;
    }
  }

  /**
   * Terminate a session
   * @param {string} sessionId - The session ID
   * @param {string} reason - Termination reason
   * @returns {Promise} API response
   */
  static async terminateSession(sessionId, reason = 'Terminated by technician') {
    try {
      const response = await apiClient.post(`/remote-access/session/${sessionId}/terminate`, {
        reason
      });
      return response;
    } catch (error) {
      console.error('Failed to terminate session:', error);
      throw error;
    }
  }

  /**
   * Get active sessions
   * @returns {Promise} API response
   */
  static async getActiveSessions() {
    try {
      const response = await apiClient.get('/remote-access/sessions/active');
      return response;
    } catch (error) {
      console.error('Failed to get active sessions:', error);
      throw error;
    }
  }

  /**
   * Get session history
   * @param {Object} filters - Filter options
   * @returns {Promise} API response
   */
  static async getSessionHistory(filters = {}) {
    try {
      const response = await apiClient.get('/remote-access/sessions/history', {
        params: filters
      });
      return response;
    } catch (error) {
      console.error('Failed to get session history:', error);
      throw error;
    }
  }

  /**
   * Get session statistics
   * @param {string} sessionId - The session ID (optional)
   * @returns {Promise} API response
   */
  static async getSessionStats(sessionId = null) {
    try {
      const params = sessionId ? { session_id: sessionId } : {};
      const response = await apiClient.get('/remote-access/stats', { params });
      return response;
    } catch (error) {
      console.error('Failed to get session stats:', error);
      throw error;
    }
  }

  /**
   * Update session settings
   * @param {string} sessionId - The session ID
   * @param {Object} settings - Session settings
   * @returns {Promise} API response
   */
  static async updateSessionSettings(sessionId, settings) {
    try {
      const response = await apiClient.put(`/remote-access/session/${sessionId}/settings`, settings);
      return response;
    } catch (error) {
      console.error('Failed to update session settings:', error);
      throw error;
    }
  }

  /**
   * Send control command to client
   * @param {string} sessionId - The session ID
   * @param {Object} command - Control command
   * @returns {Promise} API response
   */
  static async sendControlCommand(sessionId, command) {
    try {
      const response = await apiClient.post(`/remote-access/session/${sessionId}/control`, command);
      return response;
    } catch (error) {
      console.error('Failed to send control command:', error);
      throw error;
    }
  }

  /**
   * Get WebSocket connection URL for remote access
   * @param {string} sessionId - The session ID
   * @param {string} role - User role (client/technician)
   * @returns {string} WebSocket URL
   */
  static getWebSocketUrl(sessionId, role = 'technician') {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsHost = process.env.REACT_APP_WS_HOST || window.location.host.replace(':3000', ':8080');
    return `${wsProtocol}//${wsHost}/ws/remote-access?session_id=${sessionId}&role=${role}`;
  }

  /**
   * Validate session ID format
   * @param {string} sessionId - The session ID to validate
   * @returns {boolean} True if valid format
   */
  static validateSessionIdFormat(sessionId) {
    // Session ID should be 6-12 alphanumeric characters
    return /^[A-Z0-9]{6,12}$/.test(sessionId);
  }

  /**
   * Generate a random session ID
   * @param {number} length - Length of the session ID (default: 8)
   * @returns {string} Generated session ID
   */
  static generateRandomSessionId(length = 8) {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    let result = '';
    for (let i = 0; i < length; i++) {
      result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
  }

  /**
   * Format session duration
   * @param {number} startTime - Start timestamp
   * @param {number} endTime - End timestamp (optional, defaults to now)
   * @returns {string} Formatted duration
   */
  static formatSessionDuration(startTime, endTime = Date.now()) {
    const duration = endTime - startTime;
    const hours = Math.floor(duration / (1000 * 60 * 60));
    const minutes = Math.floor((duration % (1000 * 60 * 60)) / (1000 * 60));
    const seconds = Math.floor((duration % (1000 * 60)) / 1000);
    
    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    } else {
      return `${seconds}s`;
    }
  }

  /**
   * Get session status color for UI
   * @param {string} status - Session status
   * @returns {string} Color name for Material-UI
   */
  static getSessionStatusColor(status) {
    switch (status?.toLowerCase()) {
      case 'active':
      case 'connected':
        return 'success';
      case 'pending':
      case 'waiting':
        return 'warning';
      case 'terminated':
      case 'failed':
      case 'error':
        return 'error';
      case 'disconnected':
      default:
        return 'default';
    }
  }
}

export default RemoteAccessService;