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

	// Create request for 2.0.0.127
	req, err := http.NewRequest("GET", "/ip/2.0.0.127", nil)
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
		t.Errorf("Expected 2.0.0.127 to be blacklisted, but it was not detected as blacklisted")
	}

	// Verify zen.spamhaus.org is in the blacklist list
	blacklistIPs, found := response.BlackList["zen.spamhaus.org"]
	if !found {
		t.Errorf("Expected 2.0.0.127 to be blacklisted by zen.spamhaus.org, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Verify response code is 127.0.0.11
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
