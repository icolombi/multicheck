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
		{"GET-endpoint IP key", "ip:1.2.3.4"},
		{"GET-endpoint domain key", "domain:example.com"},
		{"bare IP (pre-1.6.16 clients)", "1.2.3.4"},
		{"bare domain (pre-1.6.16 clients)", "example.com"},
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

func TestResolveCacheKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantKey string
		wantOK  bool
	}{
		{"namespaced IP", "ip:1.2.3.4", "ip:1.2.3.4", true},
		{"namespaced domain", "domain:example.com", "domain:example.com", true},
		{"bare IPv4 maps to the IP namespace", "1.2.3.4", "ip:1.2.3.4", true},
		{"bare domain maps to the domain namespace", "Example.COM", "domain:example.com", true},
		{"post IP prefix", "post:ip:1.2.3.4:0123456789abcdef", "post:ip:1.2.3.4:0123456789abcdef", true},
		{"post domain prefix", "post:domain:example.com:0123456789abcdef", "post:domain:example.com:0123456789abcdef", true},
		{"ip namespace with a domain", "ip:example.com", "", false},
		{"domain namespace with an IP", "domain:1.2.3.4", "", false},
		{"foreign key", "session:abc", "", false},
		{"wildcard", "*", "", false},
		{"empty", "", "", false},
		{"over the length limit", strings.Repeat("a", 300) + ".com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotOK := resolveCacheKey(tt.key)
			if gotOK != tt.wantOK || (tt.wantOK && gotKey != tt.wantKey) {
				t.Errorf("resolveCacheKey(%q) = (%q, %v), want (%q, %v)", tt.key, gotKey, gotOK, tt.wantKey, tt.wantOK)
			}
		})
	}
}

// TestDomainEndpointsRejectIPv4 covers the cache poisoning path: an IPv4 address
// is a syntactically valid domain name, and checking it as one used to write a
// domain-list verdict that the IP endpoint then served.
func TestDomainEndpointsRejectIPv4(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{"GET", http.MethodGet, "/domain/1.2.3.4", nil},
		{"POST", http.MethodPost, "/domain/check", []byte(`{"domain":"1.2.3.4","blacklists":["multi.uribl.com"]}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, tt.method, tt.target, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			var domain Domain
			if err := json.Unmarshal(rec.Body.Bytes(), &domain); err != nil {
				t.Fatalf("cannot decode response: %v", err)
			}
			if !containsSubstring(domain.Errors, "IP endpoints") {
				t.Errorf("Errors = %v, want a pointer to the IP endpoints", domain.Errors)
			}
		})
	}
}

// TestGetDomainInvalidReportsReason covers the 400 that came with an empty Errors list.
func TestGetDomainInvalidReportsReason(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/domain/not_a_domain", nil)

	var domain Domain
	if err := json.Unmarshal(rec.Body.Bytes(), &domain); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	if len(domain.Errors) == 0 {
		t.Error("Errors is empty on a rejected domain")
	}
}

func TestRootListsClearCacheAsDelete(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/", nil)

	var root Root
	if err := json.Unmarshal(rec.Body.Bytes(), &root); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}
	for _, endpoint := range root.EndPoints {
		if strings.Contains(endpoint, "clear-cache") && !strings.HasPrefix(endpoint, "DELETE ") {
			t.Errorf("endpoint list advertises %q, want DELETE", endpoint)
		}
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

// TestGetWithEmptyBlacklistAnswers503 covers a GET endpoint with no default
// blacklist: the fan-out queried nothing and answered a confident "not listed".
func TestGetWithEmptyBlacklistAnswers503(t *testing.T) {
	originalIP, originalDomain := configuration.ipBlacklist, configuration.domainBlacklist
	defer func() { configuration.ipBlacklist, configuration.domainBlacklist = originalIP, originalDomain }()
	configuration.ipBlacklist, configuration.domainBlacklist = nil, nil

	for _, target := range []string{"/ip/1.2.3.4", "/domain/example.com"} {
		t.Run(target, func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, target, nil)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}
			if header := rec.Header().Get("Cache-Control"); header != "" {
				t.Errorf("Cache-Control = %q on a 503, want it unset", header)
			}
			var body struct {
				BlackListed bool
				Status      bool
				Errors      []string
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("cannot decode response: %v", err)
			}
			if body.BlackListed || body.Status {
				t.Errorf("BlackListed=%v Status=%v, want both false", body.BlackListed, body.Status)
			}
			if !containsSubstring(body.Errors, "no ") || !containsSubstring(body.Errors, "/check") {
				t.Errorf("Errors = %v, want a pointer to the POST endpoint", body.Errors)
			}
		})
	}
}
