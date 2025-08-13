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

export class FileTransferService {
  /**
   * Get all file transfers for a session
   * @param {string} sessionId - The session ID
   * @returns {Promise} API response
   */
  static async getTransfers(sessionId) {
    try {
      const response = await apiClient.get('/filetransfer/transfers', {
        params: { session_id: sessionId }
      });
      return response;
    } catch (error) {
      console.error('Failed to get transfers:', error);
      throw error;
    }
  }

  /**
   * Get a specific file transfer by ID
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async getTransfer(transferId) {
    try {
      const response = await apiClient.get(`/filetransfer/transfers/${transferId}`);
      return response;
    } catch (error) {
      console.error('Failed to get transfer:', error);
      throw error;
    }
  }

  /**
   * Upload a file
   * @param {FormData} formData - Form data containing the file and metadata
   * @returns {Promise} API response
   */
  static async uploadFile(formData) {
    try {
      const response = await apiClient.post('/filetransfer/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        timeout: 300000, // 5 minutes for file uploads
        onUploadProgress: (progressEvent) => {
          const percentCompleted = Math.round(
            (progressEvent.loaded * 100) / progressEvent.total
          );
          console.log(`Upload progress: ${percentCompleted}%`);
        },
      });
      return response;
    } catch (error) {
      console.error('Failed to upload file:', error);
      throw error;
    }
  }

  /**
   * Request a file download
   * @param {Object} downloadRequest - Download request data
   * @param {string} downloadRequest.filename - Name of the file to download
   * @param {string} downloadRequest.session_id - Session ID
   * @param {string} downloadRequest.technician - Technician name
   * @returns {Promise} API response
   */
  static async requestDownload(downloadRequest) {
    try {
      const response = await apiClient.post('/filetransfer/download/request', downloadRequest);
      return response;
    } catch (error) {
      console.error('Failed to request download:', error);
      throw error;
    }
  }

  /**
   * Download a completed file
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response with file data
   */
  static async downloadFile(transferId) {
    try {
      const response = await apiClient.get(`/filetransfer/download/${transferId}`, {
        responseType: 'blob',
        timeout: 300000, // 5 minutes for file downloads
        onDownloadProgress: (progressEvent) => {
          if (progressEvent.lengthComputable) {
            const percentCompleted = Math.round(
              (progressEvent.loaded * 100) / progressEvent.total
            );
            console.log(`Download progress: ${percentCompleted}%`);
          }
        },
      });
      return response;
    } catch (error) {
      console.error('Failed to download file:', error);
      throw error;
    }
  }

  /**
   * Approve or reject a file transfer
   * @param {string} transferId - The transfer ID
   * @param {boolean} approved - Whether to approve the transfer
   * @param {string} message - Optional message
   * @returns {Promise} API response
   */
  static async approveTransfer(transferId, approved, message = '') {
    try {
      const response = await apiClient.post(`/filetransfer/transfers/${transferId}/approve`, {
        approved,
        message,
      });
      return response;
    } catch (error) {
      console.error('Failed to approve transfer:', error);
      throw error;
    }
  }

  /**
   * Pause a file transfer
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async pauseTransfer(transferId) {
    try {
      const response = await apiClient.post(`/filetransfer/transfers/${transferId}/pause`);
      return response;
    } catch (error) {
      console.error('Failed to pause transfer:', error);
      throw error;
    }
  }

  /**
   * Resume a file transfer
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async resumeTransfer(transferId) {
    try {
      const response = await apiClient.post(`/filetransfer/transfers/${transferId}/resume`);
      return response;
    } catch (error) {
      console.error('Failed to resume transfer:', error);
      throw error;
    }
  }

  /**
   * Cancel a file transfer
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async cancelTransfer(transferId) {
    try {
      const response = await apiClient.post(`/filetransfer/transfers/${transferId}/cancel`);
      return response;
    } catch (error) {
      console.error('Failed to cancel transfer:', error);
      throw error;
    }
  }

  /**
   * Delete a file transfer record
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async deleteTransfer(transferId) {
    try {
      const response = await apiClient.delete(`/filetransfer/transfers/${transferId}`);
      return response;
    } catch (error) {
      console.error('Failed to delete transfer:', error);
      throw error;
    }
  }

  /**
   * Get transfer statistics
   * @param {string} sessionId - The session ID (optional)
   * @returns {Promise} API response
   */
  static async getStats(sessionId = null) {
    try {
      const params = sessionId ? { session_id: sessionId } : {};
      const response = await apiClient.get('/filetransfer/stats', { params });
      return response;
    } catch (error) {
      console.error('Failed to get stats:', error);
      throw error;
    }
  }

  /**
   * Get file transfer settings
   * @returns {Promise} API response
   */
  static async getSettings() {
    try {
      const response = await apiClient.get('/filetransfer/settings');
      return response;
    } catch (error) {
      console.error('Failed to get settings:', error);
      throw error;
    }
  }

  /**
   * Update file transfer settings
   * @param {Object} settings - Settings object
   * @returns {Promise} API response
   */
  static async updateSettings(settings) {
    try {
      const response = await apiClient.put('/filetransfer/settings', settings);
      return response;
    } catch (error) {
      console.error('Failed to update settings:', error);
      throw error;
    }
  }

  /**
   * Get transfer progress
   * @param {string} transferId - The transfer ID
   * @returns {Promise} API response
   */
  static async getProgress(transferId) {
    try {
      const response = await apiClient.get(`/filetransfer/transfers/${transferId}/progress`);
      return response;
    } catch (error) {
      console.error('Failed to get progress:', error);
      throw error;
    }
  }

  /**
   * Get transfer history
   * @param {Object} filters - Filter options
   * @param {string} filters.sessionId - Session ID filter
   * @param {string} filters.status - Status filter
   * @param {string} filters.type - Type filter (upload/download)
   * @param {Date} filters.startDate - Start date filter
   * @param {Date} filters.endDate - End date filter
   * @param {number} filters.page - Page number
   * @param {number} filters.limit - Items per page
   * @returns {Promise} API response
   */
  static async getHistory(filters = {}) {
    try {
      const params = {};
      
      if (filters.sessionId) params.session_id = filters.sessionId;
      if (filters.status) params.status = filters.status;
      if (filters.type) params.type = filters.type;
      if (filters.startDate) params.start_date = filters.startDate.toISOString();
      if (filters.endDate) params.end_date = filters.endDate.toISOString();
      if (filters.page) params.page = filters.page;
      if (filters.limit) params.limit = filters.limit;
      
      const response = await apiClient.get('/filetransfer/history', { params });
      return response;
    } catch (error) {
      console.error('Failed to get history:', error);
      throw error;
    }
  }

  /**
   * Validate a file before upload
   * @param {File} file - The file to validate
   * @returns {Promise} API response
   */
  static async validateFile(file) {
    try {
      const formData = new FormData();
      formData.append('file', file);
      
      const response = await apiClient.post('/filetransfer/validate', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      return response;
    } catch (error) {
      console.error('Failed to validate file:', error);
      throw error;
    }
  }

  /**
   * Get available files for download
   * @param {string} sessionId - The session ID
   * @returns {Promise} API response
   */
  static async getAvailableFiles(sessionId) {
    try {
      const response = await apiClient.get('/filetransfer/files', {
        params: { session_id: sessionId }
      });
      return response;
    } catch (error) {
      console.error('Failed to get available files:', error);
      throw error;
    }
  }

  /**
   * Test connection to file transfer service
   * @returns {Promise} API response
   */
  static async testConnection() {
    try {
      const response = await apiClient.get('/filetransfer/health');
      return response;
    } catch (error) {
      console.error('Failed to test connection:', error);
      throw error;
    }
  }

  /**
   * Get WebSocket connection URL
   * @param {string} sessionId - The session ID
   * @param {string} role - User role (client/technician)
   * @returns {string} WebSocket URL
   */
  static getWebSocketUrl(sessionId, role = 'technician') {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsHost = process.env.REACT_APP_WS_HOST || window.location.host.replace(':3000', ':8080');
    return `${wsProtocol}//${wsHost}/ws/filetransfer?session_id=${sessionId}&role=${role}`;
  }

  /**
   * Format file size in human readable format
   * @param {number} bytes - Size in bytes
   * @returns {string} Formatted size
   */
  static formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  /**
   * Format transfer speed
   * @param {number} bytesPerSecond - Speed in bytes per second
   * @returns {string} Formatted speed
   */
  static formatSpeed(bytesPerSecond) {
    if (bytesPerSecond === 0) return '0 B/s';
    
    const k = 1024;
    const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
    const i = Math.floor(Math.log(bytesPerSecond) / Math.log(k));
    
    return parseFloat((bytesPerSecond / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  /**
   * Format duration in human readable format
   * @param {number} seconds - Duration in seconds
   * @returns {string} Formatted duration
   */
  static formatDuration(seconds) {
    if (seconds === 0) return '0s';
    
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    if (hours > 0) {
      return `${hours}h ${minutes}m ${secs}s`;
    } else if (minutes > 0) {
      return `${minutes}m ${secs}s`;
    } else {
      return `${secs}s`;
    }
  }

  /**
   * Calculate ETA based on progress and speed
   * @param {number} totalBytes - Total file size in bytes
   * @param {number} transferredBytes - Bytes already transferred
   * @param {number} speed - Current speed in bytes per second
   * @returns {number} ETA in seconds
   */
  static calculateETA(totalBytes, transferredBytes, speed) {
    if (speed === 0 || transferredBytes >= totalBytes) {
      return 0;
    }
    
    const remainingBytes = totalBytes - transferredBytes;
    return Math.ceil(remainingBytes / speed);
  }

  /**
   * Validate file extension
   * @param {string} filename - The filename
   * @param {Array} allowedExtensions - Array of allowed extensions
   * @returns {boolean} Whether the file extension is allowed
   */
  static validateFileExtension(filename, allowedExtensions) {
    if (!filename || !allowedExtensions || allowedExtensions.length === 0) {
      return false;
    }
    
    const extension = '.' + filename.split('.').pop().toLowerCase();
    return allowedExtensions.includes(extension);
  }

  /**
   * Validate file size
   * @param {number} fileSize - File size in bytes
   * @param {number} maxSize - Maximum allowed size in bytes
   * @returns {boolean} Whether the file size is within limits
   */
  static validateFileSize(fileSize, maxSize) {
    return fileSize <= maxSize;
  }

  /**
   * Generate a unique transfer ID
   * @returns {string} Unique transfer ID
   */
  static generateTransferId() {
    return 'transfer_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
  }

  /**
   * Check if transfer status is active
   * @param {string} status - Transfer status
   * @returns {boolean} Whether the transfer is active
   */
  static isActiveStatus(status) {
    return ['pending', 'approved', 'in_progress', 'paused'].includes(status);
  }

  /**
   * Check if transfer status is final
   * @param {string} status - Transfer status
   * @returns {boolean} Whether the transfer is in a final state
   */
  static isFinalStatus(status) {
    return ['completed', 'failed', 'cancelled', 'rejected'].includes(status);
  }

  /**
   * Get status color for UI display
   * @param {string} status - Transfer status
   * @returns {string} Color name for Material-UI
   */
  static getStatusColor(status) {
    const colors = {
      pending: 'warning',
      approved: 'info',
      rejected: 'error',
      in_progress: 'primary',
      paused: 'default',
      completed: 'success',
      failed: 'error',
      cancelled: 'default'
    };
    
    return colors[status] || 'default';
  }
}

export default FileTransferService;