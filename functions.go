package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"
)

// Reads variables from config file
func ReadConfig(c Config) (configuration Config) {
	configPath := "."

	cp, ok := os.LookupEnv("GSS_CONFIG_PATH")

	if ok {
		configPath = cp
	}

	// Viper expects the config name *without* the extension; passing "config.toml"
	// only worked by accident because SetConfigType made it fall back to an
	// exact-name lookup.
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(configPath)
	viper.AutomaticEnv() // read value ENV variable

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("Fatal error config file: \n", err)
		os.Exit(1)
	}
	configuration.domainBlacklist = viper.GetStringSlice("domainBlacklist")
	configuration.ipBlacklist = viper.GetStringSlice("ipBlacklist")
	configuration.CacheControlMaxAge = viper.GetInt("cacheControlMaxAge")
	configuration.RedisCacheTTL = viper.GetInt("redisCacheTTL")
	configuration.MaxCustomBlacklists = viper.GetInt("maxCustomBlacklists")
	configuration.MaxCustomNameservers = viper.GetInt("maxCustomNameservers")
	configuration.MaxRequestBodySize = viper.GetInt64("maxRequestBodySize")
	configuration.MaxStringLength = viper.GetInt("maxStringLength")
	configuration.DNSQueryTimeout = viper.GetInt("dnsQueryTimeout")
	configuration.HTTPReadTimeout = viper.GetInt("httpReadTimeout")
	configuration.HTTPWriteTimeout = viper.GetInt("httpWriteTimeout")
	configuration.HTTPIdleTimeout = viper.GetInt("httpIdleTimeout")
	configuration.HTTPReadHeaderTimeout = viper.GetInt("httpReadHeaderTimeout")
	configuration.nameServers = viper.GetStringSlice("nameServers")
	configuration.listenPort = viper.GetString("listenPort")
	configuration.RedisHost = viper.GetString("redisHost")
	configuration.RedisPort = viper.GetInt("redisPort")
	configuration.RedisDatabase = viper.GetInt("redisDatabase")
	configuration.RedisPassword = viper.GetString("redisPassword")
	configuration.RedisMaxIdle = viper.GetInt("redisMaxIdle")
	configuration.RedisMaxActive = viper.GetInt("redisMaxActive")
	configuration.RedisConnTimeout = viper.GetInt("redisConnTimeout")
	configuration.RedisHealthCheckInterval = viper.GetInt("redisHealthCheckInterval")
	configuration.MemStatsInterval = viper.GetInt("memStatsInterval")
	configuration.TrustProxyHeaders = viper.GetBool("trustProxyHeaders")

	// Apply defaults for the optional tuning parameters, so configuration files
	// written before these keys existed keep working unchanged.
	applyConfigDefaults(&configuration)

	return configuration
}

// applyConfigDefaults fills in every optional tuning parameter, keeping older
// config.toml files valid.
//
// This must cover all of them, not just the most recent additions: a missing key
// reads back as zero, and several of those zeros silently disable the service
// rather than degrading it. maxStringLength = 0 rejects every IP and domain as
// "too long", dnsQueryTimeout = 0 hands an already-expired context to every
// lookup, maxRequestBodySize = 0 rejects every POST body, redisCacheTTL = 0 makes
// SET ... EX fail so nothing is ever cached, and the httpXxxTimeout values simply
// turn the corresponding protections off.
func applyConfigDefaults(c *Config) {
	if c.CacheControlMaxAge <= 0 {
		c.CacheControlMaxAge = 3600
	}
	if c.RedisCacheTTL <= 0 {
		c.RedisCacheTTL = 300
	}
	if c.MaxCustomBlacklists <= 0 {
		c.MaxCustomBlacklists = 20
	}
	if c.MaxCustomNameservers <= 0 {
		c.MaxCustomNameservers = 3
	}
	if c.MaxRequestBodySize <= 0 {
		c.MaxRequestBodySize = 1048576 // 1 MB
	}
	if c.MaxStringLength <= 0 {
		c.MaxStringLength = 253 // DNS name length limit
	}
	if c.DNSQueryTimeout <= 0 {
		c.DNSQueryTimeout = 5
	}
	if c.HTTPReadTimeout <= 0 {
		c.HTTPReadTimeout = 30
	}
	if c.HTTPWriteTimeout <= 0 {
		c.HTTPWriteTimeout = 30
	}
	if c.HTTPIdleTimeout <= 0 {
		c.HTTPIdleTimeout = 60
	}
	if c.HTTPReadHeaderTimeout <= 0 {
		c.HTTPReadHeaderTimeout = 10
	}
	if c.listenPort == "" {
		// Without this the server would silently bind :80.
		c.listenPort = ":8080"
	}
	if c.RedisHost == "" {
		c.RedisHost = "127.0.0.1"
	}
	if c.RedisPort <= 0 {
		c.RedisPort = 6379
	}
	if c.RedisMaxIdle <= 0 {
		c.RedisMaxIdle = 8
	}
	if c.RedisMaxActive <= 0 {
		c.RedisMaxActive = 64
	}
	if c.RedisConnTimeout <= 0 {
		c.RedisConnTimeout = 2
	}
	if c.RedisHealthCheckInterval <= 0 {
		c.RedisHealthCheckInterval = 5
	}
	if c.MemStatsInterval <= 0 {
		c.MemStatsInterval = 10
	}
}

