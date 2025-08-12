# Core Components

## Cross-Platform Client

**Function**: The software installed on the client's machine. Its responsibility is to generate the session ID, capture the screen, receive and apply commands from the technician (mouse and keyboard), and manage file transfers.

**Technology**: Qt/C++ was chosen to ensure high performance, low resource consumption, and native compatibility with Windows, Linux, and macOS.

## Connection Server

**Function**: The heart of the system. It manages all sessions, handles authentication, routes screen data and commands between the client and the technician, and handles end-to-end encryption.

**Technology**: Go (Golang) was selected for its high performance in concurrency (with goroutines), making it ideal for efficiently managing multiple real-time connections via WebSockets.

## Technician Portal

**Function**: A web-based interface that allows the support technician to start, manage, and end sessions. This is where the technician enters the client's ID, views the remote screen, and interacts with the machine.

**Technology**: React for the frontend (the user interface) and Node.js for the portal's backend, offering fast development and a fluid experience for the technician.