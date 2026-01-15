import http from 'k6/http';
import { check, sleep } from 'k6';

// Configuration: 100 users, running for 30 seconds
export const options = {
  vus: 100,
  duration: '30s',
};

export default function () {
  // 1. Define the payload
  const payload = JSON.stringify({
    text: "Stress test message",
    user: "load_tester",
    to_user: "user_1" // Targeting a specific inbox triggers the Redis List logic
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  // 2. Send the request (POST /send)
  const res = http.post('http://localhost:8080/send', payload, params);

  // 3. Verify the server didn't crash (Expect HTTP 200)
  check(res, {
    'is status 200': (r) => r.status === 200,
  });

  // Short pause between requests (0.1s) to behave like a "fast" human/bot
  sleep(0.1);
}