// validateConfig rejects a configuration that would produce a broken runtime.
// It runs at startup only, so failing loudly here is preferable to serving
// nonsense results: an invalid nameserver would otherwise build a resolver that
// fails every single lookup.
func validateConfig(c *Config) error {
	if len(c.ipBlacklist) == 0 && len(c.domainBlacklist) == 0 {
		return fmt.Errorf("both ipBlacklist and domainBlacklist are empty: nothing can be checked")
	}
	// An empty nameServers list is legitimate: it selects the system resolver.
	if len(c.nameServers) > 0 {
		if valid, errMsg := validateNameservers(c.nameServers, len(c.nameServers)); !valid {
			return fmt.Errorf("invalid nameServers: %s", errMsg)
		}
	}
	return nil
}

// Cached process and Redis state, refreshed by the background monitors.
// Sampling out of band keeps runtime.ReadMemStats (which stops the world) and
// the Redis PING round-trip off the request path.
var (
	monitorsOnce     sync.Once
	memAllocCached   atomic.Uint64
	numGCCached      atomic.Uint32
	redisUp          atomic.Bool
	redisConnsCached atomic.Int64
	redisLastErr     atomic.Value // string
)

// startBackgroundMonitors samples memory statistics and Redis availability on a
// timer so handlers can read both without paying their cost per request.
// Safe to call more than once: only the first call starts the goroutines.
func startBackgroundMonitors() {
	monitorsOnce.Do(func() {
		// Prime the caches so the very first request sees real values.
		sampleMemStats()
		refreshRedisStatus()

		go func() {
			ticker := time.NewTicker(time.Duration(configuration.MemStatsInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				sampleMemStats()
			}
		}()

		go func() {
			ticker := time.NewTicker(time.Duration(configuration.RedisHealthCheckInterval) * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				refreshRedisStatus()
			}
		}()
	})
}

// sampleMemStats refreshes the cached memory figures. runtime.ReadMemStats
// stops the world, which is why it must never run inside a handler.
func sampleMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memAllocCached.Store(bToKb(m.Alloc))
	numGCCached.Store(m.NumGC)
}

// refreshRedisStatus pings Redis once and stores the outcome for the handlers.
func refreshRedisStatus() {
	reply, err := pingRedis()
	redisConnsCached.Store(int64(c.ActiveCount()))
	if err == nil && reply == "PONG" {
		redisUp.Store(true)
		redisLastErr.Store("")
		return
	}
	redisUp.Store(false)
	redisLastErr.Store(redisErrorMessage(reply, err))
}

// redisErrorMessage describes a failed PING. err is nil when Redis answers with
// an unexpected reply, so it must never be dereferenced unconditionally.
func redisErrorMessage(reply string, err error) string {
	if err != nil {
		return "Redis: " + err.Error()
	}
	return fmt.Sprintf("Redis: unexpected PING reply %q", reply)
}

// redisStatus returns the cached availability, active connection count and, when
// Redis is down, the message describing the last failure.
func redisStatus() (available bool, connections int, errorMsg string) {
	available = redisUp.Load()
	connections = int(redisConnsCached.Load())
	if !available {
		if msg, ok := redisLastErr.Load().(string); ok {
			errorMsg = msg
		}
	}
	return available, connections, errorMsg
}

// MemUsage returns the latest memory sample taken by the background monitor.
func MemUsage() (memAlloc uint64, numGC uint32) {
	return memAllocCached.Load(), numGCCached.Load()
}

func bToKb(b uint64) uint64 {
	return b / 1024
}

// dnsblSentinelCodes are replies that do not mean "listed".
//
// 127.0.0.1 and 127.255.255.255 are long-standing false positives. The
// 127.255.255.252-254 range is how Spamhaus (and others) signal that the query
// itself was refused — typically because the resolver is a public one, or is over
// its query quota. Counting those as a listing produces confident false positives
// exactly when the service is being rate limited.
var dnsblSentinelCodes = []string{
	"127.0.0.1",
	"127.255.255.252",
	"127.255.255.253",
	"127.255.255.254",
	"127.255.255.255",
}

