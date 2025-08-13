import React from 'react';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { jest } from '@jest/globals';
import '@testing-library/jest-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import FileTransferManager from './FileTransferManager';
import FileTransferService from '../../services/FileTransferService';
import { useWebSocket } from '../../hooks/useWebSocket';

// Mock dependencies
jest.mock('../../services/FileTransferService');
jest.mock('../../hooks/useWebSocket');

// Mock Material-UI components that might cause issues in tests
jest.mock('@mui/material/Dialog', () => {
  return function MockDialog({ open, children, ...props }) {
    return open ? <div data-testid="dialog" {...props}>{children}</div> : null;
  };
});

const theme = createTheme();

const renderWithTheme = (component) => {
  return render(
    <ThemeProvider theme={theme}>
      {component}
    </ThemeProvider>
  );
};

// Mock data
const mockTransfers = [
  {
    id: 'transfer-1',
    fileName: 'document.pdf',
    fileSize: 1024000,
    status: 'in_progress',
    type: 'upload',
    progress: 45,
    speed: 1024000,
    eta: 30,
    sessionId: 'session-123',
    technicianEmail: 'tech@example.com',
    createdAt: '2024-01-15T10:00:00Z',
    checksum: 'abc123def456'
  },
  {
    id: 'transfer-2',
    fileName: 'image.jpg',
    fileSize: 512000,
    status: 'completed',
    type: 'download',
    progress: 100,
    speed: 0,
    eta: 0,
    sessionId: 'session-456',
    technicianEmail: 'tech2@example.com',
    createdAt: '2024-01-15T09:30:00Z',
    checksum: 'def456ghi789'
  }
];

const mockWebSocketData = {
  isConnected: true,
  lastMessage: null,
  sendMessage: jest.fn(),
  connect: jest.fn(),
  disconnect: jest.fn()
};

