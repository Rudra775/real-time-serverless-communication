import { check, sleep } from 'k6';
import http from 'k6/http';

export const options = {
  stages: [
    { duration: '30s', target: 50 },  // Warm up to 50 users
    { duration: '1m', target: 200 },  // Ramp to 200 concurrent users
    { duration: '30s', target: 0 },   // Cool down
  ],
};

export default function () {
  // Use a random user ID to simulate different people
  const userId = `user_${__VU}`;
  
  // Payload matching your Go struct
  const payload = JSON.stringify({
    to_user: "all",           // Broadcast to room/all
    message: `Hello from VU ${__VU}` 
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  // Hit the Nginx Load Balancer (Port 80)
  const res = http.post('http://localhost:80/send', payload, params);

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  // Wait 1 second between messages (Realistic Chat)
  sleep(1); 
}