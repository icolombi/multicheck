package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// setupTestWithResolver initializes configuration, Redis pool and resolver for tests
func setupTestWithResolver() {
	if configuration.listenPort == "" {
		configuration = ReadConfig(configuration)
		c = redisConnect()
	}

	// Always initialize resolver and nameservers (even if already configured)
	nameservers = configuration.nameServers
	if len(nameservers) > 0 {
		resolver = &net.Resolver{
			PreferGo:     true,
			StrictErrors: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				randomIndex := rand.Intn(len(nameservers))
				nameserver := nameservers[randomIndex]
				return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
			},
		}
	}
}

func TestHealthCheckHandler(t *testing.T) {
	setupTestWithResolver()

	// Struct to hold the response
	type Message struct {
		Alive            bool
		Redis            bool
		RedisConnections int
	}

	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HealthCheckHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Remove newlines at the end of the string

	var r Message
	err = json.NewDecoder(rr.Body).Decode(&r)
	if err != nil {
		fmt.Println(err)
	}

	if r.Redis != true || r.Alive != true {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), r)
	}
}

func TestDomainBlacklist(t *testing.T) {
	// Initialize configuration, Redis and resolver
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	// Create router with handler
	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	// Create request for test.uribl.com
	req, err := http.NewRequest("GET", "/domain/test.uribl.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Execute the request
	r.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decodifica la risposta JSON
	var response Domain
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check the domain is valid
	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	// Check the domain is blacklisted
	if !response.BlackListed {
		t.Errorf("Expected test.uribl.com to be blacklisted, but it was not detected as blacklisted")
	}

	// check that multi.uribl.com is in the blacklist list
	blacklistIPs, found := response.BlackList["multi.uribl.com"]
	if !found {
		t.Errorf("Expected test.uribl.com to be blacklisted by multi.uribl.com, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// check that the response is 127.0.0.14
		expectedIP := net.ParseIP("127.0.0.14")
		found := false
		for _, ip := range blacklistIPs {
			if ip.Equal(expectedIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected IP 127.0.0.14 from multi.uribl.com, got %v", blacklistIPs)
		}
	}

	// Check that there are no errors
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

func TestIPBlacklist(t *testing.T) {
	// Initialize configuration, Redis and resolver
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	// Create router with handler
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	// Create request for 127.0.0.2
	req, err := http.NewRequest("GET", "/ip/127.0.0.2", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Execute the request
	r.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decode JSON response
	var response Ip
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify IP is valid
	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	// Verify IP is blacklisted
	if !response.BlackListed {
		t.Errorf("Expected 127.0.0.2 to be blacklisted, but it was not detected as blacklisted")
	}

	// Verify zen.spamhaus.org is in the blacklist list
	blacklistIPs, found := response.BlackList["zen.spamhaus.org"]
	if !found {
		t.Errorf("Expected 127.0.0.2 to be blacklisted by zen.spamhaus.org, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Verify response code is 127.0.0.2
		expectedIP := net.ParseIP("127.0.0.2")
		found := false
		for _, ip := range blacklistIPs {
			if ip.Equal(expectedIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected IP 127.0.0.2 from zen.spamhaus.org, got %v", blacklistIPs)
		}
	}

	// Check that there are no errors
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

// Test to verify that GET /ip returns 400 with invalid IP
func TestGetIpInvalid(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	req, _ := http.NewRequest("GET", "/ip/192.168.8.111111", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that GET /domain returns 400 with invalid domain
func TestGetDomainInvalid(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	req, _ := http.NewRequest("GET", "/domain/invalid..domain", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /ip/check returns 400 with invalid IP
func TestPostCheckIpInvalidIP(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	body := CheckIpRequest{IP: "999.999.999.999", Blacklists: []string{"zen.spamhaus.org"}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /ip/check returns 400 with empty blacklist
func TestPostCheckIpEmptyBlacklists(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	body := CheckIpRequest{IP: "8.8.8.8", Blacklists: []string{}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /ip/check returns 400 with too many blacklists
func TestPostCheckIpTooManyBlacklists(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	bl := make([]string, configuration.MaxCustomBlacklists+1)
	for i := range bl {
		bl[i] = fmt.Sprintf("bl%d.org", i)
	}
	body := CheckIpRequest{IP: "8.8.8.8", Blacklists: bl}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /ip/check returns 400 with invalid nameserver
func TestPostCheckIpInvalidNameservers(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	body := CheckIpRequest{IP: "8.8.8.8", Blacklists: []string{"zen.spamhaus.org"}, Nameservers: []string{"invalid"}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with invalid domain
func TestPostCheckDomainInvalidDomain(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")
	body := CheckDomainRequest{Domain: "invalid..domain", Blacklists: []string{"multi.uribl.com"}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with empty blacklist
func TestPostCheckDomainEmptyBlacklists(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")
	body := CheckDomainRequest{Domain: "example.com", Blacklists: []string{}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with malformed JSON
func TestPostCheckDomainInvalidJSON(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")
	invalidJSON := []byte(`{"domain": "example.com", "blacklists": [`)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestPostCheckDomain(t *testing.T) {
	// Initialize configuration, Redis and resolver
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	// Create router with handler
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	// Prepare request body
	requestBody := CheckDomainRequest{
		Domain:     "test.uribl.com",
		Blacklists: []string{"multi.uribl.com"},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Execute the request
	r.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decode JSON response
	var response Domain
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check the domain is valid
	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	// Check the domain is blacklisted
	if !response.BlackListed {
		t.Errorf("Expected test.uribl.com to be blacklisted, but it was not detected as blacklisted")
	}

	// Check that multi.uribl.com is in the blacklist list
	blacklistIPs, found := response.BlackList["multi.uribl.com"]
	if !found {
		t.Errorf("Expected test.uribl.com to be blacklisted by multi.uribl.com, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Verifica che il codice di risposta sia 127.0.0.14
		expectedIP := net.ParseIP("127.0.0.14")
		found := false
		for _, ip := range blacklistIPs {
			if ip.Equal(expectedIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected IP 127.0.0.14 from multi.uribl.com, got %v", blacklistIPs)
		}
	}

	// Check that there are no errors
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

func TestPostCheckIP(t *testing.T) {
	// Initialize configuration, Redis and resolver
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	// Create router with handler
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	// Prepare request body
	requestBody := CheckIpRequest{
		IP:         "2.0.0.127",
		Blacklists: []string{"zen.spamhaus.org"},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}

	// Create POST request
	req, err := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Execute the request
	r.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decode JSON response
	var response Ip
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check IP is valid
	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	// Check IP is blacklisted
	if !response.BlackListed {
		t.Errorf("Expected 2.0.0.127 to be blacklisted, but it was not detected as blacklisted")
	}

	// Check that zen.spamhaus.org is in the blacklist list
	blacklistIPs, found := response.BlackList["zen.spamhaus.org"]
	if !found {
		t.Errorf("Expected 2.0.0.127 to be blacklisted by zen.spamhaus.org, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Check that the response is 127.0.0.11
		expectedIP := net.ParseIP("127.0.0.11")
		found := false
		for _, ip := range blacklistIPs {
			if ip.Equal(expectedIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected IP 127.0.0.11 from zen.spamhaus.org, got %v", blacklistIPs)
		}
	}

	// Check that there are no errors
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

// Test to verify that POST /ip/check returns correct CacheKey
func TestPostCheckIpCacheKey(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	// Prepare request body
	requestBody := CheckIpRequest{
		IP:         "2.0.0.127",
		Blacklists: []string{"zen.spamhaus.org", "bl.spamcop.net"},
	}
	bodyBytes, _ := json.Marshal(requestBody)

	// First request - should not be cached
	req1, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	var response1 Ip
	json.NewDecoder(rr1.Body).Decode(&response1)

	// Check that CacheKey is present
	if response1.CacheKey == "" {
		t.Errorf("Expected CacheKey to be present, got empty string")
	}

	// Check that CacheKey has correct format: post:ip:<ip>:<hash>
	if !strings.HasPrefix(response1.CacheKey, "post:ip:") {
		t.Errorf("Expected CacheKey to start with 'post:ip:', got %s", response1.CacheKey)
	}

	if !strings.Contains(response1.CacheKey, requestBody.IP) {
		t.Errorf("Expected CacheKey to contain IP %s, got %s", requestBody.IP, response1.CacheKey)
	}

	// Check that hash part exists (format: post:ip:<ip>:<16-char-hash>)
	parts := strings.Split(response1.CacheKey, ":")
	if len(parts) != 4 {
		t.Errorf("Expected CacheKey to have 4 parts separated by ':', got %d parts: %s", len(parts), response1.CacheKey)
	} else if len(parts[3]) != 16 {
		t.Errorf("Expected hash part to be 16 characters, got %d: %s", len(parts[3]), parts[3])
	}

	// Second request with same parameters - should be cached with same CacheKey
	req2, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Ip
	json.NewDecoder(rr2.Body).Decode(&response2)

	// Check that second response has same CacheKey
	if response2.CacheKey != response1.CacheKey {
		t.Errorf("Expected same CacheKey for identical requests: %s != %s", response1.CacheKey, response2.CacheKey)
	}

	// Check that second response is from cache
	if !response2.Cached {
		t.Errorf("Expected second request to be cached")
	}

	t.Logf("CacheKey verified: %s (Cached: %v -> %v)", response1.CacheKey, response1.Cached, response2.Cached)
}

// Test to verify that POST /domain/check returns correct CacheKey
func TestPostCheckDomainCacheKey(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	// Prepare request body
	requestBody := CheckDomainRequest{
		Domain:     "test.uribl.com",
		Blacklists: []string{"multi.uribl.com"},
	}
	bodyBytes, _ := json.Marshal(requestBody)

	// First request - should not be cached
	req1, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	var response1 Domain
	json.NewDecoder(rr1.Body).Decode(&response1)

	// Check that CacheKey is present
	if response1.CacheKey == "" {
		t.Errorf("Expected CacheKey to be present, got empty string")
	}

	// Check that CacheKey has correct format: post:domain:<domain>:<hash>
	if !strings.HasPrefix(response1.CacheKey, "post:domain:") {
		t.Errorf("Expected CacheKey to start with 'post:domain:', got %s", response1.CacheKey)
	}

	if !strings.Contains(response1.CacheKey, requestBody.Domain) {
		t.Errorf("Expected CacheKey to contain domain %s, got %s", requestBody.Domain, response1.CacheKey)
	}

	// Check that hash part exists (format: post:domain:<domain>:<16-char-hash>)
	parts := strings.Split(response1.CacheKey, ":")
	if len(parts) != 4 {
		t.Errorf("Expected CacheKey to have 4 parts separated by ':', got %d parts: %s", len(parts), response1.CacheKey)
	} else if len(parts[3]) != 16 {
		t.Errorf("Expected hash part to be 16 characters, got %d: %s", len(parts[3]), parts[3])
	}

	// Second request with same parameters - should be cached with same CacheKey
	req2, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Domain
	json.NewDecoder(rr2.Body).Decode(&response2)

	// Check that second response has same CacheKey
	if response2.CacheKey != response1.CacheKey {
		t.Errorf("Expected same CacheKey for identical requests: %s != %s", response1.CacheKey, response2.CacheKey)
	}

	// Check that second response is from cache
	if !response2.Cached {
		t.Errorf("Expected second request to be cached")
	}

	t.Logf("CacheKey verified: %s (Cached: %v -> %v)", response1.CacheKey, response1.Cached, response2.Cached)
}

// Test to verify that cache deletion works with POST endpoint CacheKey
func TestPostCheckIpCacheDeletionWithCacheKey(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	// Prepare request body
	requestBody := CheckIpRequest{
		IP:         "2.0.0.127",
		Blacklists: []string{"zen.spamhaus.org"},
	}
	bodyBytes, _ := json.Marshal(requestBody)

	// First request - populate cache
	req1, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	var response1 Ip
	json.NewDecoder(rr1.Body).Decode(&response1)

	if response1.CacheKey == "" {
		t.Fatalf("Expected CacheKey to be present")
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	var response2 Ip
	json.NewDecoder(rr2.Body).Decode(&response2)

	if !response2.Cached {
		t.Logf("Warning: Second request was not cached (unexpected)")
	}

	// Clear cache using CacheKey from response
	reqClear, _ := http.NewRequest("GET", "/clear-cache/"+response1.CacheKey, nil)
	rrClear := httptest.NewRecorder()
	r.ServeHTTP(rrClear, reqClear)

	if rrClear.Code != http.StatusOK {
		t.Fatalf("Cache clear request failed with status %v", rrClear.Code)
	}

	var clearResponse ClearCache
	json.NewDecoder(rrClear.Body).Decode(&clearResponse)

	if !clearResponse.Status {
		t.Errorf("Expected cache clear to succeed, got Status=false. Errors: %v", clearResponse.Errors)
	}

	// Third request - should NOT be cached after deletion
	req3, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	var response3 Ip
	json.NewDecoder(rr3.Body).Decode(&response3)

	if response3.Cached {
		t.Errorf("Expected third request to NOT be cached after deletion, got Cached=true")
	}

	t.Logf("Cache deletion with CacheKey verified: %s (Cached: %v -> deleted -> %v)",
		response1.CacheKey, response2.Cached, response3.Cached)
}

// Test to verify that GET / returns the endpoint list
func TestRootHandler(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/", RootHandler).Methods("GET")
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}

	var response Root
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check that endpoints list is not empty
	if len(response.EndPoints) == 0 {
		t.Errorf("Expected EndPoints to be populated, got empty list")
	}

	// Check that configuration is present
	if len(response.IpBlacklist) == 0 {
		t.Errorf("Expected IpBlacklist to be populated, got empty list")
	}
}

// Test to verify that GET /clear-cache/{key} works
func TestClearCache(t *testing.T) {
	setupTestWithResolver()

	// First, create a cache entry by requesting an IP
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	testIP := "8.8.8.8"

	// First request to populate cache
	req1, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	// Now clear the cache
	req2, _ := http.NewRequest("GET", "/clear-cache/"+testIP, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr2.Code)
	}

	var response ClearCache
	err := json.NewDecoder(rr2.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Status {
		t.Errorf("Expected Status to be true, got false. Errors: %v", response.Errors)
	}

	if response.Key != testIP {
		t.Errorf("Expected Key to be %s, got %s", testIP, response.Key)
	}
}

// Test to verify that a clean IP is not blacklisted
func TestGetIpNotBlacklisted(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	// 8.8.8.8 should not be blacklisted
	req, _ := http.NewRequest("GET", "/ip/8.8.8.8", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}

	var response Ip
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	if response.BlackListed {
		t.Errorf("Expected 8.8.8.8 to NOT be blacklisted, but it was detected as blacklisted")
	}

	if len(response.BlackList) > 0 {
		t.Errorf("Expected BlackList to be empty, got %v", response.BlackList)
	}
}

// Test to verify that a clean domain is not blacklisted
func TestGetDomainNotBlacklisted(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	// uribl.com should not be blacklisted
	req, _ := http.NewRequest("GET", "/domain/uribl.com", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}

	var response Domain
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	if response.BlackListed {
		t.Errorf("Expected google.com to NOT be blacklisted, but it was detected as blacklisted")
	}

	if len(response.BlackList) > 0 {
		t.Errorf("Expected BlackList to be empty, got %v", response.BlackList)
	}
}

// Test to verify cache hit on second IP request
func TestGetIpCacheHit(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	testIP := "1.1.1.1"

	// First request - should not be cached
	req1, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	var response1 Ip
	json.NewDecoder(rr1.Body).Decode(&response1)

	if response1.Cached {
		t.Logf("Warning: First request was cached (unexpected, but possible if previous test left data)")
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	var response2 Ip
	json.NewDecoder(rr2.Body).Decode(&response2)

	if !response2.Cached {
		t.Errorf("Expected second request to be cached (Cached=true), got Cached=false")
	}
}

// Test to verify cache hit on second domain request
func TestGetDomainCacheHit(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	testDomain := "example.com"

	// First request - should not be cached
	req1, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	var response1 Domain
	json.NewDecoder(rr1.Body).Decode(&response1)

	if response1.Cached {
		t.Logf("Warning: First request was cached (unexpected, but possible if previous test left data)")
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	var response2 Domain
	json.NewDecoder(rr2.Body).Decode(&response2)

	if !response2.Cached {
		t.Errorf("Expected second request to be cached (Cached=true), got Cached=false")
	}
}

// Test to verify that GET /ip returns 400 with IP too long
func TestGetIpTooLong(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	// Create a string longer than maxStringLength (253 characters)
	longIP := ""
	for i := 0; i < 260; i++ {
		longIP += "1"
	}

	req, _ := http.NewRequest("GET", "/ip/"+longIP, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}

	var response Ip
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Status {
		t.Errorf("Expected Status to be false, got true")
	}

	if len(response.Errors) == 0 {
		t.Errorf("Expected error message about length, got no errors")
	}
}

// Test to verify that GET /domain returns 400 with domain too long
func TestGetDomainTooLong(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	// Create a string longer than maxStringLength (253 characters)
	longDomain := ""
	for i := 0; i < 260; i++ {
		longDomain += "a"
	}
	longDomain += ".com"

	req, _ := http.NewRequest("GET", "/domain/"+longDomain, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}

	var response Domain
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Status {
		t.Errorf("Expected Status to be false, got true")
	}

	if len(response.Errors) == 0 {
		t.Errorf("Expected error message about length, got no errors")
	}
}

// Test to verify that POST /ip/check returns 400 with too many nameservers
func TestPostCheckIpTooManyNameservers(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	// Create more nameservers than allowed
	ns := make([]string, configuration.MaxCustomNameservers+1)
	for i := range ns {
		ns[i] = fmt.Sprintf("8.8.%d.%d", i/256, i%256)
	}

	body := CheckIpRequest{IP: "8.8.8.8", Blacklists: []string{"zen.spamhaus.org"}, Nameservers: ns}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with too many nameservers
func TestPostCheckDomainTooManyNameservers(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	// Create more nameservers than allowed
	ns := make([]string, configuration.MaxCustomNameservers+1)
	for i := range ns {
		ns[i] = fmt.Sprintf("8.8.%d.%d", i/256, i%256)
	}

	body := CheckDomainRequest{Domain: "example.com", Blacklists: []string{"multi.uribl.com"}, Nameservers: ns}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with too many blacklists
func TestPostCheckDomainTooManyBlacklists(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	bl := make([]string, configuration.MaxCustomBlacklists+1)
	for i := range bl {
		bl[i] = fmt.Sprintf("bl%d.org", i)
	}
	body := CheckDomainRequest{Domain: "example.com", Blacklists: bl}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /ip/check works with custom nameservers
func TestPostCheckIpWithCustomNameservers(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	// Use a valid custom nameserver (Google DNS)
	body := CheckIpRequest{
		IP:          "2.0.0.127",
		Blacklists:  []string{"zen.spamhaus.org"},
		Nameservers: []string{"8.8.8.8"},
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}

	var response Ip
	json.NewDecoder(rr.Body).Decode(&response)

	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	if !response.Status {
		t.Errorf("Expected Status to be true, got false. Errors: %v", response.Errors)
	}
}

// Test to verify that POST /domain/check works with custom nameservers
func TestPostCheckDomainWithCustomNameservers(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	// Use a valid custom nameserver (Google DNS)
	body := CheckDomainRequest{
		Domain:      "test.uribl.com",
		Blacklists:  []string{"multi.uribl.com"},
		Nameservers: []string{"8.8.8.8"},
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}

	var response Domain
	json.NewDecoder(rr.Body).Decode(&response)

	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	if !response.Status {
		t.Errorf("Expected Status to be true, got false. Errors: %v", response.Errors)
	}
}

// Test to verify that POST /ip/check returns 400 with invalid JSON
func TestPostCheckIpInvalidJSON(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	invalidJSON := []byte(`{"ip": "8.8.8.8", "blacklists": [`)
	req, _ := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify that POST /domain/check returns 400 with invalid nameservers
func TestPostCheckDomainInvalidNameservers(t *testing.T) {
	setupTestWithResolver()
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	body := CheckDomainRequest{Domain: "example.com", Blacklists: []string{"multi.uribl.com"}, Nameservers: []string{"invalid"}}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test to verify cache consistency for domain blacklist checks
// Verifies that cached responses are identical to original responses except for Cached flag
func TestDomainBlacklistCacheConsistency(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	testDomain := "github.com"

	// First request - should not be cached
	req1, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	var response1 Domain
	err := json.NewDecoder(rr1.Body).Decode(&response1)
	if err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Verify domain is valid
	if !response1.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	// Verify first response is not from cache
	if response1.Cached {
		t.Logf("Warning: First request was already cached (possible from previous test)")
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Domain
	err = json.NewDecoder(rr2.Body).Decode(&response2)
	if err != nil {
		t.Fatalf("Failed to decode second response: %v", err)
	}

	// Verify second response is from cache
	if !response2.Cached {
		t.Errorf("Expected second request to be cached (Cached=true), got Cached=false")
	}

	// Compare all fields except Cached and TimeTaken
	if response1.Domain != response2.Domain {
		t.Errorf("Domain mismatch: %v != %v", response1.Domain, response2.Domain)
	}

	if response1.ValidDomain != response2.ValidDomain {
		t.Errorf("ValidDomain mismatch: %v != %v", response1.ValidDomain, response2.ValidDomain)
	}

	if response1.BlackListed != response2.BlackListed {
		t.Errorf("BlackListed mismatch: %v != %v", response1.BlackListed, response2.BlackListed)
	}

	if response1.Status != response2.Status {
		t.Errorf("Status mismatch: %v != %v", response1.Status, response2.Status)
	}

	// Compare BlackList maps
	if len(response1.BlackList) != len(response2.BlackList) {
		t.Errorf("BlackList length mismatch: %v != %v", len(response1.BlackList), len(response2.BlackList))
	}

	for bl, ips1 := range response1.BlackList {
		ips2, found := response2.BlackList[bl]
		if !found {
			t.Errorf("Blacklist %s found in first response but not in second", bl)
			continue
		}

		if len(ips1) != len(ips2) {
			t.Errorf("IP count mismatch for blacklist %s: %v != %v", bl, len(ips1), len(ips2))
			continue
		}

		// Compare IPs (order might differ, so we check for presence)
		for _, ip1 := range ips1 {
			found := false
			for _, ip2 := range ips2 {
				if ip1.Equal(ip2) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("IP %v found in first response but not in second for blacklist %s", ip1, bl)
			}
		}
	}

	// Compare Errors
	if len(response1.Errors) != len(response2.Errors) {
		t.Errorf("Errors length mismatch: %v != %v", len(response1.Errors), len(response2.Errors))
	}

	t.Logf("Cache consistency verified for domain %s: BlackListed=%v, Cached: %v->%v",
		testDomain, response1.BlackListed, response1.Cached, response2.Cached)
}

// Test to verify cache consistency for IP blacklist checks
// Verifies that cached responses are identical to original responses except for Cached flag
func TestIPBlacklistCacheConsistency(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	testIP := "127.0.0.4"

	// First request - should not be cached
	req1, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	var response1 Ip
	err := json.NewDecoder(rr1.Body).Decode(&response1)
	if err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Verify IP is valid
	if !response1.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	// Verify first response is not from cache
	if response1.Cached {
		t.Logf("Warning: First request was already cached (possible from previous test)")
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Ip
	err = json.NewDecoder(rr2.Body).Decode(&response2)
	if err != nil {
		t.Fatalf("Failed to decode second response: %v", err)
	}

	// Verify second response is from cache
	if !response2.Cached {
		t.Errorf("Expected second request to be cached (Cached=true), got Cached=false")
	}

	// Compare all fields except Cached and TimeTaken
	if response1.IP != response2.IP {
		t.Errorf("IP mismatch: %v != %v", response1.IP, response2.IP)
	}

	if response1.ValidIP != response2.ValidIP {
		t.Errorf("ValidIP mismatch: %v != %v", response1.ValidIP, response2.ValidIP)
	}

	if response1.BlackListed != response2.BlackListed {
		t.Errorf("BlackListed mismatch: %v != %v", response1.BlackListed, response2.BlackListed)
	}

	if response1.Status != response2.Status {
		t.Errorf("Status mismatch: %v != %v", response1.Status, response2.Status)
	}

	// Compare BlackList maps
	if len(response1.BlackList) != len(response2.BlackList) {
		t.Errorf("BlackList length mismatch: %v != %v", len(response1.BlackList), len(response2.BlackList))
	}

	for bl, ips1 := range response1.BlackList {
		ips2, found := response2.BlackList[bl]
		if !found {
			t.Errorf("Blacklist %s found in first response but not in second", bl)
			continue
		}

		if len(ips1) != len(ips2) {
			t.Errorf("IP count mismatch for blacklist %s: %v != %v", bl, len(ips1), len(ips2))
			continue
		}

		// Compare IPs (order might differ, so we check for presence)
		for _, ip1 := range ips1 {
			found := false
			for _, ip2 := range ips2 {
				if ip1.Equal(ip2) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("IP %v found in first response but not in second for blacklist %s", ip1, bl)
			}
		}
	}

	// Compare Errors
	if len(response1.Errors) != len(response2.Errors) {
		t.Errorf("Errors length mismatch: %v != %v", len(response1.Errors), len(response2.Errors))
	}

	t.Logf("Cache consistency verified for IP %s: BlackListed=%v, Cached: %v->%v",
		testIP, response1.BlackListed, response1.Cached, response2.Cached)
}

// Test to verify cache deletion for domain blacklist checks
// Verifies that after cache deletion, the next request is not from cache
func TestDomainBlacklistCacheDeletion(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	testDomain := "github.com"

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Domain
	json.NewDecoder(rr2.Body).Decode(&response2)

	if !response2.Cached {
		t.Logf("Warning: Second request was not cached (unexpected)")
	}

	// Clear cache
	reqClear, _ := http.NewRequest("GET", "/clear-cache/"+testDomain, nil)
	rrClear := httptest.NewRecorder()
	r.ServeHTTP(rrClear, reqClear)

	if rrClear.Code != http.StatusOK {
		t.Fatalf("Cache clear request failed with status %v", rrClear.Code)
	}

	var clearResponse ClearCache
	err := json.NewDecoder(rrClear.Body).Decode(&clearResponse)
	if err != nil {
		t.Fatalf("Failed to decode clear cache response: %v", err)
	}

	if !clearResponse.Status {
		t.Errorf("Expected cache clear to succeed, got Status=false. Errors: %v", clearResponse.Errors)
	}

	// Third request - should NOT be cached after deletion
	req3, _ := http.NewRequest("GET", "/domain/"+testDomain, nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("Third request failed with status %v", rr3.Code)
	}

	var response3 Domain
	err = json.NewDecoder(rr3.Body).Decode(&response3)
	if err != nil {
		t.Fatalf("Failed to decode third response: %v", err)
	}

	// Verify third response is NOT from cache
	if response3.Cached {
		t.Errorf("Expected third request to NOT be cached after deletion (Cached=false), got Cached=true")
	}

	// Verify domain is still valid and checked correctly
	if !response3.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	if !response3.Status {
		t.Errorf("Expected Status to be true, got false. Errors: %v", response3.Errors)
	}

	t.Logf("Cache deletion verified for domain %s: Cached before deletion=%v, Cached after deletion=%v",
		testDomain, response2.Cached, response3.Cached)
}

// Test to verify cache deletion for IP blacklist checks
// Verifies that after cache deletion, the next request is not from cache
func TestIPBlacklistCacheDeletion(t *testing.T) {
	setupTestWithResolver()

	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	testIP := "127.0.0.4"

	// First request - populate cache
	req1, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %v", rr1.Code)
	}

	// Second request - should be cached
	req2, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %v", rr2.Code)
	}

	var response2 Ip
	json.NewDecoder(rr2.Body).Decode(&response2)

	if !response2.Cached {
		t.Logf("Warning: Second request was not cached (unexpected)")
	}

	// Clear cache
	reqClear, _ := http.NewRequest("GET", "/clear-cache/"+testIP, nil)
	rrClear := httptest.NewRecorder()
	r.ServeHTTP(rrClear, reqClear)

	if rrClear.Code != http.StatusOK {
		t.Fatalf("Cache clear request failed with status %v", rrClear.Code)
	}

	var clearResponse ClearCache
	err := json.NewDecoder(rrClear.Body).Decode(&clearResponse)
	if err != nil {
		t.Fatalf("Failed to decode clear cache response: %v", err)
	}

	if !clearResponse.Status {
		t.Errorf("Expected cache clear to succeed, got Status=false. Errors: %v", clearResponse.Errors)
	}

	// Third request - should NOT be cached after deletion
	req3, _ := http.NewRequest("GET", "/ip/"+testIP, nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("Third request failed with status %v", rr3.Code)
	}

	var response3 Ip
	err = json.NewDecoder(rr3.Body).Decode(&response3)
	if err != nil {
		t.Fatalf("Failed to decode third response: %v", err)
	}

	// Verify third response is NOT from cache
	if response3.Cached {
		t.Errorf("Expected third request to NOT be cached after deletion (Cached=false), got Cached=true")
	}

	// Verify IP is still valid and checked correctly
	if !response3.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	if !response3.Status {
		t.Errorf("Expected Status to be true, got false. Errors: %v", response3.Errors)
	}

	t.Logf("Cache deletion verified for IP %s: Cached before deletion=%v, Cached after deletion=%v",
		testIP, response2.Cached, response3.Cached)
}