// isDNSBLSentinel reports whether a DNSBL reply is a sentinel rather than a listing.
func isDNSBLSentinel(ip net.IP) bool {
	for _, code := range dnsblSentinelCodes {
		if ip.Equal(net.ParseIP(code)) {
			return true
		}
	}
	return false
}

// removeIPFromSlice drops the DNSBL sentinel replies from a lookup result,
// returning the remaining (genuine) listings and the sentinels that were removed
// so the caller can report a refused query instead of silently dropping it.
func removeIPFromSlice(slice []net.IP) (result []net.IP, sentinels []net.IP) {
	for _, ip := range slice {
		if isDNSBLSentinel(ip) {
			sentinels = append(sentinels, ip)
			continue
		}
		result = append(result, ip)
	}
	return result, sentinels
}

// isQueryRefused reports whether the sentinel replies indicate the DNSBL rejected
// the query (as opposed to the harmless 127.0.0.1 / 127.255.255.255 noise).
func isQueryRefused(sentinels []net.IP) bool {
	for _, ip := range sentinels {
		if ip.Equal(net.ParseIP("127.255.255.252")) ||
			ip.Equal(net.ParseIP("127.255.255.253")) ||
			ip.Equal(net.ParseIP("127.255.255.254")) {
			return true
		}
	}
	return false
}

// Validates a list of nameservers (must be valid IPs)
func validateNameservers(nameservers []string, maxAllowed int) (valid bool, errorMsg string) {
	// If the list is empty, it's valid (we'll use defaults)
	if len(nameservers) == 0 {
		return true, ""
	}

	// Check maximum limit
	if len(nameservers) > maxAllowed {
		return false, fmt.Sprintf("too many nameservers: maximum %d allowed, received %d", maxAllowed, len(nameservers))
	}

	// Validate that each nameserver is a valid IP
	for _, ns := range nameservers {
		// Check that it's not empty or whitespace only
		trimmed := strings.TrimSpace(ns)
		if trimmed == "" {
			return false, "nameserver entries cannot be empty or whitespace"
		}

		// IP validation
		if net.ParseIP(trimmed) == nil {
			return false, fmt.Sprintf("invalid nameserver: '%s' is not a valid IP address", trimmed)
		}
	}

	return true, ""
}

// Validates a list of blacklists for DNS syntax and limits
func validateBlacklists(blacklists []string, maxAllowed int) (valid bool, errorMsg string) {
	// Check for empty list
	if len(blacklists) == 0 {
		return false, "blacklist array cannot be empty"
	}

	// Check maximum limit
	if len(blacklists) > maxAllowed {
		return false, fmt.Sprintf("too many blacklists: maximum %d allowed, received %d", maxAllowed, len(blacklists))
	}

	// Validate DNS syntax for each blacklist
	for _, bl := range blacklists {
		// Check that it's not empty or whitespace only
		trimmed := strings.TrimSpace(bl)
		if trimmed == "" {
			return false, "blacklist entries cannot be empty or whitespace"
		}

		// Check maximum length (DNS standard: 253 characters)
		if len(trimmed) > configuration.MaxStringLength {
			return false, fmt.Sprintf("blacklist name too long: '%s' (maximum %d characters)", trimmed, configuration.MaxStringLength)
		}

		// Basic DNS format validation
		// Must contain at least one dot and valid characters
		if !strings.Contains(trimmed, ".") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (must be a valid DNS name)", trimmed)
		}

		// Check for invalid DNS characters
		// DNS allows: letters, numbers, hyphen and dot
		for _, char := range trimmed {
			if !((char >= 'a' && char <= 'z') ||
				(char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '.') {
				return false, fmt.Sprintf("invalid blacklist format: '%s' (contains invalid DNS characters)", trimmed)
			}
		}

		// Check that it doesn't start or end with dot or hyphen
		if strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") ||
			strings.HasPrefix(trimmed, "-") || strings.HasSuffix(trimmed, "-") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (cannot start or end with . or -)", trimmed)
		}

		// Check for consecutive dots
		if strings.Contains(trimmed, "..") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (cannot contain consecutive dots)", trimmed)
		}
	}

	return true, ""
}

