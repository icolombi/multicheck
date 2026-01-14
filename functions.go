package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
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

// Passando un IP restituisce la sua presenza in un elenco di blacklist
func checkBlacklistIP(ipAddress string) (blacklisted bool, blacklistsActive map[string][]net.IP, errorList []string) {

	blackLists := configuration.ipBlacklist
	max := len(blackLists)
	reverseIP := reverseIP(ipAddress)
	blacklistsActive = make(map[string][]net.IP)
	ip.BlackListed = false
	// WaitGroup
	var wg sync.WaitGroup
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkIPDNS(&wg, blacklist, reverseIP, blacklistsActive, errorCh)

	}

	error := <-errorCh
	if error != "" {
		errorList = append(errorList, error)
	}
	wg.Wait()
	close(errorCh)

	return ip.BlackListed, blacklistsActive, errorList
}

// Funzione per le query DNS sull'IP
func checkIPDNS(wg *sync.WaitGroup, blacklist string, reverseIP string, blacklistsActive map[string][]net.IP, errorCh chan string) {
	//fmt.Println("Checking " + blacklist)
	defer wg.Done()
	var error string
	value, err := resolver.LookupIP(context.Background(), "ip4", reverseIP+"."+blacklist+".")
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
		ip.BlackListed = true
		blacklistsActive[blacklist] = value
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

	blackLists := configuration.domainBlacklist
	max := len(blackLists)
	blacklistsActive = make(map[string][]net.IP)
	domain.BlackListed = false
	// WaitGroup
	var wg sync.WaitGroup
	errorCh := make(chan string, max)
	wg.Add(max)
	for _, blacklist := range blackLists {
		go checkDomainDNS(&wg, blacklist, domainName, blacklistsActive, errorCh)
	}

	error := <-errorCh
	if error != "" {
		errorList = append(errorList, error)
	}
	wg.Wait()
	close(errorCh)

	return domain.BlackListed, blacklistsActive, errorList
}

// Funzione per le query DNS sul dominio
func checkDomainDNS(wg *sync.WaitGroup, blacklist string, domainName string, blacklistsActive map[string][]net.IP, errorCh chan string) {
	defer wg.Done() // Decrease the WaitGroup counter by 1
	var error string

	//value, err := net.LookupIP(domainName + "." + blacklist)

	// Debug tempo esecuzione query DNS
	//start := time.Now()
	value, err := resolver.LookupIP(context.Background(), "ip4", domainName+"."+blacklist+".")
	value = removeIPFromSlice(value)

	if err != nil {
		//fmt.Println("Error: " + err.Error())
		if !strings.Contains(err.Error(), ": no such host") {
			error = err.Error()
		}
	}

	if len(value) != 0 {

		domain.BlackListed = true
		blacklistsActive[blacklist] = value
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
