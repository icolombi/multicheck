package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/dchest/validator"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
)

// Structure to generate the start log in JSON format
type StartLog struct {
	CurrentTime time.Time
	Redis       bool // Is Redis available at the start?
	ListenPort  string
	Errors      []string
}

// Structure to generate the log in JSON format
type Log struct {
	CurrentTime      time.Time
	HTTPMethod       string // HTTP method (GET, POST, PUT, etc.)
	HTTPStatusCode   int    // HTTP response status code
	Method           string // Endpoint path
	Param            string
	RequestBody      string // JSON body for POST requests
	Errors           []string
	MemoryAlloc      uint64  // Memory allocated
	NumGC            uint32  // Garbage Collection
	TimeTaken        float64 // Time taken to execute the request
	Cached           bool    // Was found in cache?
	ClientIP         string  // Client IP address
	Redis            bool    // Is Redis available?
	RedisConnections int     // Count Redis Active connections
}

// Structure to hold configuration settings
type Config struct {
	domainBlacklist       []string
	ipBlacklist           []string
	CacheControlMaxAge    int
	RedisCacheTTL         int
	MaxCustomBlacklists   int
	MaxCustomNameservers  int
	MaxRequestBodySize    int64
	MaxStringLength       int
	DNSQueryTimeout       int
	HTTPReadTimeout       int
	HTTPWriteTimeout      int
	HTTPIdleTimeout       int // Max time a keep-alive connection can be idle before closing
	HTTPReadHeaderTimeout int // Max time to read request headers (defends against slow-header attacks)
	nameServers           []string
	listenPort            string
	RedisHost             string
	RedisPort             int
	RedisDatabase         int
	RedisPassword         string
	RedisMaxIdle          int // Idle connections kept in the pool
	RedisMaxActive        int // Upper bound on concurrent Redis connections
	RedisConnTimeout      int // Connect/read/write timeout for Redis, in seconds
	// Intervals of the background monitors that keep the Redis PING and
	// runtime.ReadMemStats off the request path, in seconds.
	RedisHealthCheckInterval int
	MemStatsInterval         int
}

// Struct to represent the response of an IP object
type Ip struct {
	IP          string              // The input IP
	ValidIP     bool                // Is the IP valid?
	BlackListed bool                // Is the IP blacklisted?
	Status      bool                // Is the check valid?
	BlackList   map[string][]net.IP // List of blacklists that affect the IP
	Errors      []string            // List of errors
	TimeTaken   float64             // Time taken
	Cached      bool                // From Redis?
	CacheKey    string              // Redis cache key used
}

// Struct for the request body of POST /ip/check
type CheckIpRequest struct {
	IP          string   `json:"ip"`
	Blacklists  []string `json:"blacklists"`
	Nameservers []string `json:"nameservers,omitempty"`
}

// Struct to represent the response of a Domain object
type Domain struct {
	Domain      string              // The input Domain
	ValidDomain bool                // Is the Domain valid?
	BlackListed bool                // Is the Domain blacklisted?
	Status      bool                // Is the check valid?
	BlackList   map[string][]net.IP // List of blacklists that affect the IP
	Errors      []string            // List of errors
	TimeTaken   float64             // Time taken
	Cached      bool                // From Redis?
	CacheKey    string              // Redis cache key used
}

// Struct for the request body of POST /domain/check
type CheckDomainRequest struct {
	Domain      string   `json:"domain"`
	Blacklists  []string `json:"blacklists"`
	Nameservers []string `json:"nameservers,omitempty"`
}

// Struct to represent the response of a DelCache object
type ClearCache struct {
	Status    bool
	Key       string
	Errors    []string
	TimeTaken float64
}

// Struct for the health check object
type Health struct {
	Alive            bool
	Redis            bool
	RedisConnections int
	CachedItems      int
	Uptime           time.Duration
	GoVersion        string
	Version          string
	MemoryAlloc      uint64
}

// Struct for the root object (Help, main page)
type Root struct {
	EndPoints       []string
	DomainBlacklist []string
	IpBlacklist     []string
	RedisCacheTTL   int
	NameServers     []string
	ListenPort      string
}

// Variable to hold the start log
var startLog StartLog

// Variable to hold configuration settings
var configuration Config

var c *redis.Pool

var resolver *net.Resolver

var nameservers []string

var startTime = time.Now()

// version can be set during build with -ldflags "-X main.version=x.y.z"
var version = "1.5.0"

