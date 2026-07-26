package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// The tests in this file cover the request paths that are rejected before any DNS
// or Redis work happens, so they need neither a resolver nor a Redis server.

// newTestRouter mirrors the route table registered in main().
func newTestRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", RootHandler).Methods("GET")
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("DELETE")
	return r
}

func doRequest(t *testing.T, method, target string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)
	return rec
}

// TestGetIpRejectsIPv6 covers the case where net.ParseIP accepted an IPv6 address
// that reverseIP could not reverse, so the service reported a confident
// "not blacklisted" derived from a nonsensical query.
func TestGetIpRejectsIPv6(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/ip/2001:db8::1", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var ip Ip
	if err := json.Unmarshal(rec.Body.Bytes(), &ip); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if ip.ValidIP {
		t.Error("ValidIP = true for an IPv6 address")
	}
	if !containsSubstring(ip.Errors, "IPv6") {
		t.Errorf("Errors = %v, want an explanation mentioning IPv6", ip.Errors)
	}
}

func TestPostCheckIpRejectsIPv6(t *testing.T) {
	body, err := json.Marshal(CheckIpRequest{IP: "2001:db8::1", Blacklists: []string{"zen.spamhaus.org"}})
	if err != nil {
		t.Fatalf("cannot build the request body: %v", err)
	}

	rec := doRequest(t, http.MethodPost, "/ip/check", body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var ip Ip
	if err := json.Unmarshal(rec.Body.Bytes(), &ip); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if !containsSubstring(ip.Errors, "IPv6") {
		t.Errorf("Errors = %v, want an explanation mentioning IPv6", ip.Errors)
	}
}

// TestNoCacheControlOnRejectedRequest covers the header being set before the
// validation branches, which declared 400 responses cacheable for a whole hour.
func TestNoCacheControlOnRejectedRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{"invalid IP", http.MethodGet, "/ip/not-an-ip", nil},
		{"IPv6", http.MethodGet, "/ip/2001:db8::1", nil},
		{"invalid domain", http.MethodGet, "/domain/not_a_domain", nil},
		{"POST with an invalid IP", http.MethodPost, "/ip/check", []byte(`{"ip":"nope","blacklists":["zen.spamhaus.org"]}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, tt.method, tt.target, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if header := rec.Header().Get("Cache-Control"); header != "" {
				t.Errorf("Cache-Control = %q on a rejected request, want it unset", header)
			}
		})
	}
}

func TestClearCacheRejectsForeignKeys(t *testing.T) {
	// The endpoint is unauthenticated and deletes by exact key, so it must refuse
	// anything this service could not have written: the Redis database may be
	// shared with other applications.
	tests := []struct {
		name string
		key  string
	}{
		{"another application's key", "session:abc123"},
		{"a wildcard attempt", "*"},
		{"an arbitrary string", "some-random-key"},
		{"an underscore-laden key", "user_profile_42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, http.MethodDelete, "/clear-cache/"+tt.key, nil)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d for key %q, want %d", rec.Code, tt.key, http.StatusBadRequest)
			}

			var clearCache ClearCache
			if err := json.Unmarshal(rec.Body.Bytes(), &clearCache); err != nil {
				t.Fatalf("cannot decode response: %v", err)
			}
			if clearCache.Status {
				t.Error("Status = true for a rejected key")
			}
		})
	}
}

func TestClearCacheAcceptsOwnKeyFormats(t *testing.T) {
	// Redis is unavailable in the unit environment, so an accepted key gets as far
	// as the availability check and stops there. What matters is that it is not
	// rejected as malformed.
	tests := []struct {
		name string
		key  string
	}{
		{"GET-endpoint IP key", "1.2.3.4"},
		{"GET-endpoint domain key", "example.com"},
		{"POST-endpoint IP key", "post:ip:1.2.3.4:abcdef0123456789"},
		{"POST-endpoint domain key", "post:domain:example.com:abcdef0123456789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, http.MethodDelete, "/clear-cache/"+tt.key, nil)

			if rec.Code == http.StatusBadRequest {
				t.Errorf("key %q was rejected as malformed", tt.key)
			}
		})
	}
}

// TestClearCacheRejectsGet locks in that the destructive operation is no longer
// reachable over GET, where browser prefetch and cross-site requests can reach it.
func TestClearCacheRejectsGet(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/clear-cache/1.2.3.4", nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d for GET /clear-cache, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestClearCacheReportsRedisUnavailable(t *testing.T) {
	if available, _, _ := redisStatus(); available {
		t.Skip("this test describes the Redis-down path, but Redis is reachable")
	}

	rec := doRequest(t, http.MethodDelete, "/clear-cache/1.2.3.4", nil)

	// The handler used to answer 200 with Status:false, so a client had no way to
	// tell that nothing had been deleted.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var clearCache ClearCache
	if err := json.Unmarshal(rec.Body.Bytes(), &clearCache); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if clearCache.Status {
		t.Error("Status = true while Redis is unavailable")
	}
}

func TestIsOwnCacheKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"IPv4", "1.2.3.4", true},
		{"domain", "example.com", true},
		{"post IP prefix", "post:ip:1.2.3.4:0123456789abcdef", true},
		{"post domain prefix", "post:domain:example.com:0123456789abcdef", true},
		{"foreign key", "session:abc", false},
		{"wildcard", "*", false},
		{"empty", "", false},
		{"over the length limit", strings.Repeat("a", 300) + ".com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOwnCacheKey(tt.key); got != tt.want {
				t.Errorf("isOwnCacheKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestPostRejectsUnknownFields(t *testing.T) {
	// DisallowUnknownFields is what turns a typo into a clear 400 instead of a
	// silently ignored parameter.
	body := []byte(`{"ip":"1.2.3.4","blacklists":["zen.spamhaus.org"],"nameserver":["8.8.8.8"]}`)

	rec := doRequest(t, http.MethodPost, "/ip/check", body)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d for a body with an unknown field, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTruncateForLog(t *testing.T) {
	t.Run("short bodies are logged verbatim", func(t *testing.T) {
		body := `{"ip":"1.2.3.4"}`
		if got := truncateForLog(body); got != body {
			t.Errorf("truncateForLog(%q) = %q, want it unchanged", body, got)
		}
	})

	t.Run("long bodies are capped", func(t *testing.T) {
		got := truncateForLog(strings.Repeat("x", maxLoggedBodyBytes*2))
		if len(got) > maxLoggedBodyBytes+len("...[truncated]") {
			t.Errorf("truncated length = %d, want at most %d", len(got), maxLoggedBodyBytes+len("...[truncated]"))
		}
		if !strings.HasSuffix(got, "...[truncated]") {
			t.Error("truncated body does not carry the truncation marker")
		}
	})
}

func TestClientIPFrom(t *testing.T) {
	original := configuration.TrustProxyHeaders
	defer func() { configuration.TrustProxyHeaders = original }()

	t.Run("proxy headers are ignored by default", func(t *testing.T) {
		configuration.TrustProxyHeaders = false
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.9")

		if got := clientIPFrom(req); got != "10.0.0.1:5555" {
			t.Errorf("clientIPFrom() = %q, want the untrusted RemoteAddr", got)
		}
	})

	t.Run("first X-Forwarded-For entry wins when trusted", func(t *testing.T) {
		configuration.TrustProxyHeaders = true
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")

		if got := clientIPFrom(req); got != "203.0.113.9" {
			t.Errorf("clientIPFrom() = %q, want the originating client", got)
		}
	})

	t.Run("falls back to X-Real-IP when trusted", func(t *testing.T) {
		configuration.TrustProxyHeaders = true
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		req.Header.Set("X-Real-IP", "203.0.113.9")

		if got := clientIPFrom(req); got != "203.0.113.9" {
			t.Errorf("clientIPFrom() = %q, want the X-Real-IP value", got)
		}
	})
}

func TestInvalidIPMessage(t *testing.T) {
	t.Run("names IPv6 explicitly", func(t *testing.T) {
		if msg := invalidIPMessage("2001:db8::1"); !strings.Contains(msg, "IPv6") {
			t.Errorf("message = %q, want it to mention IPv6", msg)
		}
	})

	t.Run("reports a malformed address generically", func(t *testing.T) {
		if msg := invalidIPMessage("nonsense"); strings.Contains(msg, "IPv6") {
			t.Errorf("message = %q, want a generic invalid-format message", msg)
		}
	})
}

func containsSubstring(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}
	return false
}
