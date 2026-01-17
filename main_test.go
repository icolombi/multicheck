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

func TestHealthCheckHandler(t *testing.T) {

	// Struct per contenere la risposta
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
	// Inizializza la configurazione
	configuration = ReadConfig(configuration)

	// Inizializza il resolver
	nameservers = configuration.nameServers
	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	resolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Prendo un name server a caso dalla configurazione
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}

	// Crea il router con il handler
	r := mux.NewRouter()
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")

	// Crea la richiesta per test.uribl.com
	req, err := http.NewRequest("GET", "/domain/test.uribl.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Crea un ResponseRecorder per registrare la risposta
	rr := httptest.NewRecorder()

	// Esegui la richiesta
	r.ServeHTTP(rr, req)

	// Controlla lo status code
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

	// Verifica che il dominio sia valido
	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	// Verifica che il dominio sia blacklistato
	if !response.BlackListed {
		t.Errorf("Expected test.uribl.com to be blacklisted, but it was not detected as blacklisted")
	}

	// Verifica che multi.uribl.com sia nella lista delle blacklist
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

	// Verifica che non ci siano errori
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

func TestIPBlacklist(t *testing.T) {
	// Inizializza la configurazione
	configuration = ReadConfig(configuration)

	// Inizializza il resolver
	nameservers = configuration.nameServers
	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	resolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Prendo un name server a caso dalla configurazione
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}

	// Crea il router con il handler
	r := mux.NewRouter()
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")

	// Crea la richiesta per 2.0.0.127
	req, err := http.NewRequest("GET", "/ip/2.0.0.127", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Crea un ResponseRecorder per registrare la risposta
	rr := httptest.NewRecorder()

	// Esegui la richiesta
	r.ServeHTTP(rr, req)

	// Controlla lo status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decodifica la risposta JSON
	var response Ip
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verifica che l'IP sia valido
	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	// Verifica che l'IP sia blacklistato
	if !response.BlackListed {
		t.Errorf("Expected 2.0.0.127 to be blacklisted, but it was not detected as blacklisted")
	}

	// Verifica che zen.spamhaus.org sia nella lista delle blacklist
	blacklistIPs, found := response.BlackList["zen.spamhaus.org"]
	if !found {
		t.Errorf("Expected 2.0.0.127 to be blacklisted by zen.spamhaus.org, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Verifica che il codice di risposta sia 127.0.0.11
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

	// Verifica che non ci siano errori
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

func TestPostCheckDomain(t *testing.T) {
	// Inizializza la configurazione
	configuration = ReadConfig(configuration)

	// Inizializza il resolver
	nameservers = configuration.nameServers
	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	resolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Prendo un name server a caso dalla configurazione
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}

	// Crea il router con il handler
	r := mux.NewRouter()
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")

	// Prepara il body della richiesta
	requestBody := CheckDomainRequest{
		Domain:     "test.uribl.com",
		Blacklists: []string{"multi.uribl.com"},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}

	// Crea la richiesta POST
	req, err := http.NewRequest("POST", "/domain/check", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Crea un ResponseRecorder per registrare la risposta
	rr := httptest.NewRecorder()

	// Esegui la richiesta
	r.ServeHTTP(rr, req)

	// Controlla lo status code
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

	// Verifica che il dominio sia valido
	if !response.ValidDomain {
		t.Errorf("Expected ValidDomain to be true, got false")
	}

	// Verifica che il dominio sia blacklistato
	if !response.BlackListed {
		t.Errorf("Expected test.uribl.com to be blacklisted, but it was not detected as blacklisted")
	}

	// Verifica che multi.uribl.com sia nella lista delle blacklist
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

	// Verifica che non ci siano errori
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}

func TestPostCheckIP(t *testing.T) {
	// Inizializza la configurazione
	configuration = ReadConfig(configuration)

	// Inizializza il resolver
	nameservers = configuration.nameServers
	if len(nameservers) == 0 {
		t.Fatal("No nameservers configured in config.toml")
	}

	resolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Prendo un name server a caso dalla configurazione
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}

	// Crea il router con il handler
	r := mux.NewRouter()
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")

	// Prepara il body della richiesta
	requestBody := CheckIpRequest{
		IP:         "2.0.0.127",
		Blacklists: []string{"zen.spamhaus.org"},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}

	// Crea la richiesta POST
	req, err := http.NewRequest("POST", "/ip/check", bytes.NewBuffer(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Crea un ResponseRecorder per registrare la risposta
	rr := httptest.NewRecorder()

	// Esegui la richiesta
	r.ServeHTTP(rr, req)

	// Controlla lo status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Decodifica la risposta JSON
	var response Ip
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verifica che l'IP sia valido
	if !response.ValidIP {
		t.Errorf("Expected ValidIP to be true, got false")
	}

	// Verifica che l'IP sia blacklistato
	if !response.BlackListed {
		t.Errorf("Expected 2.0.0.127 to be blacklisted, but it was not detected as blacklisted")
	}

	// Verifica che zen.spamhaus.org sia nella lista delle blacklist
	blacklistIPs, found := response.BlackList["zen.spamhaus.org"]
	if !found {
		t.Errorf("Expected 2.0.0.127 to be blacklisted by zen.spamhaus.org, but it was not found in BlackList map")
		t.Logf("BlackList content: %+v", response.BlackList)
	} else {
		// Verifica che il codice di risposta sia 127.0.0.11
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

	// Verifica che non ci siano errori
	if len(response.Errors) > 0 {
		t.Logf("Errors reported: %v", response.Errors)
	}
}
