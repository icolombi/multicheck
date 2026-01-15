#!/bin/bash

# Examples for POST /ip/check and POST /domain/check endpoints

echo "=== POST /ip/check - Valid request with custom blacklists ==="
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "1.2.3.4",
    "blacklists": [
      "zen.spamhaus.org",
      "bl.spamcop.net",
      "cbl.abuseat.org"
    ]
  }' | jq

echo -e "\n=== POST /ip/check - Invalid IP format ==="
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "invalid-ip",
    "blacklists": ["zen.spamhaus.org"]
  }' | jq

echo -e "\n=== POST /ip/check - Too many blacklists (exceeds limit) ==="
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "1.2.3.4",
    "blacklists": [
      "bl1.example.org", "bl2.example.org", "bl3.example.org", "bl4.example.org",
      "bl5.example.org", "bl6.example.org", "bl7.example.org", "bl8.example.org",
      "bl9.example.org", "bl10.example.org", "bl11.example.org", "bl12.example.org",
      "bl13.example.org", "bl14.example.org", "bl15.example.org", "bl16.example.org",
      "bl17.example.org", "bl18.example.org", "bl19.example.org", "bl20.example.org",
      "bl21.example.org"
    ]
  }' | jq

echo -e "\n=== POST /ip/check - Invalid blacklist format ==="
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "1.2.3.4",
    "blacklists": ["AAAA", "zen.spamhaus.org"]
  }' | jq

echo -e "\n=== POST /ip/check - Empty blacklist array ==="
curl -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "1.2.3.4",
    "blacklists": []
  }' | jq

echo -e "\n=== POST /domain/check - Valid request with custom blacklists ==="
curl -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "blacklists": [
      "dbl.spamhaus.org",
      "multi.uribl.com"
    ]
  }' | jq

echo -e "\n=== POST /domain/check - Invalid domain format ==="
curl -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "invalid domain!",
    "blacklists": ["dbl.spamhaus.org"]
  }' | jq

echo -e "\n=== POST /domain/check - Invalid blacklist with consecutive dots ==="
curl -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "blacklists": ["example..org"]
  }' | jq

echo -e "\n=== POST /domain/check - Invalid JSON body ==="
curl -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "blacklists": ["dbl.spamhaus.org"],
    "extraField": "not-allowed"
  }' | jq
