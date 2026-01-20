#!/bin/bash

# Test script per verificare il funzionamento della cache key nel frontend
# Date: 2026-01-20

echo "=========================================="
echo "Test Cache Key - Backend API"
echo "=========================================="
echo

# Colori per output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test 1: GET endpoint IP
echo -e "${BLUE}Test 1: GET /ip/{ip} - Cache Key${NC}"
RESPONSE=$(curl -s http://localhost:8080/ip/1.1.1.1)
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
if [ "$CACHE_KEY" == "1.1.1.1" ]; then
    echo -e "${GREEN}✓ PASS${NC} - CacheKey for GET IP: $CACHE_KEY"
else
    echo -e "${RED}✗ FAIL${NC} - Expected '1.1.1.1', got: $CACHE_KEY"
fi
echo

# Test 2: GET endpoint Domain
echo -e "${BLUE}Test 2: GET /domain/{domain} - Cache Key${NC}"
RESPONSE=$(curl -s http://localhost:8080/domain/example.com)
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
if [ "$CACHE_KEY" == "example.com" ]; then
    echo -e "${GREEN}✓ PASS${NC} - CacheKey for GET Domain: $CACHE_KEY"
else
    echo -e "${RED}✗ FAIL${NC} - Expected 'example.com', got: $CACHE_KEY"
fi
echo

# Test 3: POST endpoint IP con blacklist personalizzate
echo -e "${BLUE}Test 3: POST /ip/check - Cache Key with Custom Blacklists${NC}"
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}')
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
if [[ "$CACHE_KEY" == post:ip:8.8.8.8:* ]]; then
    echo -e "${GREEN}✓ PASS${NC} - CacheKey for POST IP: $CACHE_KEY"
else
    echo -e "${RED}✗ FAIL${NC} - Expected 'post:ip:8.8.8.8:*', got: $CACHE_KEY"
fi
echo

# Test 4: POST endpoint Domain con blacklist personalizzate
echo -e "${BLUE}Test 4: POST /domain/check - Cache Key with Custom Blacklists${NC}"
RESPONSE=$(curl -s -X POST http://localhost:8080/domain/check \
  -H "Content-Type: application/json" \
  -d '{"domain":"test.uribl.com","blacklists":["multi.uribl.com"]}')
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
if [[ "$CACHE_KEY" == post:domain:test.uribl.com:* ]]; then
    echo -e "${GREEN}✓ PASS${NC} - CacheKey for POST Domain: $CACHE_KEY"
else
    echo -e "${RED}✗ FAIL${NC} - Expected 'post:domain:test.uribl.com:*', got: $CACHE_KEY"
fi
echo

# Test 5: Cancellazione cache con POST endpoint key
echo -e "${BLUE}Test 5: Cache Deletion with POST CacheKey${NC}"

# Prima richiesta - popola cache
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"9.9.9.9","blacklists":["zen.spamhaus.org"]}')
CACHE_KEY=$(echo $RESPONSE | jq -r '.CacheKey')
CACHED1=$(echo $RESPONSE | jq -r '.Cached')
echo "  First request - Cached: $CACHED1, CacheKey: $CACHE_KEY"

# Seconda richiesta - dovrebbe essere in cache
sleep 1
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"9.9.9.9","blacklists":["zen.spamhaus.org"]}')
CACHED2=$(echo $RESPONSE | jq -r '.Cached')
echo "  Second request - Cached: $CACHED2"

# Cancella la cache usando il CacheKey
CLEAR_RESPONSE=$(curl -s http://localhost:8080/clear-cache/$CACHE_KEY)
CLEAR_STATUS=$(echo $CLEAR_RESPONSE | jq -r '.Status')
echo "  Cache deletion - Status: $CLEAR_STATUS"

# Terza richiesta - NON dovrebbe essere in cache
sleep 1
RESPONSE=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"9.9.9.9","blacklists":["zen.spamhaus.org"]}')
CACHED3=$(echo $RESPONSE | jq -r '.Cached')
echo "  Third request - Cached: $CACHED3"

if [ "$CACHED2" == "true" ] && [ "$CACHED3" == "false" ] && [ "$CLEAR_STATUS" == "true" ]; then
    echo -e "${GREEN}✓ PASS${NC} - Cache deletion works correctly (Cached: false → true → deleted → false)"
else
    echo -e "${RED}✗ FAIL${NC} - Cache deletion failed (Cached2=$CACHED2, Clear=$CLEAR_STATUS, Cached3=$CACHED3)"
fi
echo

# Test 6: Verifica che blacklist diversi = cache key diversi
echo -e "${BLUE}Test 6: Different Blacklists = Different Cache Keys${NC}"
RESPONSE1=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["zen.spamhaus.org"]}')
KEY1=$(echo $RESPONSE1 | jq -r '.CacheKey')

RESPONSE2=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","blacklists":["bl.spamcop.net"]}')
KEY2=$(echo $RESPONSE2 | jq -r '.CacheKey')

if [ "$KEY1" != "$KEY2" ]; then
    echo -e "${GREEN}✓ PASS${NC} - Different blacklists produce different keys"
    echo "  Key 1: $KEY1"
    echo "  Key 2: $KEY2"
else
    echo -e "${RED}✗ FAIL${NC} - Keys should be different but are the same: $KEY1"
fi
echo

# Test 7: Verifica che blacklist in ordine diverso = stessa cache key
echo -e "${BLUE}Test 7: Same Blacklists in Different Order = Same Cache Key${NC}"
RESPONSE1=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"7.7.7.7","blacklists":["zen.spamhaus.org","bl.spamcop.net"]}')
KEY1=$(echo $RESPONSE1 | jq -r '.CacheKey')
CACHED1=$(echo $RESPONSE1 | jq -r '.Cached')

RESPONSE2=$(curl -s -X POST http://localhost:8080/ip/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"7.7.7.7","blacklists":["bl.spamcop.net","zen.spamhaus.org"]}')
KEY2=$(echo $RESPONSE2 | jq -r '.CacheKey')
CACHED2=$(echo $RESPONSE2 | jq -r '.Cached')

if [ "$KEY1" == "$KEY2" ] && [ "$CACHED2" == "true" ]; then
    echo -e "${GREEN}✓ PASS${NC} - Same blacklists in different order produce same key (with cache hit)"
    echo "  Key: $KEY1"
    echo "  Cached: $CACHED1 → $CACHED2"
else
    echo -e "${RED}✗ FAIL${NC} - Keys should be the same"
    echo "  Key 1: $KEY1 (Cached: $CACHED1)"
    echo "  Key 2: $KEY2 (Cached: $CACHED2)"
fi
echo

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "All tests completed. Check results above."
echo
echo "Frontend URL: http://localhost:5173"
echo "Backend URL: http://localhost:8080"
echo
