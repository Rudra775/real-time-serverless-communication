# Serverless Real-Time Communication Engine (Go)

A **serverless-first real-time messaging engine** built in **Go**, designed as a **transport-layer alternative to WebSockets** for environments where persistent TCP connections are unreliable or unsupported.

This project focuses on **explicit concurrency control, reliability guarantees, and distributed system tradeoffs**, rather than framework-level abstractions.

---

## 🚀 Motivation

Traditional WebSocket-based systems rely on:
- Long-lived TCP connections
- Sticky sessions
- Stateful servers

These assumptions break down in **serverless and horizontally scaled environments** (AWS Lambda, Cloud Run, Vercel).

This project explores an alternative architecture:
- **Server → Client**: Server-Sent Events (SSE)
- **Client → Server**: HTTP POST
- **Cross-instance coordination**: Redis Pub/Sub
- **Reliability**: ACK-based at-least-once delivery

The result is a **stateless, scalable, and serverless-compatible real-time communication engine**.

---

## 🎯 Design Goals

- Serverless compatibility (no persistent connections)
- Explicit Go concurrency model
- At-least-once message delivery
- Horizontal scalability without sticky sessions
- Clear failure modes and tradeoffs
- Library-first, framework-agnostic API

---

## 🏗 Architecture Overview

Clients (Browser / Services)
│
│ SSE (receive)
│ HTTP POST (send)
▼
Go API Layer
│
├── Session Manager
├── Message Router
├── Room Registry
├── ACK Tracker (mandatory)
▼
Redis
├── Pub/Sub fanout
└── Pending message tracking


---

## 🔁 Message Flow

1. Client establishes an SSE stream via `/connect`
2. Client sends messages via `POST /send`
3. Server publishes messages to Redis Pub/Sub
4. All instances receive the event
5. Messages are routed to local sessions
6. Client acknowledges delivery via `POST /ack`
7. Unacknowledged messages are retried

---

## ⚙ Core Concurrency Model (Go)

- One goroutine per session
- Bounded channels for backpressure
- Context-based lifecycle control
- Explicit cleanup on disconnect

This ensures predictable resource usage and avoids goroutine leaks under load.

---

## 📦 Features

- Real-time messaging without WebSockets
- Room-based broadcasting
- Direct session messaging
- Redis-backed Pub/Sub fanout
- **At-least-once delivery (ACK-based)**
- Automatic retries for dropped messages
- Stateless server instances
- Framework-agnostic Go library

---

## 🔌 Public API

### HTTP Endpoints

| Endpoint | Description |
|--------|------------|
| `GET /connect?user=ID` | Establish SSE stream |
| `POST /send` | Emit message to room or session |
| `POST /ack` | Confirm message delivery |
| `GET /health` | Health check |

---

## 📚 Go Library Usage

```go
server, _ := socketserve.NewServer(cfg)

http.HandleFunc("/connect", server.HandleConnect)
http.HandleFunc("/send", server.HandleSend)
http.HandleFunc("/ack", server.HandleAck)

