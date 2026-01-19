package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// setupTest initializes configuration and Redis pool for tests
func setupTest() {
	if configuration.listenPort == "" {
		configuration = ReadConfig(configuration)
		c = redisConnect()
	}
}

// Test to verify that GET /ip returns 400 with invalid IP
func TestGetIpInvalid(t *testing.T) {
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
	setupTest()
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
