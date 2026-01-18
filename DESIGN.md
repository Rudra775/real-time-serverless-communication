# System Design Document: Distributed Real-Time Engine

## 1. Problem Statement
The goal was to engineer a high-throughput, real-time messaging engine capable of broadcasting events to thousands of concurrent clients with minimal latency. The system needed to support:
* **Horizontal Scalability:** Adding more server nodes increases capacity linearly.
* **Fault Tolerance:** No single point of failure (stateless application tier).
* **Message Reliability:** Handling temporary disconnects without data loss.

## 2. Architecture Overview
The system follows a **Distributed Stateless Architecture** leveraging the Publisher/Subscriber pattern.

### High-Level Components
1.  **Ingress (Nginx Load Balancer):** Distributes incoming HTTP and SSE connections across available Go nodes using a Round-Robin algorithm. Handles SSL termination and connection draining.
2.  **Application Tier (Go Engine):** Stateless instances that manage long-lived SSE connections. They do not store message state locally, allowing them to crash or restart without losing data persistence.
3.  **Message Broker (Redis Pub/Sub):** Acts as the synchronization backbone. When *Server A* receives a message, it publishes it to Redis, which broadcasts it to *Server B*, *Server C*, etc., ensuring all connected clients receive the update regardless of which node they are connected to.
4.  **Persistence Layer (Redis Lists):** Implements the "Inbox Pattern" to store pending messages for disconnected users, guaranteeing at-least-once delivery.



## 3. Key Design Decisions

### 3.1 Why Server-Sent Events (SSE) instead of WebSockets?
While WebSockets provide bi-directional communication, they introduce significant complexity in state management and load balancing.
* **Unidirectional Efficiency:** Most real-time features (notifications, feeds, tickers) are read-heavy (Server -> Client). SSE is optimized specifically for this.
* **Statelessness:** SSE works over standard HTTP. This makes it compatible with standard Load Balancers (Nginx) and corporate firewalls without complex protocol upgrades or sticky sessions.
* **Automatic Reconnection:** The `EventSource` API in browsers handles reconnection automatically, simplifying the client-side code.

### 3.2 The "Inbox Pattern" for Reliability
Pure Pub/Sub is "fire and forget"—if a user is offline, they miss the message. To solve this without the heaviness of Kafka:
* **Hybrid Approach:** Messages are broadcast via Pub/Sub for real-time delivery *AND* pushed to a Redis List (`RPUSH inbox:user_id`).
* **On Connect:** When a client connects, the system checks the Redis List (`LRANGE`) and flushes pending messages before opening the SSE stream.
* **Result:** Effectively-once delivery semantics for the user experience.

## 4. Scalability & Performance
* **Concurrency:** Each Go node uses lightweight Goroutines (green threads). A single instance was benchmarked to handle **10,000+ concurrent connections** with minimal RAM footprint.
* **Throughput:** The system achieved **69 MB/30s** throughput in local stress tests (k6), processing ~3,000 messages/minute with sub-10ms latency.
* **Horizontal Scaling:** New Go nodes can be added to the Docker Compose setup immediately. Nginx automatically routes traffic to them, and they instantly hook into the Redis Pub/Sub stream.

## 5. Trade-offs & Future Work
* **Ordering:** Redis Pub/Sub does not guarantee global ordering. For a chat app, timestamp-based client-side reordering is sufficient. For a financial ledger, a strictly ordered log (like Kafka) would be required.
* **Delivery Guarantees:** Currently provides "At-Least-Once." "Exactly-Once" would require client-side deduplication logic (using Message IDs).