// Creates a custom resolver with the specified nameservers
func createCustomResolver(nameservers []string) (*net.Resolver, error) {
	// Defensive guard: POST handlers validate nameservers before calling this,
	// but an empty slice would cause rand.Intn(0) to panic.
	if len(nameservers) == 0 {
		return nil, fmt.Errorf("createCustomResolver: nameservers list is empty")
	}
	return &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Pick a random nameserver from the configured list.
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			// Forward the network requested by the resolver: it retries over TCP
			// when a UDP answer comes back truncated, and hardcoding "udp" here
			// would silently defeat that retry.
			return d.DialContext(ctx, network, net.JoinHostPort(nameserver, "53"))
		},
	}, nil
}

// Given an IP, returns its presence in a list of blacklists
func checkBlacklistIP(ipAddress string) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {
	return checkBlacklistIPWithCustomList(ipAddress, configuration.ipBlacklist, nil)
}

// Given an IP and a custom list of blacklists, returns its presence
// If customResolver is nil, uses the global resolver
func checkBlacklistIPWithCustomList(ipAddress string, blackLists []string, customResolver *net.Resolver) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {

	// If no custom resolver is specified, use the global one
	resolverToUse := resolver
	if customResolver != nil {
		resolverToUse = customResolver
	}

	max := len(blackLists)
	reverseIP := reverseIP(ipAddress)
	blacklistsActive = make(map[string][]net.IP)
	blacklisted = false
	// WaitGroup and Mutex to protect concurrent map access
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkIPDNS(&wg, &mu, blacklist, reverseIP, blacklistsActive, errorCh, &blacklisted, resolverToUse)

	}

	// Wait for all goroutines to finish, then drain the entire error channel.
	// Reading after wg.Wait() ensures all goroutines have already sent their
	// results, so we collect every error instead of just the first one.
	wg.Wait()
	close(errorCh)
	for errMsg := range errorCh {
		if errMsg != "" {
			errorList = append(errorList, errMsg)
		}
	}

	return blacklisted, blacklistsActive, errorList
}

// Function for DNS queries on IP
func checkIPDNS(wg *sync.WaitGroup, mu *sync.Mutex, blacklist string, reverseIP string, blacklistsActive map[string][]net.IP, errorCh chan string, blacklisted *bool, resolverToUse *net.Resolver) {
	//fmt.Println("Checking " + blacklist)
	defer wg.Done()
	var error string

	// Create context with timeout to prevent DNS queries from hanging
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuration.DNSQueryTimeout)*time.Second)
	defer cancel()

	value, err := resolverToUse.LookupIP(ctx, "ip4", reverseIP+"."+blacklist+".")
	value, sentinels := removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}
	// A refused query is reported as an error rather than being silently dropped:
	// the result for this blacklist is unknown, not "clean".
	if isQueryRefused(sentinels) {
		error = fmt.Sprintf("%s refused the query (resolver blocked or over quota)", blacklist)
	}
	if len(value) != 0 {
		//fmt.Print("Blacklisted A: ")
		//fmt.Println(value)
		// Protect both shared writes: several goroutines reach this branch
		// concurrently, so the flag needs the same lock as the map.
		mu.Lock()
		*blacklisted = true
		blacklistsActive[blacklist] = value
		mu.Unlock()
	}
	errorCh <- error
}

// isIPv4 reports whether the address is an IPv4 address usable for DNSBL queries.
//
// net.ParseIP alone is not enough: it accepts IPv6, which reverseIP cannot handle
// and which none of the configured blacklist zones serve. Callers must reject
// anything this rejects, otherwise the query is built from an unreversed address
// and the resulting "not listed" answer is meaningless.
func isIPv4(ipAddress string) bool {
	parsed := net.ParseIP(ipAddress)
	return parsed != nil && parsed.To4() != nil
}

// Function to convert an IPv4 address to a reverse domain name (i.e. "127.0.0.1" > "1.0.0.127").
// Returns an empty string for anything that is not IPv4, so a caller that skipped
// validation produces no query rather than a nonsensical one.
func reverseIP(ipAddress string) string {
	if !isIPv4(ipAddress) {
		return ""
	}
	parts := strings.Split(ipAddress, ".")
	reversedParts := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		reversedParts[i] = parts[len(parts)-1-i]
	}
	return strings.Join(reversedParts, ".")
}

// Given a domain, returns its presence in a list of blacklists
func checkBlacklistDomain(domainName string) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {
	return checkBlacklistDomainWithCustomList(domainName, configuration.domainBlacklist, nil)
}

