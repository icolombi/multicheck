package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
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

	viper.SetConfigName("config.toml") // config file name without extension
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

	return configuration
}

// Function to get memory usage
func MemUsage() (memAlloc uint64, numGC uint32) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memAlloc = bToKb(m.Alloc)
	numGC = m.NumGC
	return memAlloc, numGC
}

func bToKb(b uint64) uint64 {
	return b / 1024
}

// Function to remove IPs 127.0.0.1 and 127.255.255.255 from a slice
func removeIPFromSlice(slice []net.IP) []net.IP {
	var result []net.IP

	for _, ip := range slice {
		if !ip.Equal(net.ParseIP("127.0.0.1")) &&
			!ip.Equal(net.ParseIP("127.255.255.255")) {
			result = append(result, ip)
		}
	}
	return result
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
func createCustomResolver(nameservers []string) *net.Resolver {
	// Defensive guard: POST handlers validate nameservers before calling this,
	// but an empty slice would cause rand.Intn(0) to panic.
	if len(nameservers) == 0 {
		log.Fatal("createCustomResolver: nameservers list is empty")
	}
	return &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			// Prendo un name server a caso
			randomIndex := rand.Intn(len(nameservers))
			nameserver := nameservers[randomIndex]
			return d.DialContext(ctx, "udp", net.JoinHostPort(nameserver, "53"))
		},
	}
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
	value = removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}
	if len(value) != 0 {
		//fmt.Print("Blacklisted A: ")
		//fmt.Println(value)
		*blacklisted = true
		// Protect shared map write
		mu.Lock()
		blacklistsActive[blacklist] = value
		mu.Unlock()
	}
	errorCh <- error
}

// Function to convert an IP address to a reverse domain name (i.e. "127.0.0.1" > "1.0.0.127")
func reverseIP(ipAddress string) string {
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
	value = removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}

	if len(value) != 0 {

		*blacklisted = true
		// Protect shared map write
		mu.Lock()
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

// setRedisKey set a key to Redis
func setRedisKey(key string, value string) error {

	conn := c.Get()
	defer conn.Close()

	conn.Send("SET", key, value)
	conn.Send("EXPIRE", key, configuration.RedisCacheTTL)
	conn.Flush()
	conn.Receive()
	_, err := conn.Receive()
	if err != nil {
		return err
	}
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
