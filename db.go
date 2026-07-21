package main

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

func redisConnect() *redis.Pool {
	connString := fmt.Sprintf("%s:%d", configuration.RedisHost, configuration.RedisPort)
	timeout := time.Duration(configuration.RedisConnTimeout) * time.Second

	return &redis.Pool{
		MaxIdle:   configuration.RedisMaxIdle,
		MaxActive: configuration.RedisMaxActive,
		// Recycle connections instead of keeping them forever: idle ones are
		// dropped after 4 minutes, live ones rotated after 30 minutes.
		IdleTimeout:     240 * time.Second,
		MaxConnLifetime: 30 * time.Minute,
		// Wait for a free connection rather than returning ErrPoolExhausted:
		// a failed Get is indistinguishable from a cache miss for the callers,
		// and would trigger a redundant DNS fan-out exactly under peak load.
		Wait: true,
		Dial: func() (redis.Conn, error) {
			// Every phase gets a timeout: without them an unresponsive Redis
			// blocks the handler until the HTTP write timeout fires.
			// DialPassword and DialDatabase are no-ops when empty/zero.
			return redis.Dial("tcp", connString,
				redis.DialConnectTimeout(timeout),
				redis.DialReadTimeout(timeout),
				redis.DialWriteTimeout(timeout),
				redis.DialPassword(configuration.RedisPassword),
				redis.DialDatabase(configuration.RedisDatabase),
			)
		},
		// Validate connections that sat idle long enough to have been closed
		// server-side, so a stale one is not handed to a request.
		TestOnBorrow: func(conn redis.Conn, lastUsed time.Time) error {
			if time.Since(lastUsed) < time.Minute {
				return nil
			}
			_, err := conn.Do("PING")
			return err
		},
	}
}
