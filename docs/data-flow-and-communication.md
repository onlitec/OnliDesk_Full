### Data Flow and Communication

The communication flow is secure and follows a request-response model with real-time capabilities.

1.  The **Client** connects to the **Connection Server** and generates a unique session ID.
2.  The **Technician** enters this ID into the **Technician Portal**.
3.  The **Portal** sends an access request to the **Server**.
4.  The **Server** sends a real-time notification (via **WebSocket**) to the **Client**.
5.  The **Client** accepts the request, and a secure, encrypted connection is established.
6.  The **Server** acts as a proxy, routing the screen stream from the client to the technician portal and the commands back to the client, all in real time.