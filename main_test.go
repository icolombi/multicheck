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
