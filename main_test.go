package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
