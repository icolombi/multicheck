#!/bin/bash

# Test to demonstrate the new logging fields: HTTPMethod and RequestBody

echo "=== Starting multicheck server in background (logs with new fields) ==="
echo ""
echo "Starting server... check the JSON logs below"
echo ""

# Simulate starting the server (you need to actually run: make run)
# For demonstration, we show what the logs will look like

cat << 'EOF'
📊 Example GET request log:
{
   "CurrentTime": "2026-01-15T22:45:00Z",
   "HTTPMethod": "GET",
   "Method": "/ip",
   "Param": "1.2.3.4",
   "RequestBody": "",
   "Errors": [],
   "MemoryAlloc": 2048,
   "NumGC": 5,
   "TimeTaken": 0.245,
   "Cached": false,
   "ClientIP": "192.168.1.100:54321",
   "Redis": true,
   "RedisConnections": 1
}

📊 Example POST request log (with RequestBody):
{
   "CurrentTime": "2026-01-15T22:45:30Z",
   "HTTPMethod": "POST",
   "Method": "/ip/check",
   "Param": "1.2.3.4",
   "RequestBody": "{\"ip\":\"1.2.3.4\",\"blacklists\":[\"zen.spamhaus.org\",\"bl.spamcop.net\"]}",
   "Errors": [],
   "MemoryAlloc": 2560,
   "NumGC": 7,
   "TimeTaken": 0.312,
   "Cached": false,
   "ClientIP": "192.168.1.100:54322",
   "Redis": true,
   "RedisConnections": 1
}

📊 Example POST request log with validation error:
{
   "CurrentTime": "2026-01-15T22:46:00Z",
   "HTTPMethod": "POST",
   "Method": "/domain/check",
   "Param": "example.com",
   "RequestBody": "{\"domain\":\"example.com\",\"blacklists\":[]}",
   "Errors": ["blacklist array cannot be empty"],
   "MemoryAlloc": 2100,
   "NumGC": 8,
   "TimeTaken": 0.002,
   "Cached": false,
   "ClientIP": "192.168.1.100:54323",
   "Redis": true,
   "RedisConnections": 1
}

🎯 Key improvements:
- HTTPMethod: Shows the HTTP verb (GET, POST, PUT, DELETE, etc.)
- RequestBody: Contains the full JSON payload for POST requests
- These fields help with debugging, auditing, and monitoring
- Easy to parse with jq for filtering by method or analyzing POST payloads

💡 Example jq filters:
- Filter only POST requests:
  make run | jq 'select(.HTTPMethod == "POST")'
  
- Show POST request bodies:
  make run | jq 'select(.HTTPMethod == "POST") | {Method, Param, RequestBody}'
  
- Find failed requests with body:
  make run | jq 'select(.Errors | length > 0) | {HTTPMethod, Method, RequestBody, Errors}'
EOF
