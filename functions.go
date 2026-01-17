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
	configuration.nameServers = viper.GetStringSlice("nameServers")
	configuration.listenPort = viper.GetString("listenPort")

	return configuration
}

// Funzione per ricavare l'uso di memoria
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

// Funzione per rimuovere l'IP 127.0.0.1 da uno slice
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

// Valida una lista di nameserver (devono essere IP validi)
func validateNameservers(nameservers []string, maxAllowed int) (valid bool, errorMsg string) {
	// Se la lista è vuota, è valida (useremo i default)
	if len(nameservers) == 0 {
		return true, ""
	}

	// Controllo limite massimo
	if len(nameservers) > maxAllowed {
		return false, fmt.Sprintf("too many nameservers: maximum %d allowed, received %d", maxAllowed, len(nameservers))
	}

	// Validazione che ogni nameserver sia un IP valido
	for _, ns := range nameservers {
		// Controllo che non sia vuoto o solo spazi
		trimmed := strings.TrimSpace(ns)
		if trimmed == "" {
			return false, "nameserver entries cannot be empty or whitespace"
		}

		// Validazione IP
		if net.ParseIP(trimmed) == nil {
			return false, fmt.Sprintf("invalid nameserver: '%s' is not a valid IP address", trimmed)
		}
	}

	return true, ""
}

// Valida una lista di blacklist per sintassi DNS e limiti
func validateBlacklists(blacklists []string, maxAllowed int) (valid bool, errorMsg string) {
	// Controllo lista vuota
	if len(blacklists) == 0 {
		return false, "blacklist array cannot be empty"
	}

	// Controllo limite massimo
	if len(blacklists) > maxAllowed {
		return false, fmt.Sprintf("too many blacklists: maximum %d allowed, received %d", maxAllowed, len(blacklists))
	}

	// Validazione sintassi DNS per ogni blacklist
	for _, bl := range blacklists {
		// Controllo che non sia vuoto o solo spazi
		trimmed := strings.TrimSpace(bl)
		if trimmed == "" {
			return false, "blacklist entries cannot be empty or whitespace"
		}

		// Validazione base del formato DNS
		// Deve contenere almeno un punto e caratteri validi
		if !strings.Contains(trimmed, ".") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (must be a valid DNS name)", trimmed)
		}

		// Controllo caratteri non validi per DNS
		// DNS permette: lettere, numeri, trattino e punto
		for _, char := range trimmed {
			if !((char >= 'a' && char <= 'z') ||
				(char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '.') {
				return false, fmt.Sprintf("invalid blacklist format: '%s' (contains invalid DNS characters)", trimmed)
			}
		}

		// Controllo che non inizi o finisca con punto o trattino
		if strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") ||
			strings.HasPrefix(trimmed, "-") || strings.HasSuffix(trimmed, "-") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (cannot start or end with . or -)", trimmed)
		}

		// Controllo punti consecutivi
		if strings.Contains(trimmed, "..") {
			return false, fmt.Sprintf("invalid blacklist format: '%s' (cannot contain consecutive dots)", trimmed)
		}
	}

	return true, ""
}

// Crea un resolver custom con i nameserver specificati
func createCustomResolver(nameservers []string) *net.Resolver {
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

// Passando un IP restituisce la sua presenza in un elenco di blacklist
func checkBlacklistIP(ipAddress string) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {
	return checkBlacklistIPWithCustomList(ipAddress, configuration.ipBlacklist, nil)
}

// Passando un IP e una lista custom di blacklist restituisce la sua presenza
// Se customResolver è nil, usa il resolver globale
func checkBlacklistIPWithCustomList(ipAddress string, blackLists []string, customResolver *net.Resolver) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {

	// Se non è specificato un resolver custom, usa quello globale
	resolverToUse := resolver
	if customResolver != nil {
		resolverToUse = customResolver
	}

	max := len(blackLists)
	reverseIP := reverseIP(ipAddress)
	blacklistsActive = make(map[string][]net.IP)
	blacklisted = false
	// WaitGroup e Mutex per proteggere accessi concorrenti alla map
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkIPDNS(&wg, &mu, blacklist, reverseIP, blacklistsActive, errorCh, &blacklisted, resolverToUse)

	}

	error := <-errorCh
	if error != "" {
		errorList = append(errorList, error)
	}
	wg.Wait()
	close(errorCh)

	return blacklisted, blacklistsActive, errorList
}

// Funzione per le query DNS sull'IP
func checkIPDNS(wg *sync.WaitGroup, mu *sync.Mutex, blacklist string, reverseIP string, blacklistsActive map[string][]net.IP, errorCh chan string, blacklisted *bool, resolverToUse *net.Resolver) {
	//fmt.Println("Checking " + blacklist)
	defer wg.Done()
	var error string
	value, err := resolverToUse.LookupIP(context.Background(), "ip4", reverseIP+"."+blacklist+".")
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
		// Proteggo la scrittura sulla map condivisa
		mu.Lock()
		blacklistsActive[blacklist] = value
		mu.Unlock()
	}
	errorCh <- error
}

// Funzione per la conversione di un indirizzo IP in un nome di dominio inverso (i.e. "127.0.0.1" > "1.0.0.127")
func reverseIP(ipAddress string) string {
	parts := strings.Split(ipAddress, ".")
	reversedParts := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		reversedParts[i] = parts[len(parts)-1-i]
	}
	return strings.Join(reversedParts, ".")
}

// Passando un dominio restituisce la sua presenza in un elenco di blacklist
func checkBlacklistDomain(domainName string) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {
	return checkBlacklistDomainWithCustomList(domainName, configuration.domainBlacklist, nil)
}

// Passando un dominio e una lista custom di blacklist restituisce la sua presenza
// Se customResolver è nil, usa il resolver globale
func checkBlacklistDomainWithCustomList(domainName string, blackLists []string, customResolver *net.Resolver) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {

	// Se non è specificato un resolver custom, usa quello globale
	resolverToUse := resolver
	if customResolver != nil {
		resolverToUse = customResolver
	}

	max := len(blackLists)
	blacklistsActive = make(map[string][]net.IP)
	blacklisted = false
	// WaitGroup e Mutex per proteggere accessi concorrenti alla map
	var wg sync.WaitGroup
	var mu sync.Mutex
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkDomainDNS(&wg, &mu, blacklist, domainName, blacklistsActive, errorCh, &blacklisted, resolverToUse)
	}

	error := <-errorCh
	if error != "" {
		errorList = append(errorList, error)
	}
	wg.Wait()
	close(errorCh)

	return blacklisted, blacklistsActive, errorList
}

// Funzione per le query DNS sul dominio
func checkDomainDNS(wg *sync.WaitGroup, mu *sync.Mutex, blacklist string, domainName string, blacklistsActive map[string][]net.IP, errorCh chan string, blacklisted *bool, resolverToUse *net.Resolver) {
	defer wg.Done() // Decrease the WaitGroup counter by 1
	var error string

	//value, err := net.LookupIP(domainName + "." + blacklist)

	// Debug tempo esecuzione query DNS
	//start := time.Now()
	value, err := resolverToUse.LookupIP(context.Background(), "ip4", domainName+"."+blacklist+".")
	value = removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}

	if len(value) != 0 {

		*blacklisted = true
		// Proteggo la scrittura sulla map condivisa
		mu.Lock()
		blacklistsActive[blacklist] = value
		mu.Unlock()
	}

	// Debug tempo esecuzione query DNS
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
