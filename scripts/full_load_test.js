import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    // Team A: 200 Users listening (SSE)
    listeners: {
      executor: 'constant-vus',
      vus: 200,
      duration: '30s',
      exec: 'listener', // Runs the listener function below
    },
    // Team B: 10 Users spamming messages (Publishers)
    publishers: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'publisher', // Runs the publisher function below
    },
  },
};

// ----------------------------------------------------------------
// TEAM A: The Listeners (SSE)
// ----------------------------------------------------------------
export function listener() {
  const userId = `listener_${__VU}`;
  const url = `http://localhost:8080/connect?id=${userId}`;

  const params = { timeout: '25s' }; 

  try {
    const res = http.get(url, params);
    // We expect this to timeout (success for SSE)
  } catch (e) {
    if (e.error_code !== 1050) console.error('Listener died:', e);
  }
}

// ----------------------------------------------------------------
// TEAM B: The Publishers (POST /send)
// ----------------------------------------------------------------
export function publisher() {
  const url = 'http://localhost:8080/send';
  const payload = JSON.stringify({
    message: "Hello from load test!",
    // We broadcast to "general" channel if your app supports it, 
    // or specific users if your logic requires it.
    channel: "general" 
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const res = http.post(url, payload, params);
  
  check(res, {
    'publisher status 200': (r) => r.status === 200,
  });

  sleep(0.1); // Send 10 messages per second per user
}