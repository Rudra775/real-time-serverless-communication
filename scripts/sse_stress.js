import http from 'k6/http';
import { check } from 'k6';

export const options = {
  // 1. CONCURRENCY: This effectively tests "How many users can be online at once?"
  // Start with 200, then try bumping to 1000 or 2000.
  vus: 200, 
  duration: '30s',
};

export default function () {
  const userId = `stress_user_${__VU}`;
  const url = `http://localhost:8080/connect?id=${userId}`;

  const params = {
    // 2. THE TRICK: We tell k6 to wait 25s for a response.
    // Since SSE is an infinite stream, k6 will just sit there holding the line open.
    // This simulates a user staying on the page for 25 seconds.
    timeout: '25s', 
  };

  try {
    const res = http.get(url, params);
    
    // If the server closes the connection properly (e.g. restart), we get 200.
    check(res, { 'status is 200': (r) => r.status === 200 });
  } catch (error) {
    // 3. ERROR HANDLING: 
    // If k6 says "Request timed out", that is actually a SUCCESS for SSE! 
    // It means the server kept the connection open the whole time without crashing.
    if (error.error_code !== 1050) { // 1050 is the timeout error code in k6
      console.error(`Connection failed for ${userId}:`, error);
    }
  }
}