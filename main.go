package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"

	"time"

	"github.com/dchest/validator"
	"github.com/gorilla/mux"
)

// Struttura per generare il log di start in JSON
type StartLog struct {
	CurrentTime time.Time
	Redis       bool // Is Redis available at the start?
	ListenPort  string
	Errors      []string
}

// Struttura per generare il log in formato JSON
type Log struct {
	CurrentTime      time.Time
	Method           string
	Param            string
	Errors           []string
	MemoryAlloc      uint64  // Memory allocated
	NumGC            uint32  // Garbage Collection
	TimeTaken        float64 // Time taken to execute the request
	Cached           bool    // Was found in cache?
	ClientIP         string  // Client IP address
	Redis            bool    // Is Redis available?
	RedisConnections int     // Count Redis Active connections
}

// Struttura per contenere le configurazioni
type Config struct {
	domainBlacklist    []string
	ipBlacklist        []string
	CacheControlMaxAge int
	RedisCacheTTL      int
	nameServers        []string
	listenPort         string
}

// Struct per rappresentare la risposta di un oggetto di tipo IP
type Ip struct {
	IP          string              // The input IP
	ValidIP     bool                // Is the IP valid?
	BlackListed bool                // Is the IP blacklisted?
	Status      bool                // Is the check valid?
	BlackList   map[string][]net.IP // List of blacklists that affect the IP
	Errors      []string            // List of errors
	TimeTaken   float64             // Time taken
	Cached      bool                // From Redis?
}

// Struct per rappresentare la risposta un oggetto di tipo Domain
type Domain struct {
	Domain      string              // The input Domain
	ValidDomain bool                // Is the Domain valid?
	BlackListed bool                // Is the Domain blacklisted?
	Status      bool                // Is the check valid?
	BlackList   map[string][]net.IP // List of blacklists that affect the IP
	Errors      []string            // List of errors
	TimeTaken   float64             // Time taken
	Cached      bool                // From Redis?
}

// Struct per rappresentare la risposta di un oggetto DelCache
type ClearCache struct {
	Status    bool
	Key       string
	Errors    []string
	TimeTaken float64
}

// Struct per l'oggetto di health check
type Health struct {
	Alive            bool
	Redis            bool
	RedisConnections int
	Uptime           time.Duration
}

// Struct per l'oggetto di root (Help, pagina principale)
type Root struct {
	EndPoints       []string
	DomainBlacklist []string
	IpBlacklist     []string
	RedisCacheTTL   int
	NameServers     []string
	ListenPort      string
}

// Variabile per contenere lo start log
var startLog StartLog

// Variabile per contenere le configurazioni
var configuration Config

// Variabile per contenere le informazioni sull'IP
var ip Ip

// Variabile per contenere le informazioni sul dominio
var domain Domain

// Variabile per contenere le informazioni sull'healthckeck
var health Health

// Variabile per contenere le informazioni sugli endpoints
var endpoints Root

// Variabile per contenere le informazioni sul metodo ClearaCache
var clearCache ClearCache

var c = redisConnect()

var resolver *net.Resolver

var nameservers []string

var uptime time.Duration
var startTime = time.Now()

