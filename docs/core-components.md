### Core Components

#### Cross-Platform Client
* **Function:** The software installed on the client's machine. It is responsible for generating the session ID, capturing the screen, receiving and applying commands from the technician (mouse and keyboard), and managing file transfers.
* **Technology:** **Qt/C++** was chosen to ensure high performance, low resource consumption, and native compatibility with Windows, Linux, and macOS.

#### Connection Server
* **Function:** The heart of the system. It manages all sessions, handles authentication, routes screen data and commands between the client and the technician, and handles end-to-end encryption.
* **Technology:** **Go (Golang)** was selected for its high performance in concurrency, making it ideal for efficiently managing multiple real-time connections via WebSockets.

#### Technician Portal
* **Function:** A web-based interface that allows the support technician to start, manage, and end sessions. This is where the technician views the remote screen and interacts with the client's machine.
* **Technology:** **React** for the frontend and **Node.js** for the portal's backend, offering rapid development and a fluid user experience.