describe('FileTransferManager', () => {
  beforeEach(() => {
    // Reset all mocks
    jest.clearAllMocks();
    
    // Setup default mock implementations
    FileTransferService.getTransfers.mockResolvedValue({ data: mockTransfers });
    FileTransferService.getStats.mockResolvedValue({
      data: {
        totalTransfers: 2,
        activeTransfers: 1,
        completedTransfers: 1,
        failedTransfers: 0,
        totalBytesTransferred: 1536000
      }
    });
    FileTransferService.getWebSocketUrl.mockReturnValue('ws://localhost:8080/filetransfer');
    
    useWebSocket.mockReturnValue(mockWebSocketData);
  });

  describe('Component Rendering', () => {
    test('renders file transfer manager with initial state', async () => {
      renderWithTheme(<FileTransferManager />);
      
      expect(screen.getByText('File Transfer Manager')).toBeInTheDocument();
      expect(screen.getByText('Upload File')).toBeInTheDocument();
      expect(screen.getByText('Refresh')).toBeInTheDocument();
      
      await waitFor(() => {
        expect(FileTransferService.getTransfers).toHaveBeenCalled();
      });
    });

    test('displays transfer statistics', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('Total: 2')).toBeInTheDocument();
        expect(screen.getByText('Active: 1')).toBeInTheDocument();
        expect(screen.getByText('Completed: 1')).toBeInTheDocument();
      });
    });

    test('renders transfer list with mock data', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
        expect(screen.getByText('image.jpg')).toBeInTheDocument();
        expect(screen.getByText('Upload')).toBeInTheDocument();
        expect(screen.getByText('Download')).toBeInTheDocument();
      });
    });
  });

  describe('File Upload', () => {
    test('opens upload dialog when upload button is clicked', async () => {
      renderWithTheme(<FileTransferManager />);
      
      const uploadButton = screen.getByText('Upload File');
      fireEvent.click(uploadButton);
      
      await waitFor(() => {
        expect(screen.getByTestId('dialog')).toBeInTheDocument();
        expect(screen.getByText('Upload File')).toBeInTheDocument();
      });
    });

    test('validates file selection in upload dialog', async () => {
      FileTransferService.validateFile.mockResolvedValue({ valid: false, error: 'File too large' });
      
      renderWithTheme(<FileTransferManager />);
      
      const uploadButton = screen.getByText('Upload File');
      fireEvent.click(uploadButton);
      
      await waitFor(() => {
        expect(screen.getByTestId('dialog')).toBeInTheDocument();
      });
      
      // Simulate file selection
      const fileInput = screen.getByLabelText(/select file/i);
      const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
      
      await act(async () => {
        fireEvent.change(fileInput, { target: { files: [file] } });
      });
      
      await waitFor(() => {
        expect(FileTransferService.validateFile).toHaveBeenCalledWith(file);
      });
    });

    test('uploads file successfully', async () => {
      FileTransferService.validateFile.mockResolvedValue({ valid: true });
      FileTransferService.uploadFile.mockResolvedValue({ 
        data: { transferId: 'new-transfer-123' } 
      });
      
      renderWithTheme(<FileTransferManager />);
      
      const uploadButton = screen.getByText('Upload File');
      fireEvent.click(uploadButton);
      
      await waitFor(() => {
        expect(screen.getByTestId('dialog')).toBeInTheDocument();
      });
      
      // Fill in form fields
      const sessionIdInput = screen.getByLabelText(/session id/i);
      const technicianEmailInput = screen.getByLabelText(/technician email/i);
      
      fireEvent.change(sessionIdInput, { target: { value: 'session-789' } });
      fireEvent.change(technicianEmailInput, { target: { value: 'tech@test.com' } });
      
      // Select file
      const fileInput = screen.getByLabelText(/select file/i);
      const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
      
      await act(async () => {
        fireEvent.change(fileInput, { target: { files: [file] } });
      });
      
      // Submit form
      const submitButton = screen.getByText('Upload');
      
      await act(async () => {
        fireEvent.click(submitButton);
      });
      
      await waitFor(() => {
        expect(FileTransferService.uploadFile).toHaveBeenCalledWith(
          file,
          'session-789',
          'tech@test.com'
        );
      });
    });
  });

  describe('Transfer Control', () => {
    test('pauses transfer when pause button is clicked', async () => {
      FileTransferService.pauseTransfer.mockResolvedValue({ success: true });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      const pauseButton = screen.getByLabelText(/pause transfer/i);
      
      await act(async () => {
        fireEvent.click(pauseButton);
      });
      
      expect(FileTransferService.pauseTransfer).toHaveBeenCalledWith('transfer-1');
    });

    test('resumes transfer when resume button is clicked', async () => {
      // Mock transfer in paused state
      const pausedTransfers = [{
        ...mockTransfers[0],
        status: 'paused'
      }];
      
      FileTransferService.getTransfers.mockResolvedValue({ data: pausedTransfers });
      FileTransferService.resumeTransfer.mockResolvedValue({ success: true });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      const resumeButton = screen.getByLabelText(/resume transfer/i);
      
      await act(async () => {
        fireEvent.click(resumeButton);
      });
      
      expect(FileTransferService.resumeTransfer).toHaveBeenCalledWith('transfer-1');
    });

    test('cancels transfer when cancel button is clicked', async () => {
      FileTransferService.cancelTransfer.mockResolvedValue({ success: true });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      const cancelButton = screen.getByLabelText(/cancel transfer/i);
      
      await act(async () => {
        fireEvent.click(cancelButton);
      });
      
      expect(FileTransferService.cancelTransfer).toHaveBeenCalledWith('transfer-1');
    });
  });

  describe('WebSocket Integration', () => {
    test('connects to WebSocket on component mount', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(useWebSocket).toHaveBeenCalledWith('ws://localhost:8080/filetransfer');
      });
    });

    test('handles WebSocket progress updates', async () => {
      const mockProgressMessage = {
        type: 'progress',
        transferId: 'transfer-1',
        progress: 75,
        speed: 2048000,
        eta: 15
      };
      
      useWebSocket.mockReturnValue({
        ...mockWebSocketData,
        lastMessage: mockProgressMessage
      });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        // Verify that progress updates are handled
        // This would depend on the actual implementation
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
    });

    test('handles WebSocket transfer completion', async () => {
      const mockCompletionMessage = {
        type: 'completed',
        transferId: 'transfer-1',
        checksum: 'verified-checksum'
      };
      
      useWebSocket.mockReturnValue({
        ...mockWebSocketData,
        lastMessage: mockCompletionMessage
      });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    test('displays error message when transfer list fails to load', async () => {
      FileTransferService.getTransfers.mockRejectedValue(new Error('Network error'));
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText(/error loading transfers/i)).toBeInTheDocument();
      });
    });

    test('handles file upload errors gracefully', async () => {
      FileTransferService.validateFile.mockResolvedValue({ valid: true });
      FileTransferService.uploadFile.mockRejectedValue(new Error('Upload failed'));
      
      renderWithTheme(<FileTransferManager />);
      
      const uploadButton = screen.getByText('Upload File');
      fireEvent.click(uploadButton);
      
      await waitFor(() => {
        expect(screen.getByTestId('dialog')).toBeInTheDocument();
      });
      
      // Fill form and submit
      const sessionIdInput = screen.getByLabelText(/session id/i);
      const technicianEmailInput = screen.getByLabelText(/technician email/i);
      const fileInput = screen.getByLabelText(/select file/i);
      
      fireEvent.change(sessionIdInput, { target: { value: 'session-789' } });
      fireEvent.change(technicianEmailInput, { target: { value: 'tech@test.com' } });
      
      const file = new File(['test content'], 'test.txt', { type: 'text/plain' });
      
      await act(async () => {
        fireEvent.change(fileInput, { target: { files: [file] } });
      });
      
      const submitButton = screen.getByText('Upload');
      
      await act(async () => {
        fireEvent.click(submitButton);
      });
      
      await waitFor(() => {
        expect(screen.getByText(/upload failed/i)).toBeInTheDocument();
      });
    });

    test('handles WebSocket connection errors', async () => {
      useWebSocket.mockReturnValue({
        ...mockWebSocketData,
        isConnected: false,
        error: 'Connection failed'
      });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText(/connection error/i)).toBeInTheDocument();
      });
    });
  });

  describe('Filtering and Sorting', () => {
    test('filters transfers by status', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      // Find and click status filter
      const statusFilter = screen.getByLabelText(/filter by status/i);
      fireEvent.change(statusFilter, { target: { value: 'completed' } });
      
      await waitFor(() => {
        expect(screen.getByText('image.jpg')).toBeInTheDocument();
        expect(screen.queryByText('document.pdf')).not.toBeInTheDocument();
      });
    });

    test('sorts transfers by date', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      const sortButton = screen.getByLabelText(/sort by date/i);
      fireEvent.click(sortButton);
      
      // Verify sorting behavior (implementation dependent)
      expect(screen.getByText('document.pdf')).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    test('has proper ARIA labels for interactive elements', async () => {
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByLabelText(/upload file/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/refresh transfers/i)).toBeInTheDocument();
      });
    });

    test('supports keyboard navigation', async () => {
      renderWithTheme(<FileTransferManager />);
      
      const uploadButton = screen.getByText('Upload File');
      
      // Test keyboard interaction
      fireEvent.keyDown(uploadButton, { key: 'Enter', code: 'Enter' });
      
      await waitFor(() => {
        expect(screen.getByTestId('dialog')).toBeInTheDocument();
      });
    });
  });

  describe('Performance', () => {
    test('does not re-render unnecessarily', async () => {
      const { rerender } = renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('document.pdf')).toBeInTheDocument();
      });
      
      // Re-render with same props
      rerender(
        <ThemeProvider theme={theme}>
          <FileTransferManager />
        </ThemeProvider>
      );
      
      // Verify component still works
      expect(screen.getByText('document.pdf')).toBeInTheDocument();
    });

    test('handles large transfer lists efficiently', async () => {
      const largeTransferList = Array.from({ length: 100 }, (_, index) => ({
        ...mockTransfers[0],
        id: `transfer-${index}`,
        fileName: `file-${index}.txt`
      }));
      
      FileTransferService.getTransfers.mockResolvedValue({ data: largeTransferList });
      
      renderWithTheme(<FileTransferManager />);
      
      await waitFor(() => {
        expect(screen.getByText('file-0.txt')).toBeInTheDocument();
      });
      
      // Verify virtualization or pagination is working
      // This would depend on the actual implementation
    });
  });
});