func main() {
	// Momento di avvio, usato per calcolare l'uptime

	configuration = ReadConfig(configuration)

	// Inizializza il router
	r := mux.NewRouter()

	// Endpoints della API
	//r.HandleFunc("/items", GetItems).Methods("GET")
	// r.HandleFunc("/items/{id}", GetItem).Methods("GET")
	// r.HandleFunc("/items", CreateItem).Methods("POST")
	// r.HandleFunc("/items/{id}", UpdateItem).Methods("PUT")
	// r.HandleFunc("/items/{id}", DeleteItem).Methods("DELETE")

	r.HandleFunc("/", RootHandler).Methods("GET")
	r.HandleFunc("/health", HealthCheckHandler).Methods("GET")
	r.HandleFunc("/ip/{ip}", GetIp).Methods("GET")
	r.HandleFunc("/domain/{domain}", GetDomain).Methods("GET")
	r.HandleFunc("/clear-cache/{key}", DelCache).Methods("GET")

	// Definisco un custom resolver per poter utilizzare dei name server di versi da quelli di sistema
	// Elenco dei name servers
	nameservers = configuration.nameServers

	resolver = &net.Resolver{
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

	// Check Redis availability
	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		startLog.Redis = false
		startLog.Errors = append(startLog.Errors, "Redis: "+err.Error())

	} else {
		startLog.Redis = true
	}

	// Avvia il server
	//fmt.Println("Server in ascolto...")
	u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
	err = http.ListenAndServe(configuration.listenPort, r)

	if err != nil {
		startLog.Errors = append(startLog.Errors, err.Error())
		u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
		fmt.Println(string(u))
		log.Fatalln("There's an error with the server", err)

	} else {

		fmt.Println("Avviato")
		u, err := json.MarshalIndent(StartLog{CurrentTime: time.Now(), Errors: startLog.Errors, Redis: startLog.Redis, ListenPort: configuration.listenPort}, "", "   ")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(u))
	}

}

// Funzione di root /
func RootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	clientIP := r.RemoteAddr

	var endpointsList []string
	endpointsList = append(endpointsList, "/ip/<ip>", "/domain/<domain>", "/health", "/clear-cache/<object-name>")

	endpoints = Root{EndPoints: endpointsList,
		DomainBlacklist: configuration.domainBlacklist,
		IpBlacklist:     configuration.ipBlacklist,
		RedisCacheTTL:   configuration.RedisCacheTTL,
		NameServers:     configuration.nameServers,
		ListenPort:      configuration.listenPort}
	json.NewEncoder(w).Encode(endpoints)

	// Log
	var errors []string
	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{CurrentTime: time.Now(), Method: "/", Errors: errors, MemoryAlloc: memAlloc, NumGC: numGC, ClientIP: clientIP}, "", "   ")

	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
}

// Funzione di healthcheck
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {

	var errors []string
	w.Header().Set("Content-Type", "application/json")

	// Controllo lo stato di Redis

	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		health.Redis = false
		errors = append(errors, "Redis: "+err.Error())

	} else {
		health.Redis = true
		health.RedisConnections = getRedisConnections()
	}
	uptime = time.Duration(time.Since(startTime).Seconds())
	health.Alive = true
	health = Health{Alive: health.Alive, Redis: health.Redis, RedisConnections: health.RedisConnections, Uptime: uptime}
	json.NewEncoder(w).Encode(health)
	// Log

	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{CurrentTime: time.Now(), Method: "/health", Errors: errors, MemoryAlloc: memAlloc, NumGC: numGC, Redis: health.Redis, RedisConnections: health.RedisConnections}, "", "   ")

	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
}

// Funzione per ottenere info sull'IP
func GetIp(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string

	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		health.Redis = false
		errors = append(errors, "Redis: "+err.Error())
	} else {
		health.Redis = true
		health.RedisConnections = getRedisConnections()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	params := mux.Vars(r)
	ip.Cached = false
	ip.Status = true
	ip.BlackListed = false
	ip.BlackList = nil
	ipAddress := params["ip"]
	// Se l'IP è valido, proseguo con le query DNS
	if net.ParseIP(params["ip"]) != nil {
		ip.ValidIP = true
		// Cerco in Redis se è in cache
		value, err := getRedisKey(ipAddress)

		// Se non è in cache controllo le blacklist
		if err != nil {

			ip.BlackListed, ip.BlackList, errors = checkBlacklistIP(ipAddress)

			// Se è in cache prendo i dati da Redis
		} else {
			//fmt.Println(value)
			json.Unmarshal([]byte(value), &ip)
			ip.Cached = true
		}

	} else {
		// Se l'IP non è valido imposto a False la relativa variabile
		ip.ValidIP = false
		ip.Status = false
	}
	elapsed := time.Since(start).Seconds()
	ip = Ip{TimeTaken: elapsed, IP: params["ip"],
		Cached:      ip.Cached,
		ValidIP:     ip.ValidIP,
		BlackListed: ip.BlackListed,
		BlackList:   ip.BlackList,
		Status:      ip.Status,
		Errors:      errors}
	//fmt.Println(json.Marshal(ip))
	// Se l'IP non è in cache creo la chiave in Redis
	if !ip.Cached {
		value, _ := json.Marshal(ip)
		valueStr := string(value)
		err := setRedisKey(ipAddress, valueStr)
		if err != nil {
			errors = append(errors, string(err.Error()))
		}
	}

	json.NewEncoder(w).Encode(ip)

	// Log
	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{CurrentTime: time.Now(), Method: "/ip", Param: params["ip"], Errors: errors, MemoryAlloc: memAlloc, NumGC: numGC, TimeTaken: elapsed, Cached: ip.Cached, ClientIP: clientIP, Redis: health.Redis, RedisConnections: health.RedisConnections}, "", "   ")

	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
}

