# Onlidesk - Remote Desktop Solution

Onlidesk is a secure, cross-platform remote desktop solution designed for technical support and remote assistance. The system consists of three main components that work together to provide seamless remote access capabilities.

## 🏗️ Architecture Overview

The Onlidesk architecture is modular, scalable, and secure, divided into three main components:

### 🖥️ Cross-Platform Client
- **Technology**: Qt/C++
- **Purpose**: Software installed on client machines
- **Features**: Session ID generation, screen capture, command processing, file transfers
- **Compatibility**: Windows, Linux, macOS

### 🌐 Connection Server
- **Technology**: Go (Golang)
- **Purpose**: Central hub for session management
- **Features**: Authentication, data routing, end-to-end encryption
- **Performance**: High concurrency with goroutines and WebSockets

### 👨‍💻 Technician Portal
- **Technology**: React + Node.js
- **Purpose**: Web-based interface for support technicians
- **Features**: Session management, remote screen viewing, machine interaction

## 📁 Project Structure

```
.
├── src/
│   ├── client/          # Cross-platform client (Qt/C++)
│   ├── server/          # Connection server (Go)
│   └── portal/          # Technician portal (React/Node.js)
├── test/
│   ├── unit/            # Unit tests
│   ├── integration/     # Integration tests
│   └── e2e/             # End-to-end tests
├── docs/                # Documentation
├── build/               # Build artifacts
├── tools/               # Development tools
├── scripts/             # Build and deployment scripts
└── deploy/              # Deployment configurations
```

## 🚀 Getting Started

### Prerequisites
- Qt 6.x (for client development)
- Go 1.21+ (for server development)
- Node.js 18+ (for portal development)
- Docker (for deployment)

### Development Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd Onlidesk
   ```

2. **Set up each component**
   - See `src/client/README.md` for client setup
   - See `src/server/README.md` for server setup
   - See `src/portal/README.md` for portal setup

## 🔒 Security Features

- End-to-end encryption (TLS/SSL)
- Session-based authentication
- Secure WebSocket connections
- No permanent access or backdoors

## 🌍 Cross-Platform Support

- **Windows**: Native Qt application
- **Linux**: Native Qt application
- **macOS**: Native Qt application
- **Web Portal**: Any modern browser

## 📋 MVP Features

- ✅ Remote desktop access
- ✅ Secure file transfer
- ✅ Unattended access capability
- ✅ Multi-OS compatibility
- ✅ Session management
- ✅ Real-time screen sharing

## 🛠️ Development Workflow

This project follows the BMad Method for structured development:

1. **Planning Phase** (Completed in Gemini Gem)
   - Project Brief ✅
   - Product Requirements Document (PRD) ✅
   - Technical Architecture ✅

2. **Development Phase** (Current - IDE)
   - Document sharding ✅
   - User story creation
   - Iterative development
   - Testing and QA

## 📚 Documentation

- [Project Brief](docs/project-brief.md)
- [Product Requirements](docs/prd.md)
- [Technical Architecture](docs/architecture.md)
- [API Documentation](docs/api/)
- [Deployment Guide](docs/deployment.md)

## 🤝 Contributing

Please read our contributing guidelines and code of conduct before submitting pull requests.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🆘 Support

For support and questions, please open an issue in the GitHub repository.