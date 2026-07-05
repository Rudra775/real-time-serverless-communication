# Stateless Real-Time SSE Gateway

A distributed, stateless real-time messaging gateway built in Go. Designed as an alternative to WebSockets for serverless, auto-scaling, and containerized environments where long-lived stateful application connections are a liability.

This system guarantees at-least-once delivery using an Inbox Pattern backed by Redis, enabling recovery from temporary connection drops.

---

## The Problem: WebSockets in Serverless Architectures

Traditional WebSocket architectures require stateful, persistent TCP connections between the client and the application server. This model breaks down in serverless (e.g., AWS Lambda, Google Cloud Run) and auto-scaling container environments:
* **TCP Statefulness:** Serverless instances are ephemeral and terminate frequently, causing constant connection drops.
* **Lack of Sticky Sessions:** Routing incoming client requests to the specific server holding their connection state requires complex load-balancer configurations.
* **Egress Bottlenecks:** Stateless compute tiers are not optimized to maintain thousands of idle TCP connections open simultaneously.

---

## The Solution: Decoupled Ingress and Egress

This project solves the stateless real-time communication problem by decoupling the connection state from your main application server using a sidecar gateway.

```
                  +-----------------------------------+
                  |           Client Browser          |
                  +-----------------------------------+
                     /                             ^
      1. POST /action                               \ 4. Stream SSE
                   /                                 \
                  v                                   \
        +------------------+                   +---------------+
        |   Main Backend   |                   |  Go SSE Node  |
        | (Flask, Node, ..)|                   +---------------+
        +------------------+                           ^
                  |                                    |
            2. POST /send                         3. Pub/Sub
                  |                                    |
                  v                                    |
        +------------------------------------------------------+
        |                    Redis Cluster                     |
        |              (Pub/Sub & Inbox Queues)                |
        +------------------------------------------------------+
```

1. **Client Ingress (Standard HTTP):** Clients send mutations or actions to your main application backend using standard HTTP POST requests. 
2. **Server Egress (Server-Sent Events):** The Go SSE Gateway manages the outbound streaming connections. Your backend publishes events to the Go Gateway, which routes them to the correct clients.
3. **Stateless Scalability:** Go gateway nodes do not share connection state. If a client reconnects to a different gateway node, Redis synchronizes the delivery.

---

## Key Features

* **Generic Channel Routing:** Clients subscribe to dynamic channels (e.g. `room_123` or `ticker_AAPL`) to receive targeted broadcasts.
* **Stateless Authorization (JWT):** Connection handshakes are authorized statelessly using JSON Web Tokens (JWT) signed by your main backend, containing the list of allowed channels.
* **Reliable Delivery (Inbox Pattern):** Important messages are persisted in Redis queues if a client is offline, flushed automatically when they reconnect, and deleted once the client acknowledges (ACK) receipt.
* **Administrative Isolation:** Ingress control endpoints (`/send`) are protected with API keys and blocked at the reverse-proxy level, allowing only secure server-to-server publishing.

---

## Architecture and Configuration

The gateway runs as a sidecar next to your main backend application. A typical deployment configuration involves Nginx acting as a load balancer and reverse proxy, routing client traffic to the Go instances.

### load Balancer (Nginx) Configuration
Nginx handles public SSL termination and routes traffic based on endpoints:
* `GET /connect` -> Publicly routed to the Go gateway (disabling proxy buffering for real-time streaming).
* `POST /ack` -> Publicly routed to the Go gateway (authenticated via client JWT).
* `POST /send` -> Blocked externally; only accessible within the private VPC/network.

---

## API Specifications

### 1. Establish SSE Connection
Establishes a persistent Server-Sent Events stream.

```http
GET /connect?id={client_id}&channels=general,room_A
```

In production mode (when `JWT_SECRET` is configured), connection is established using a JWT:

```http
GET /connect?token={signed_jwt}
```

*The JWT must contain standard claims along with the `channels` string array.*

### 2. Publish Message (Internal Server Only)
Publishes a message to a channel. If `to_user` or `to_users` is specified, the message is stored in the target user's offline queue.

* **URL:** `/send`
* **Method:** `POST`
* **Headers:** `X-API-Key: {your_secret_key}`
* **Payload:**
```json
{
  "channel": "room_A",
  "to_user": "user_123",
  "data": {
    "type": "notification",
    "title": "Update",
    "message": "Hello World"
  }
}
```

### 3. Acknowledge Receipt
Clears the client's offline message queue in Redis. Clients call this immediately upon establishing a connection.

* **URL:** `/ack`
* **Method:** `POST`
* **Payload (Development):**
```json
{
  "user_id": "user_123"
}
```
* **Payload (Production):**
```json
{
  "user_id": "user_123",
  "token": "{signed_jwt}"
}
```

---

## Benchmarks

Tested on standard hardware running local Docker containers under stress testing with `k6`:

| Metric | Result | Context |
| :--- | :--- | :--- |
| **Throughput** | **69 MB / 30s** | Data broadcasted to 200 concurrent clients |
| **Latency (P95)** | **14.12 ms** | End-to-end processing (Nginx -> Go -> Redis) |
| **Latency (Avg)** | **5.89 ms** | Typical response time under load |
| **Concurrency** | **200 VUs** | Stable SSE connections held open |
| **Reliability** | **100%** | Zero dropped requests (11,000+ total) |

*(Full details and test runner configuration are available in `BENCHMARKS.md`.)*

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.