// Given a domain and a custom list of blacklists, returns its presence
// If customResolver is nil, uses the global resolver
func checkBlacklistDomainWithCustomList(domainName string, blackLists []string, customResolver *net.Resolver) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {

	// If no custom resolver is specified, use the global one
	resolverToUse := resolver
	if customResolver != nil {
		resolverToUse = customResolver
	}

	max := len(blackLists)
	blacklistsActive = make(map[string][]net.IP)
	blacklisted = false
	// WaitGroup and Mutex to protect concurrent map access
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkDomainDNS(&wg, &mu, blacklist, domainName, blacklistsActive, errorCh, &blacklisted, resolverToUse)
	}

	// Wait for all goroutines to finish, then drain the entire error channel.
	// Reading after wg.Wait() ensures all goroutines have already sent their
	// results, so we collect every error instead of just the first one.
	wg.Wait()
	close(errorCh)
	for errMsg := range errorCh {
		if errMsg != "" {
			errorList = append(errorList, errMsg)
		}
	}

	return blacklisted, blacklistsActive, errorList
}

// Function for DNS queries on domain
func checkDomainDNS(wg *sync.WaitGroup, mu *sync.Mutex, blacklist string, domainName string, blacklistsActive map[string][]net.IP, errorCh chan string, blacklisted *bool, resolverToUse *net.Resolver) {
	defer wg.Done() // Decrease the WaitGroup counter by 1
	var error string

	//value, err := net.LookupIP(domainName + "." + blacklist)

	// Debug DNS query execution time
	//start := time.Now()

	// Create context with timeout to prevent DNS queries from hanging
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(configuration.DNSQueryTimeout)*time.Second)
	defer cancel()

	value, err := resolverToUse.LookupIP(ctx, "ip4", domainName+"."+blacklist+".")
	value, sentinels := removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}
	// A refused query is reported as an error rather than being silently dropped:
	// the result for this blacklist is unknown, not "clean".
	if isQueryRefused(sentinels) {
		error = fmt.Sprintf("%s refused the query (resolver blocked or over quota)", blacklist)
	}

	if len(value) != 0 {
		// Protect both shared writes: several goroutines reach this branch
		// concurrently, so the flag needs the same lock as the map.
		mu.Lock()
		*blacklisted = true
		blacklistsActive[blacklist] = value
		mu.Unlock()
	}

	// Debug DNS query execution time
	// elapsed := time.Since(start).Seconds()
	//fmt.Println(blacklist)
	//fmt.Println(elapsed)
	errorCh <- error
}

// Redis ping
func pingRedis() (reply string, err error) {
	conn := c.Get()
	defer conn.Close()
	reply, err = redis.String(conn.Do("PING"))
	return reply, err
}

func getRedisConnections() (reply int) {
	count := c.ActiveCount()
	return count
}

// getRedisKeysCount returns the number of keys in the current Redis database
func getRedisKeysCount() (count int) {
	conn := c.Get()
	defer conn.Close()

	// DBSIZE returns the number of keys in the currently selected database
	count, err := redis.Int(conn.Do("DBSIZE"))
	if err != nil {
		return 0
	}
	return count
}

// getRedisKey get a key from Redis
func getRedisKey(key string) (reply string, err error) {

	conn := c.Get()
	defer conn.Close()

	reply, err = redis.String(conn.Do("GET", key))

	return reply, err
}

// setRedisKey set a key to Redis with its expiration
func setRedisKey(key string, value string) error {

	conn := c.Get()
	defer conn.Close()

	// SET ... EX writes the value and its TTL in a single atomic command: the
	// previous SET + EXPIRE pipeline could leave a key without expiration and
	// discarded the error returned by SET.
	_, err := conn.Do("SET", key, value, "EX", configuration.RedisCacheTTL)

	return err
}

// delRedisKey delete a key from Redis
func delRedisKey(key string) (err error) {

	conn := c.Get()
	defer conn.Close()

	_, err = conn.Do("DEL", key)

	return err
}

// generateBlacklistHash creates a SHA256 hash of sorted blacklists (truncated to 16 chars)
// Used to generate cache keys for POST endpoints with custom blacklists
func generateBlacklistHash(blacklists []string) string {
	// Sort blacklists to ensure consistent hash regardless of order
	sortedBlacklists := make([]string, len(blacklists))
	copy(sortedBlacklists, blacklists)
	sort.Strings(sortedBlacklists)

	// Create hash from sorted blacklists
	data := strings.Join(sortedBlacklists, ",")
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])

	// Truncate to 16 characters for readability
	return hashStr[:16]
}

// buildPostCacheKey creates a Redis cache key for POST endpoints
// Format: post:ip:<ip>:<hash> or post:domain:<domain>:<hash>
func buildPostCacheKey(keyType, identifier string, blacklists []string) string {
	hash := generateBlacklistHash(blacklists)
	return fmt.Sprintf("post:%s:%s:%s", keyType, identifier, hash)
}
