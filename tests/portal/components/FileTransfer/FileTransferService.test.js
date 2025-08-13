import { FileTransferService } from '../../../../src/portal/frontend/src/services/FileTransferService';
import axios from 'axios';

// Mock axios
jest.mock('axios');
const mockedAxios = axios;

describe('FileTransferService', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.clear();
  });

  describe('getTransfers', () => {
    it('should fetch transfers for a session', async () => {
      const mockResponse = {
        data: [
          { id: '1', filename: 'test.txt', status: 'completed' },
          { id: '2', filename: 'doc.pdf', status: 'in_progress' }
        ]
      };
      mockedAxios.get.mockResolvedValue(mockResponse);

      const result = await FileTransferService.getTransfers('session123');

      expect(mockedAxios.get).toHaveBeenCalledWith('/filetransfer/session123');
      expect(result.data).toEqual(mockResponse.data);
    });

    it('should handle errors when fetching transfers', async () => {
      const mockError = new Error('Network error');
      mockedAxios.get.mockRejectedValue(mockError);

      await expect(FileTransferService.getTransfers('session123'))
        .rejects.toThrow('Network error');
    });
  });

  describe('uploadFile', () => {
    it('should upload a file successfully', async () => {
      const mockFormData = new FormData();
      mockFormData.append('file', new File(['content'], 'test.txt'));
      mockFormData.append('sessionId', 'session123');

      const mockResponse = {
        data: { id: 'transfer123', status: 'pending' }
      };
      mockedAxios.post.mockResolvedValue(mockResponse);

      const result = await FileTransferService.uploadFile(mockFormData);

      expect(mockedAxios.post).toHaveBeenCalledWith('/filetransfer/upload', mockFormData, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 300000
      });
      expect(result.data).toEqual(mockResponse.data);
    });
  });

  describe('validateFile', () => {
    it('should validate file size within limits', async () => {
      const file = new File(['content'], 'test.txt', { type: 'text/plain' });
      Object.defineProperty(file, 'size', { value: 1024 * 1024 }); // 1MB

      const result = await FileTransferService.validateFile(file);

      expect(result.valid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });

    it('should reject files that are too large', async () => {
      const file = new File(['content'], 'large.txt', { type: 'text/plain' });
      Object.defineProperty(file, 'size', { value: 200 * 1024 * 1024 }); // 200MB

      const result = await FileTransferService.validateFile(file);

      expect(result.valid).toBe(false);
      expect(result.errors).toContain('File size exceeds maximum allowed size');
    });

    it('should reject files with invalid extensions', async () => {
      const file = new File(['content'], 'malicious.exe', { type: 'application/x-msdownload' });

      const result = await FileTransferService.validateFile(file);

      expect(result.valid).toBe(false);
      expect(result.errors).toContain('File type not allowed');
    });
  });

  describe('formatFileSize', () => {
    it('should format bytes correctly', () => {
      expect(FileTransferService.formatFileSize(1024)).toBe('1.00 KB');
      expect(FileTransferService.formatFileSize(1048576)).toBe('1.00 MB');
      expect(FileTransferService.formatFileSize(1073741824)).toBe('1.00 GB');
    });
  });

  describe('formatSpeed', () => {
    it('should format transfer speed correctly', () => {
      expect(FileTransferService.formatSpeed(1024)).toBe('1.00 KB/s');
      expect(FileTransferService.formatSpeed(1048576)).toBe('1.00 MB/s');
    });
  });

  describe('calculateETA', () => {
    it('should calculate estimated time accurately', () => {
      const totalBytes = 1000000;
      const transferredBytes = 250000;
      const speed = 50000; // 50KB/s

      const eta = FileTransferService.calculateETA(totalBytes, transferredBytes, speed);

      expect(eta).toBe(15); // (750000 remaining / 50000 speed) = 15 seconds
    });

    it('should return 0 for completed transfers', () => {
      const eta = FileTransferService.calculateETA(1000, 1000, 100);
      expect(eta).toBe(0);
    });
  });

  describe('WebSocket URL generation', () => {
    it('should generate correct WebSocket URL for technician', () => {
      const url = FileTransferService.getWebSocketUrl('session123', 'technician');
      expect(url).toContain('ws://localhost:3001/ws/filetransfer/session123');
      expect(url).toContain('role=technician');
    });

    it('should generate correct WebSocket URL for client', () => {
      const url = FileTransferService.getWebSocketUrl('session123', 'client');
      expect(url).toContain('role=client');
    });
  });

  describe('transfer control operations', () => {
    it('should pause transfer successfully', async () => {
      const mockResponse = { data: { status: 'paused' } };
      mockedAxios.post.mockResolvedValue(mockResponse);

      const result = await FileTransferService.pauseTransfer('transfer123');

      expect(mockedAxios.post).toHaveBeenCalledWith('/filetransfer/transfer123/pause');
      expect(result.data.status).toBe('paused');
    });

    it('should resume transfer successfully', async () => {
      const mockResponse = { data: { status: 'in_progress' } };
      mockedAxios.post.mockResolvedValue(mockResponse);

      const result = await FileTransferService.resumeTransfer('transfer123');

      expect(mockedAxios.post).toHaveBeenCalledWith('/filetransfer/transfer123/resume');
      expect(result.data.status).toBe('in_progress');
    });

    it('should cancel transfer successfully', async () => {
      const mockResponse = { data: { status: 'cancelled' } };
      mockedAxios.post.mockResolvedValue(mockResponse);

      const result = await FileTransferService.cancelTransfer('transfer123');

      expect(mockedAxios.post).toHaveBeenCalledWith('/filetransfer/transfer123/cancel');
      expect(result.data.status).toBe('cancelled');
    });
  });
});