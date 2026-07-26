package main

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"plain IPv4", "1.2.3.4", true},
		{"loopback", "127.0.0.1", true},
		{"broadcast", "255.255.255.255", true},
		{"IPv6 full", "2001:db8::1", false},
		{"IPv6 loopback", "::1", false},
		{"IPv4-mapped IPv6 is still IPv4", "::ffff:1.2.3.4", true},
		{"octet out of range", "1.2.3.256", false},
		{"trailing garbage", "1x.2.3.4", false},
		{"too few octets", "1.2.3", false},
		{"empty", "", false},
		{"domain name", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPv4(tt.input); got != tt.want {
				t.Errorf("isIPv4(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestReverseIP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"typical address", "1.2.3.4", "4.3.2.1"},
		{"loopback", "127.0.0.1", "1.0.0.127"},
		{"palindrome", "1.1.1.1", "1.1.1.1"},
		// IPv6 has no valid IPv4-style reversal. Returning the address unchanged
		// used to build a nonsensical DNSBL query that answered "not listed".
		{"IPv6 yields nothing", "2001:db8::1", ""},
		{"IPv6 loopback yields nothing", "::1", ""},
		{"invalid address yields nothing", "not-an-ip", ""},
		{"empty yields nothing", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reverseIP(tt.input); got != tt.want {
				t.Errorf("reverseIP(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRemoveIPFromSlice(t *testing.T) {
	tests := []struct {
		name          string
		input         []string
		wantListings  []string
		wantSentinels []string
	}{
		{
			name:         "genuine listings are kept",
			input:        []string{"127.0.0.2", "127.0.0.11"},
			wantListings: []string{"127.0.0.2", "127.0.0.11"},
		},
		{
			name:          "classic false positives are dropped",
			input:         []string{"127.0.0.1", "127.255.255.255"},
			wantSentinels: []string{"127.0.0.1", "127.255.255.255"},
		},
		{
			name:          "refusal codes are dropped, not counted as listings",
			input:         []string{"127.255.255.252", "127.255.255.253", "127.255.255.254"},
			wantSentinels: []string{"127.255.255.252", "127.255.255.253", "127.255.255.254"},
		},
		{
			name:          "mixed reply keeps only the listing",
			input:         []string{"127.0.0.1", "127.0.0.4", "127.255.255.254"},
			wantListings:  []string{"127.0.0.4"},
			wantSentinels: []string{"127.0.0.1", "127.255.255.254"},
		},
		{
			name:  "empty reply",
			input: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listings, sentinels := removeIPFromSlice(parseIPs(t, tt.input))
			assertIPsEqual(t, "listings", listings, tt.wantListings)
			assertIPsEqual(t, "sentinels", sentinels, tt.wantSentinels)
		})
	}
}

func TestIsQueryRefused(t *testing.T) {
	tests := []struct {
		name      string
		sentinels []string
		want      bool
	}{
		{"no sentinels", nil, false},
		{"only harmless false positives", []string{"127.0.0.1", "127.255.255.255"}, false},
		{"blocked resolver", []string{"127.255.255.254"}, true},
		{"over quota", []string{"127.255.255.252"}, true},
		{"mixed", []string{"127.0.0.1", "127.255.255.253"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQueryRefused(parseIPs(t, tt.sentinels)); got != tt.want {
				t.Errorf("isQueryRefused(%v) = %v, want %v", tt.sentinels, got, tt.want)
			}
		})
	}
}

func TestGenerateBlacklistHash(t *testing.T) {
	base := generateBlacklistHash([]string{"a.example.org", "b.example.org"})

	t.Run("order does not matter", func(t *testing.T) {
		reordered := generateBlacklistHash([]string{"b.example.org", "a.example.org"})
		if reordered != base {
			t.Errorf("hash depends on ordering: %q vs %q", base, reordered)
		}
	})

	t.Run("different lists hash differently", func(t *testing.T) {
		other := generateBlacklistHash([]string{"a.example.org", "c.example.org"})
		if other == base {
			t.Errorf("distinct blacklists produced the same hash %q", base)
		}
	})

	t.Run("hash is the documented length", func(t *testing.T) {
		if len(base) != 16 {
			t.Errorf("hash length = %d, want 16", len(base))
		}
	})
}

func TestBuildPostCacheKey(t *testing.T) {
	blacklists := []string{"zen.spamhaus.org"}
	hash := generateBlacklistHash(blacklists)

	tests := []struct {
		name       string
		entityType string
		identifier string
		want       string
	}{
		{"ip key", "ip", "1.2.3.4", "post:ip:1.2.3.4:" + hash},
		{"domain key", "domain", "example.com", "post:domain:example.com:" + hash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildPostCacheKey(tt.entityType, tt.identifier, blacklists); got != tt.want {
				t.Errorf("buildPostCacheKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateBlacklists(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		max       int
		wantValid bool
	}{
		{"single valid entry", []string{"zen.spamhaus.org"}, 20, true},
		{"several valid entries", []string{"zen.spamhaus.org", "bl.spamcop.net"}, 20, true},
		{"empty list", nil, 20, false},
		{"over the limit", []string{"a.org", "b.org", "c.org"}, 2, false},
		{"entry without a dot", []string{"localhost"}, 20, false},
		{"entry with a leading dot", []string{".spamhaus.org"}, 20, false},
		{"entry with a trailing dot", []string{"spamhaus.org."}, 20, false},
		{"entry with consecutive dots", []string{"zen..spamhaus.org"}, 20, false},
		{"entry with an illegal character", []string{"zen spamhaus.org"}, 20, false},
		{"empty entry", []string{""}, 20, false},
		// Validation is all-or-nothing: one bad entry rejects the whole request.
		{"one invalid entry rejects the list", []string{"zen.spamhaus.org", "bad"}, 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errMsg := validateBlacklists(tt.input, tt.max)
			if valid != tt.wantValid {
				t.Errorf("validateBlacklists(%v) = %v (%q), want %v", tt.input, valid, errMsg, tt.wantValid)
			}
			if !valid && errMsg == "" {
				t.Error("rejection came back without an explanation")
			}
		})
	}
}

func TestValidateNameservers(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		max       int
		wantValid bool
	}{
		{"single valid IP", []string{"8.8.8.8"}, 3, true},
		{"several valid IPs", []string{"8.8.8.8", "1.1.1.1"}, 3, true},
		{"IPv6 resolver is a valid address", []string{"2001:4860:4860::8888"}, 3, true},
		{"over the limit", []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"}, 2, false},
		{"hostname instead of IP", []string{"dns.google"}, 3, false},
		{"malformed IP", []string{"8.8.8"}, 3, false},
		{"empty entry", []string{""}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errMsg := validateNameservers(tt.input, tt.max)
			if valid != tt.wantValid {
				t.Errorf("validateNameservers(%v) = %v (%q), want %v", tt.input, valid, errMsg, tt.wantValid)
			}
			if !valid && errMsg == "" {
				t.Error("rejection came back without an explanation")
			}
		})
	}
}

// TestApplyConfigDefaultsFillsEveryKey is the regression test for the failure mode
// where a config.toml missing the optional keys left them at zero. Several of those
// zeros do not degrade the service, they disable it outright.
func TestApplyConfigDefaultsFillsEveryKey(t *testing.T) {
	var empty Config
	applyConfigDefaults(&empty)

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"CacheControlMaxAge", empty.CacheControlMaxAge, 3600},
		{"RedisCacheTTL", empty.RedisCacheTTL, 300},
		{"MaxCustomBlacklists", empty.MaxCustomBlacklists, 20},
		{"MaxCustomNameservers", empty.MaxCustomNameservers, 3},
		{"MaxStringLength", empty.MaxStringLength, 253},
		{"DNSQueryTimeout", empty.DNSQueryTimeout, 5},
		{"HTTPReadTimeout", empty.HTTPReadTimeout, 30},
		{"HTTPWriteTimeout", empty.HTTPWriteTimeout, 30},
		{"HTTPIdleTimeout", empty.HTTPIdleTimeout, 60},
		{"HTTPReadHeaderTimeout", empty.HTTPReadHeaderTimeout, 10},
		{"RedisPort", empty.RedisPort, 6379},
		{"RedisMaxIdle", empty.RedisMaxIdle, 8},
		{"RedisMaxActive", empty.RedisMaxActive, 64},
		{"RedisConnTimeout", empty.RedisConnTimeout, 2},
		{"RedisHealthCheckInterval", empty.RedisHealthCheckInterval, 5},
		{"MemStatsInterval", empty.MemStatsInterval, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}

	if empty.MaxRequestBodySize != 1048576 {
		t.Errorf("MaxRequestBodySize = %d, want 1048576", empty.MaxRequestBodySize)
	}
	if empty.listenPort != ":8080" {
		t.Errorf("listenPort = %q, want %q — an empty value silently binds :80", empty.listenPort, ":8080")
	}
	if empty.RedisHost != "127.0.0.1" {
		t.Errorf("RedisHost = %q, want %q", empty.RedisHost, "127.0.0.1")
	}
}

func TestApplyConfigDefaultsKeepsExplicitValues(t *testing.T) {
	configured := Config{
		RedisCacheTTL:   60,
		MaxStringLength: 64,
		listenPort:      ":9999",
	}
	applyConfigDefaults(&configured)

	if configured.RedisCacheTTL != 60 {
		t.Errorf("RedisCacheTTL = %d, want the configured 60", configured.RedisCacheTTL)
	}
	if configured.MaxStringLength != 64 {
		t.Errorf("MaxStringLength = %d, want the configured 64", configured.MaxStringLength)
	}
	if configured.listenPort != ":9999" {
		t.Errorf("listenPort = %q, want the configured %q", configured.listenPort, ":9999")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:   "blacklists and no nameservers uses the system resolver",
			config: Config{ipBlacklist: []string{"zen.spamhaus.org"}},
		},
		{
			name: "valid nameservers",
			config: Config{
				domainBlacklist: []string{"multi.uribl.com"},
				nameServers:     []string{"8.8.8.8", "1.1.1.1"},
			},
		},
		{
			name:    "no blacklists at all",
			config:  Config{nameServers: []string{"8.8.8.8"}},
			wantErr: true,
		},
		{
			name: "nameserver that is not an IP",
			config: Config{
				ipBlacklist: []string{"zen.spamhaus.org"},
				nameServers: []string{"dns.google"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config
			applyConfigDefaults(&cfg)
			err := validateConfig(&cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRedisErrorMessage(t *testing.T) {
	t.Run("reports the error when there is one", func(t *testing.T) {
		msg := redisErrorMessage("", errors.New("connection refused"))
		if !strings.Contains(msg, "connection refused") {
			t.Errorf("message %q does not mention the underlying error", msg)
		}
	})

	// A PING can come back with an unexpected reply and a nil error; dereferencing
	// err there used to panic and take the server down.
	t.Run("handles an unexpected reply with a nil error", func(t *testing.T) {
		msg := redisErrorMessage("WRONGPONG", nil)
		if !strings.Contains(msg, "WRONGPONG") {
			t.Errorf("message %q does not mention the unexpected reply", msg)
		}
	})
}

func TestBToKb(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  uint64
	}{
		{"zero", 0, 0},
		{"exactly one KiB", 1024, 1},
		{"rounds down", 2047, 1},
		{"one MiB", 1048576, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bToKb(tt.bytes); got != tt.want {
				t.Errorf("bToKb(%d) = %d, want %d", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCreateCustomResolver(t *testing.T) {
	t.Run("builds a resolver from valid nameservers", func(t *testing.T) {
		got, err := createCustomResolver([]string{"8.8.8.8"})
		if err != nil {
			t.Fatalf("createCustomResolver() error = %v", err)
		}
		if got == nil {
			t.Fatal("createCustomResolver() returned a nil resolver without an error")
		}
	})

	// It must return an error rather than terminating the process: it is reachable
	// from the POST request path.
	t.Run("reports an error instead of exiting", func(t *testing.T) {
		if _, err := createCustomResolver(nil); err == nil {
			t.Error("createCustomResolver(nil) succeeded, want an error")
		}
	})
}

// parseIPs converts a table-test fixture into net.IP values.
func parseIPs(t *testing.T, addresses []string) []net.IP {
	t.Helper()
	if addresses == nil {
		return nil
	}
	parsed := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip := net.ParseIP(address)
		if ip == nil {
			t.Fatalf("test fixture %q is not a valid IP", address)
		}
		parsed = append(parsed, ip)
	}
	return parsed
}

func assertIPsEqual(t *testing.T, label string, got []net.IP, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range got {
		if !got[i].Equal(net.ParseIP(want[i])) {
			t.Errorf("%s[%d] = %v, want %v", label, i, got[i], want[i])
		}
	}
}