func main() {
	// Start time, used to calculate uptime

	configuration = ReadConfig(configuration)

	// Initialize Redis connection after loading configuration
	c = redisConnect()

	// Initialize the router
	r := mux.NewRouter()

	// API Endpoints
	//r.HandleFunc("/items", GetItems).Methods("GET")
	// r.HandleFunc("/items/{id}", GetItem).Methods("GET")
	// r.HandleFunc("/items", CreateItem).Methods("POST")
	// r.HandleFunc("/items/{id}", UpdateItem).Methods("PUT")
	// r.HandleFunc("/items/{id}", DeleteItem).Methods("DELETE")

	r.HandleFunc("/", RootHandler).Methods("GET")
	r.HandleFunc("/health", HealthCheckHandler).Methods("GET")
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	r.HandleFunc("/ip/check", PostCheckIp).Methods("POST")
	r.HandleFunc("/domain/check", PostCheckDomain).Methods("POST")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	// Define a custom resolver to use different name servers than system defaults
	// List of name servers
	nameservers = configuration.nameServers

	// With no nameservers configured, fall back to the system resolver
	// (/etc/resolv.conf): in Kubernetes this is the cluster DNS.
	if len(nameservers) == 0 {
		log.Println("no nameservers configured, using system default resolver")
		resolver = net.DefaultResolver
	} else {
		customResolver, err := createCustomResolver(nameservers)
		if err != nil {
			log.Fatalf("main: cannot build the DNS resolver: %v", err)
		}
		resolver = customResolver
	}

	// Start the background monitors and take the first Redis reading from them,
	// so no request has to pay for a PING or for runtime.ReadMemStats.
	startBackgroundMonitors()
	redisAvailable, _, redisErrorMsg := redisStatus()
	startLog.Redis = redisAvailable
	if !redisAvailable {
		startLog.Errors = append(startLog.Errors, redisErrorMsg)
	}

	// Start the server
	//fmt.Println("Server listening...")
	u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
	if err != nil {
		log.Printf("main: failed to marshal startup log: %v", err)
	} else {
		fmt.Println(string(u))
	}

	// Configure HTTP server with timeouts to prevent resource exhaustion
	srv := &http.Server{
		Addr:              configuration.listenPort,
		Handler:           r,
		ReadTimeout:       time.Duration(configuration.HTTPReadTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(configuration.HTTPReadHeaderTimeout) * time.Second,
		WriteTimeout:      time.Duration(configuration.HTTPWriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(configuration.HTTPIdleTimeout) * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB max header size
	}
	err = srv.ListenAndServe()

	if err != nil {
		startLog.Errors = append(startLog.Errors, err.Error())
		u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
		fmt.Println(string(u))
		log.Fatalln("There's an error with the server", err)

	} else {

		fmt.Println("Avviato")
		u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
		if err != nil {
			log.Printf("main: failed to marshal post-start log: %v", err)
		} else {
			fmt.Println(string(u))
		}
	}

}

// Root handler for /
func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	clientIP := r.RemoteAddr

	var endpointsList []string
	endpointsList = append(endpointsList, "GET /ip/<ip>", "GET /domain/<domain>", "POST /ip/check", "POST /domain/check", "GET /health", "GET /clear-cache/<object-name>")

	// Built per request: a package-level variable would be written concurrently
	// by simultaneous clients.
	endpoints := Root{EndPoints: endpointsList,
		DomainBlacklist: configuration.domainBlacklist,
		IpBlacklist:     configuration.ipBlacklist,
		RedisCacheTTL:   configuration.RedisCacheTTL,
		NameServers:     configuration.nameServers,
		ListenPort:      configuration.listenPort}
	json.NewEncoder(w).Encode(endpoints)

	// Log
	var errors []string
	logRequest("GET", http.StatusOK, "/", "", "", errors, 0, false, clientIP, false, 0)
}

// Health check handler
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {

	var errors []string
	var health Health
	w.Header().Set("Content-Type", "application/json")

	// Check Redis status. This endpoint pings live, on purpose: unlike the other
	// handlers it exists to report the current state, not to serve from cache.
	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		health.Redis = false
		errors = append(errors, redisErrorMessage(reply, err))

	} else {
		health.Redis = true
		health.RedisConnections = getRedisConnections()
		health.CachedItems = getRedisKeysCount()
	}
	// time.Since already returns a Duration: converting Seconds() into one
	// reinterpreted the seconds count as nanoseconds.
	uptime := time.Since(startTime)
	health.Alive = true
	memAlloc, _ := MemUsage()
	health = Health{Alive: health.Alive, Redis: health.Redis, RedisConnections: health.RedisConnections, CachedItems: health.CachedItems, Uptime: uptime, GoVersion: runtime.Version(), Version: version, MemoryAlloc: memAlloc}
	json.NewEncoder(w).Encode(health)
	// Log
	logRequest("GET", http.StatusOK, "/health", "", "", errors, 0, false, "", health.Redis, health.RedisConnections)
}

