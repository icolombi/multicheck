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

// Test per verificare che GET /ip restituisca 400 con IP non valido
func TestGetIpInvalid(t *testing.T) {
	configuration = ReadConfig(configuration)
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	req, _ := http.NewRequest("GET", "/ip/192.168.8.111111", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test per verificare che GET /domain restituisca 400 con dominio non valido
func TestGetDomainInvalid(t *testing.T) {
	configuration = ReadConfig(configuration)
	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	req, _ := http.NewRequest("GET", "/domain/invalid..domain", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

// Test per verificare che POST /ip/check restituisca 400 con IP non valido
func TestPostCheckIpInvalidIP(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /ip/check restituisca 400 con blacklist vuota
func TestPostCheckIpEmptyBlacklists(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /ip/check restituisca 400 con troppe blacklist
func TestPostCheckIpTooManyBlacklists(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /ip/check restituisca 400 con nameserver non valido
func TestPostCheckIpInvalidNameservers(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /domain/check restituisca 400 con dominio non valido
func TestPostCheckDomainInvalidDomain(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /domain/check restituisca 400 con blacklist vuota
func TestPostCheckDomainEmptyBlacklists(t *testing.T) {
	configuration = ReadConfig(configuration)
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

// Test per verificare che POST /domain/check restituisca 400 con JSON malformato
func TestPostCheckDomainInvalidJSON(t *testing.T) {
	configuration = ReadConfig(configuration)
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
