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
    data = request.json
    user_id = data.get('user_id')
    message = data.get('message')
    
    # Send to your real-time engine
    payload = {
        "user_id": user_id,
        "data": {
            "type": "notification",
            "title": "New Notification",
            "message": message,
            "timestamp": data.get('timestamp')
        }
    }
    
    response = requests.post(f"{REALTIME_ENGINE}/send", json=payload)
    
    return jsonify({"status": "sent", "code": response.status_code})

if __name__ == '__main__':
    app.run(port=5000, debug=True)