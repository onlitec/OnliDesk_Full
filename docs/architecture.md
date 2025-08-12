Com certeza. Aqui está o arquivo architecture.md do projeto Onlidesk, traduzido para o inglês.

Você pode copiar este texto para salvar o arquivo.

Onlidesk: Technical Architecture
1. Architecture Overview
The Onlidesk architecture is designed to be modular, scalable, and secure. The system is divided into three main components that communicate through a secure protocol. The workflow is centered on the server, which acts as a central hub for all sessions.

2. Core Components
Cross-Platform Client

Function: The software installed on the client's machine. Its responsibility is to generate the session ID, capture the screen, receive and apply commands from the technician (mouse and keyboard), and manage file transfers.

Technology: Qt/C++ was chosen to ensure high performance, low resource consumption, and native compatibility with Windows, Linux, and macOS.

Connection Server

Function: The heart of the system. It manages all sessions, handles authentication, routes screen data and commands between the client and the technician, and handles end-to-end encryption.

Technology: Go (Golang) was selected for its high performance in concurrency (with goroutines), making it ideal for efficiently managing multiple real-time connections via WebSockets.

Technician Portal

Function: A web-based interface that allows the support technician to start, manage, and end sessions. This is where the technician enters the client's ID, views the remote screen, and interacts with the machine.

Technology: React for the frontend (the user interface) and Node.js for the portal's backend, offering fast development and a fluid experience for the technician.

3. Data Flow and Communication
The communication flow is secure and follows a request-response model with real-time capabilities:

The Client connects to the Connection Server and generates a unique session ID.

The Technician enters this ID into the Technician Portal.

The Portal sends an access request to the Server.

The Server sends a real-time notification (via WebSocket) to the Client.

The Client accepts the request, and a secure, encrypted connection is established.

The Server acts as a proxy, routing the screen stream from the client to the technician portal and the commands back to the client, all in real time.

4. Technologies and Dependencies
Backend (Connection Server): Go, WebSockets, TLS/SSL.

Cross-Platform Client: C++, Qt Framework, WebSockets, TLS/SSL.

Technician Portal: React, Node.js, WebSockets, REST API.

Database: A relational database for storing session logs and technician data.

Deployment: The application will be containerized with Docker and deployed to a VPS running Ubuntu 24.