// Function to get IP information
func GetIp(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	// Errors from the DNS fan-out, kept apart from the response-wide list so a
	// partial result is never written to the cache.
	var checkErrors []string
	var ip Ip

	redisAvailable, redisConnections, redisErrorMsg := redisStatus()
	if !redisAvailable {
		errors = append(errors, redisErrorMsg)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	params := mux.Vars(r)
	ip.Cached = false
	ip.Status = true
	ip.BlackListed = false
	ip.BlackList = nil
	ipAddress := params["ip"]

	// Validate IP length
	if len(ipAddress) > configuration.MaxStringLength {
		ip.ValidIP = false
		ip.Status = false
		errors = append(errors, fmt.Sprintf("IP address too long (maximum %d characters)", configuration.MaxStringLength))
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: ipAddress, ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("GET", http.StatusBadRequest, "/ip", ipAddress, "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	} else if net.ParseIP(params["ip"]) != nil {
		ip.ValidIP = true

		// Look for it in Redis cache. Skipped entirely when Redis is down.
		if redisAvailable {
			if value, cacheErr := getRedisKey(ipAddress); cacheErr == nil {
				// If the cached entry is corrupted, fall through to a fresh DNS
				// check rather than serving malformed data.
				if unmarshalErr := json.Unmarshal([]byte(value), &ip); unmarshalErr != nil {
					errors = append(errors, "cache entry corrupted, re-checking DNS")
				} else {
					ip.Cached = true
				}
			}
		}

		// Not served from cache: check the blacklists.
		if !ip.Cached {
			ip.BlackListed, ip.BlackList, checkErrors = checkBlacklistIP(ipAddress)
			// Append, never assign: overwriting would discard the Redis errors
			// collected above.
			errors = append(errors, checkErrors...)
		}

	} else {
		// If IP is not valid, set the variable to False
		ip.ValidIP = false
		ip.Status = false
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: params["ip"], ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("GET", http.StatusBadRequest, "/ip", params["ip"], "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}
	elapsed := time.Since(start).Seconds()
	ip = Ip{TimeTaken: elapsed, IP: params["ip"],
		Cached:      ip.Cached,
		ValidIP:     ip.ValidIP,
		BlackListed: ip.BlackListed,
		BlackList:   ip.BlackList,
		Status:      ip.Status,
		CacheKey:    ipAddress,
		Errors:      errors}
	//fmt.Println(json.Marshal(ip))
	// Save to Redis only for a valid IP whose check completed without errors:
	// caching a partial result would serve it for the whole TTL.
	if !ip.Cached && ip.ValidIP && redisAvailable && len(checkErrors) == 0 {
		value, _ := json.Marshal(ip)
		valueStr := string(value)
		err := setRedisKey(ipAddress, valueStr)
		if err != nil {
			errors = append(errors, string(err.Error()))
		}
	}

	json.NewEncoder(w).Encode(ip)

	// Log
	logRequest("GET", http.StatusOK, "/ip", params["ip"], "", errors, elapsed, ip.Cached, clientIP, redisAvailable, redisConnections)
}

// Function to get domain information
func GetDomain(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	// Errors from the DNS fan-out, kept apart from the response-wide list so a
	// partial result is never written to the cache.
	var checkErrors []string
	var domain Domain

	redisAvailable, redisConnections, redisErrorMsg := redisStatus()
	if !redisAvailable {
		errors = append(errors, redisErrorMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	params := mux.Vars(r)
	domain.Cached = false
	domain.Status = true
	domain.BlackListed = false
	domain.BlackList = nil
	domainName := params["domain"]

	// Validate domain length
	if len(domainName) > configuration.MaxStringLength {
		domain.ValidDomain = false
		domain.Status = false
		errors = append(errors, fmt.Sprintf("domain name too long (maximum %d characters)", configuration.MaxStringLength))
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: domainName, ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("GET", http.StatusBadRequest, "/domain", domainName, "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	} else if validator.IsValidDomain(params["domain"]) {
		domain.ValidDomain = true

		// Look for it in Redis cache. Skipped entirely when Redis is down.
		if redisAvailable {
			if value, cacheErr := getRedisKey(domainName); cacheErr == nil {
				// If the cached entry is corrupted, fall through to a fresh DNS
				// check rather than serving malformed data.
				if unmarshalErr := json.Unmarshal([]byte(value), &domain); unmarshalErr != nil {
					errors = append(errors, "cache entry corrupted, re-checking DNS")
				} else {
					domain.Cached = true
				}
			}
		}

		// Not served from cache: check the blacklists.
		if !domain.Cached {
			domain.BlackListed, domain.BlackList, checkErrors = checkBlacklistDomain(domainName)
			// Append, never assign: overwriting would discard the Redis errors
			// collected above.
			errors = append(errors, checkErrors...)
		}

	} else {
		domain.ValidDomain = false
		domain.Status = false
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: params["domain"], ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("GET", http.StatusBadRequest, "/domain", params["domain"], "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	elapsed := time.Since(start).Seconds()
	domain = Domain{TimeTaken: elapsed, Domain: params["domain"],
		Cached:      domain.Cached,
		ValidDomain: domain.ValidDomain,
		BlackListed: domain.BlackListed,
		BlackList:   domain.BlackList,
		Status:      domain.Status,
		CacheKey:    domainName,
		Errors:      errors}

	// Save to Redis only for a valid domain whose check completed without errors:
	// caching a partial result would serve it for the whole TTL.
	if !domain.Cached && domain.ValidDomain && redisAvailable && len(checkErrors) == 0 {
		value, _ := json.Marshal(domain)
		valueStr := string(value)
		err := setRedisKey(domainName, valueStr)
		if err != nil {
			errors = append(errors, string(err.Error()))
		}
	}
	json.NewEncoder(w).Encode(domain)
	// Log
	logRequest("GET", http.StatusOK, "/domain", params["domain"], "", errors, elapsed, domain.Cached, clientIP, redisAvailable, redisConnections)
}

// POST handler to check IP against custom blacklists
func PostCheckIp(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	var req CheckIpRequest
	var ip Ip

	// Check Redis
	redisAvailable, redisConnections, redisErrorMsg := redisStatus()
	if !redisAvailable {
		errors = append(errors, redisErrorMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))

	ip.Cached = false
	ip.Status = true
	ip.BlackListed = false
	ip.BlackList = nil

	// Limit request body size to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, configuration.MaxRequestBodySize)

	// Leggi il body per il logging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		ip.Status = false
		errors = append(errors, "failed to read request body: "+err.Error())
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", "", "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}
	requestBody := string(bodyBytes)

	// Decode JSON body
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&req)
	if err != nil {
		ip.Status = false
		errors = append(errors, "invalid JSON body: "+err.Error())
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate IP length
	if len(req.IP) > configuration.MaxStringLength {
		ip.ValidIP = false
		ip.Status = false
		errors = append(errors, fmt.Sprintf("IP address too long (maximum %d characters)", configuration.MaxStringLength))
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate IP
	if net.ParseIP(req.IP) == nil {
		ip.ValidIP = false
		ip.Status = false
		errors = append(errors, "invalid IP address format")
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}
	ip.ValidIP = true

	// Validate blacklists
	valid, errorMsg := validateBlacklists(req.Blacklists, configuration.MaxCustomBlacklists)
	if !valid {
		ip.Status = false
		errors = append(errors, errorMsg)
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: true, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate nameservers if provided
	valid, errorMsg = validateNameservers(req.Nameservers, configuration.MaxCustomNameservers)
	if !valid {
		ip.Status = false
		errors = append(errors, errorMsg)
		elapsed := time.Since(start).Seconds()
		ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: true, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ip)
		logRequest("POST", http.StatusBadRequest, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Generate cache key for POST request
	cacheKey := buildPostCacheKey("ip", req.IP, req.Blacklists)

	// Check Redis cache first. Skipped entirely when Redis is down.
	if redisAvailable {
		if value, cacheErr := getRedisKey(cacheKey); cacheErr == nil {
			// Found in cache — only serve it if the entry deserialises correctly.
			// A corrupted entry falls through to a fresh DNS check.
			if unmarshalErr := json.Unmarshal([]byte(value), &ip); unmarshalErr == nil {
				ip.Cached = true
				ip.CacheKey = cacheKey
				elapsed := time.Since(start).Seconds()
				ip.TimeTaken = elapsed
				json.NewEncoder(w).Encode(ip)
				logRequest("POST", http.StatusOK, "/ip/check", req.IP, requestBody, ip.Errors, elapsed, true, clientIP, redisAvailable, redisConnections)
				return
			}
			// Unmarshal failed: log and fall through to DNS check
			errors = append(errors, "cache entry corrupted, re-checking DNS")
		}
	}

	// Not in cache, create resolver and check blacklists
	var customResolver *net.Resolver
	if len(req.Nameservers) > 0 {
		customResolver, err = createCustomResolver(req.Nameservers)
		if err != nil {
			// Nameservers are validated above, so this is an internal invariant
			// violation rather than a client mistake.
			errors = append(errors, err.Error())
			elapsed := time.Since(start).Seconds()
			ip = Ip{TimeTaken: elapsed, IP: req.IP, ValidIP: true, Status: false, Errors: errors}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ip)
			logRequest("POST", http.StatusInternalServerError, "/ip/check", req.IP, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
			return
		}
	}

	// Check custom blacklists. Append, never assign: overwriting would discard
	// the Redis errors collected above.
	var checkErrors []string
	ip.BlackListed, ip.BlackList, checkErrors = checkBlacklistIPWithCustomList(req.IP, req.Blacklists, customResolver)
	errors = append(errors, checkErrors...)

	elapsed := time.Since(start).Seconds()
	ip = Ip{
		TimeTaken:   elapsed,
		IP:          req.IP,
		Cached:      false,
		ValidIP:     true,
		BlackListed: ip.BlackListed,
		BlackList:   ip.BlackList,
		Status:      true,
		CacheKey:    cacheKey,
		Errors:      errors,
	}

	// Save to cache only when the check completed without errors: caching a
	// partial result would serve it for the whole TTL.
	if redisAvailable && len(checkErrors) == 0 {
		valueToCache, _ := json.Marshal(ip)
		valueStr := string(valueToCache)
		if err = setRedisKey(cacheKey, valueStr); err != nil {
			ip.Errors = append(ip.Errors, "cache save error: "+err.Error())
		}
	}

	json.NewEncoder(w).Encode(ip)
	logRequest("POST", http.StatusOK, "/ip/check", req.IP, requestBody, ip.Errors, elapsed, false, clientIP, redisAvailable, redisConnections)
}

// POST handler to check domains against custom blacklists
func PostCheckDomain(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	var req CheckDomainRequest
	var domain Domain

	// Check Redis
	redisAvailable, redisConnections, redisErrorMsg := redisStatus()
	if !redisAvailable {
		errors = append(errors, redisErrorMsg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))

	domain.Cached = false
	domain.Status = true
	domain.BlackListed = false
	domain.BlackList = nil

	// Limit request body size to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, configuration.MaxRequestBodySize)

	// Leggi il body per il logging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		domain.Status = false
		errors = append(errors, "failed to read request body: "+err.Error())
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", "", "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}
	requestBody := string(bodyBytes)

	// Decode JSON body
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&req)
	if err != nil {
		domain.Status = false
		errors = append(errors, "invalid JSON body: "+err.Error())
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate domain length
	if len(req.Domain) > configuration.MaxStringLength {
		domain.ValidDomain = false
		domain.Status = false
		errors = append(errors, fmt.Sprintf("domain name too long (maximum %d characters)", configuration.MaxStringLength))
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate domain
	if !validator.IsValidDomain(req.Domain) {
		domain.ValidDomain = false
		domain.Status = false
		errors = append(errors, "invalid domain format")
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: false, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}
	domain.ValidDomain = true

	// Validate blacklists
	valid, errorMsg := validateBlacklists(req.Blacklists, configuration.MaxCustomBlacklists)
	if !valid {
		domain.Status = false
		errors = append(errors, errorMsg)
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: true, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Validate nameservers if provided
	valid, errorMsg = validateNameservers(req.Nameservers, configuration.MaxCustomNameservers)
	if !valid {
		domain.Status = false
		errors = append(errors, errorMsg)
		elapsed := time.Since(start).Seconds()
		domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: true, Status: false, Errors: errors}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain)
		logRequest("POST", http.StatusBadRequest, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
		return
	}

	// Generate cache key for POST request
	cacheKey := buildPostCacheKey("domain", req.Domain, req.Blacklists)

	// Check Redis cache first. Skipped entirely when Redis is down.
	if redisAvailable {
		if value, cacheErr := getRedisKey(cacheKey); cacheErr == nil {
			// Found in cache — only serve it if the entry deserialises correctly.
			// A corrupted entry falls through to a fresh DNS check.
			if unmarshalErr := json.Unmarshal([]byte(value), &domain); unmarshalErr == nil {
				domain.Cached = true
				domain.CacheKey = cacheKey
				elapsed := time.Since(start).Seconds()
				domain.TimeTaken = elapsed
				json.NewEncoder(w).Encode(domain)
				logRequest("POST", http.StatusOK, "/domain/check", req.Domain, requestBody, domain.Errors, elapsed, true, clientIP, redisAvailable, redisConnections)
				return
			}
			// Unmarshal failed: log and fall through to DNS check
			errors = append(errors, "cache entry corrupted, re-checking DNS")
		}
	}

	// Not in cache, create resolver and check blacklists
	var customResolver *net.Resolver
	if len(req.Nameservers) > 0 {
		customResolver, err = createCustomResolver(req.Nameservers)
		if err != nil {
			// Nameservers are validated above, so this is an internal invariant
			// violation rather than a client mistake.
			errors = append(errors, err.Error())
			elapsed := time.Since(start).Seconds()
			domain = Domain{TimeTaken: elapsed, Domain: req.Domain, ValidDomain: true, Status: false, Errors: errors}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(domain)
			logRequest("POST", http.StatusInternalServerError, "/domain/check", req.Domain, requestBody, errors, elapsed, false, clientIP, redisAvailable, redisConnections)
			return
		}
	}

	// Check custom blacklists. Append, never assign: overwriting would discard
	// the Redis errors collected above.
	var checkErrors []string
	domain.BlackListed, domain.BlackList, checkErrors = checkBlacklistDomainWithCustomList(req.Domain, req.Blacklists, customResolver)
	errors = append(errors, checkErrors...)

	elapsed := time.Since(start).Seconds()
	domain = Domain{
		TimeTaken:   elapsed,
		Domain:      req.Domain,
		Cached:      false,
		ValidDomain: true,
		BlackListed: domain.BlackListed,
		BlackList:   domain.BlackList,
		Status:      true,
		CacheKey:    cacheKey,
		Errors:      errors,
	}

	// Save to cache only when the check completed without errors: caching a
	// partial result would serve it for the whole TTL.
	if redisAvailable && len(checkErrors) == 0 {
		valueToCache, _ := json.Marshal(domain)
		valueStr := string(valueToCache)
		if err = setRedisKey(cacheKey, valueStr); err != nil {
			domain.Errors = append(domain.Errors, "cache save error: "+err.Error())
		}
	}

	json.NewEncoder(w).Encode(domain)
	logRequest("POST", http.StatusOK, "/domain/check", req.Domain, requestBody, domain.Errors, elapsed, false, clientIP, redisAvailable, redisConnections)
}

// Helper function for logging (DRY)
func logRequest(httpMethod string, httpStatusCode int, method string, param string, requestBody string, errors []string, elapsed float64, cached bool, clientIP string, redisAvailable bool, redisConnections int) {
	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{
		CurrentTime:      time.Now(),
		HTTPMethod:       httpMethod,
		HTTPStatusCode:   httpStatusCode,
		Method:           method,
		Param:            param,
		RequestBody:      requestBody,
		Errors:           errors,
		MemoryAlloc:      memAlloc,
		NumGC:            numGC,
		TimeTaken:        elapsed,
		Cached:           cached,
		ClientIP:         clientIP,
		Redis:            redisAvailable,
		RedisConnections: redisConnections,
	}, "", "   ")

	// Do not panic on log marshal failure: a logging error must never crash the server.
	if err != nil {
		log.Printf("logRequest: failed to marshal log entry: %v", err)
		return
	}
	fmt.Println(string(u))
}

// Clear cache
func DelCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	var clearCache ClearCache

	redisAvailable, redisConnections, redisErrorMsg := redisStatus()
	if !redisAvailable {
		errors = append(errors, redisErrorMsg)
	}
	params := mux.Vars(r)
	key := params["key"]
	err := delRedisKey(key)
	if err != nil {
		errors = append(errors, string(err.Error()))
		clearCache.Status = false
	} else {
		clearCache.Status = true
	}
	elapsed := time.Since(start).Seconds()
	clearCache = ClearCache{TimeTaken: elapsed, Key: key, Errors: errors, Status: clearCache.Status}

	json.NewEncoder(w).Encode(clearCache)

	// Log
	logRequest("GET", http.StatusOK, "/clear-cache", key, "", errors, elapsed, false, clientIP, redisAvailable, redisConnections)
}
