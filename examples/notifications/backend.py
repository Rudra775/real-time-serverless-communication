from flask import Flask, render_template, request, jsonify
import requests
import json

app = Flask(__name__)

# Your real-time engine URL
REALTIME_ENGINE = "http://localhost:8080"

@app.route('/')
def index():
    return render_template('notifications.html')

@app.route('/send-notification', methods=['POST'])
def send_notification():
    """Simulate sending a notification (e.g., new order, payment received)"""
    import os
    data = request.json
    user_id = data.get('user_id')
    message = data.get('message')
    
    # Send to your real-time engine
    # Under the channels pattern, we send to the channel "user_{user_id}"
    # and save to the user's inbox "to_user" = user_id.
    payload = {
        "channel": f"user_{user_id}",
        "to_user": user_id,
        "data": {
            "type": "notification",
            "title": "New Notification",
            "message": message,
            "timestamp": data.get('timestamp')
        }
    }
    
    headers = {"Content-Type": "application/json"}
    api_key = os.getenv("REALTIME_API_KEY")
    if api_key:
        headers["X-API-Key"] = api_key
    
    response = requests.post(f"{REALTIME_ENGINE}/send", json=payload, headers=headers)
    
    return jsonify({"status": "sent", "code": response.status_code})


if __name__ == '__main__':
    app.run(port=5000, debug=True)