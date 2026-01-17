# Performance Benchmarks

## Test Environment
- **Hardware:** Local Development Environment (Docker on Windows/MingW)
- **Engine:** Go 1.24 + Redis 7 (Alpine)
- **Protocol:** Server-Sent Events (SSE) for egress, HTTP POST for ingress
- **Load Testing Tool:** k6 v0.56

## Stress Test Results (Ingress + Egress)
A "Full Load" test simulating 200 concurrent listeners (SSE) and 10 concurrent publishers broadcasting messages.

| Metric | Result | Description |
| :--- | :--- | :--- |
| **Throughput** | **69 MB / 30s** | Total data broadcasted to clients |
| **Request Rate** | **~65 req/s** | Sustained message processing rate |
| **Latency (P95)** | **7.46 ms** | Time to process incoming messages |
| **Stability** | **100%** | Zero connection drops for connected clients |
| **Concurrency** | **210 VUs** | 200 Listeners + 10 Publishers active simultaneously |

### Key Takeaway
The system demonstrated the ability to broadcast **2.3 MB of data per second** while maintaining single-digit millisecond latency (P95 < 8ms) for publishers. The SSE connection pool remained stable (100% success rate) throughout the saturation test.