// Funzione per ottenere info su un dominio
func GetDomain(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		health.Redis = false
		errors = append(errors, "Redis: "+err.Error())

	} else {
		health.Redis = true
		health.RedisConnections = getRedisConnections()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(configuration.CacheControlMaxAge))
	params := mux.Vars(r)
	domain.Cached = false
	domain.Status = true
	domain.BlackListed = false
	domain.BlackList = nil
	domainName := params["domain"]
	// Se il dominio è valido, proseguo con le query DNS
	if validator.IsValidDomain(params["domain"]) {
		domain.ValidDomain = true
		// Cerco in Redis se è in cache
		value, err := getRedisKey(domainName)
		// Se non è in cache controllo le blacklist
		if err != nil {
			domain.BlackListed, domain.BlackList, errors = checkBlacklistDomain(domainName)
		} else {
			json.Unmarshal([]byte(value), &domain)
			domain.Cached = true
		}

	} else {
		domain.ValidDomain = false
		domain.Status = false
	}

	elapsed := time.Since(start).Seconds()
	domain = Domain{TimeTaken: elapsed, Domain: params["domain"],
		Cached:      domain.Cached,
		ValidDomain: domain.ValidDomain,
		BlackListed: domain.BlackListed,
		BlackList:   domain.BlackList,
		Status:      domain.Status,
		Errors:      errors}

	if !domain.Cached {
		value, _ := json.Marshal(domain)
		valueStr := string(value)
		err := setRedisKey(domainName, valueStr)
		if err != nil {
			errors = append(errors, string(err.Error()))
		}
	}
	json.NewEncoder(w).Encode(domain)
	// Log
	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{CurrentTime: time.Now(), Method: "/domain", Param: params["domain"], Errors: errors, MemoryAlloc: memAlloc, NumGC: numGC, TimeTaken: elapsed, Cached: domain.Cached, ClientIP: clientIP, Redis: health.Redis, RedisConnections: health.RedisConnections}, "", "   ")

	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
}

// Cancella cache
func DelCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clientIP := r.RemoteAddr
	start := time.Now()
	var errors []string
	reply, err := pingRedis()
	if err != nil || reply != "PONG" {
		health.Redis = false
		errors = append(errors, "Redis: "+err.Error())

	} else {
		health.Redis = true
		health.RedisConnections = getRedisConnections()
	}
	params := mux.Vars(r)
	key := params["key"]
	err = delRedisKey(key)
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
	var memAlloc uint64
	var numGC uint32
	memAlloc, numGC = MemUsage()
	u, err := json.MarshalIndent(Log{CurrentTime: time.Now(), Method: "/clear-cache", Param: key, TimeTaken: elapsed, Errors: errors, MemoryAlloc: memAlloc, NumGC: numGC, ClientIP: clientIP, RedisConnections: health.RedisConnections}, "", "   ")

	if err != nil {
		panic(err)
	}
	fmt.Println(string(u))
}
