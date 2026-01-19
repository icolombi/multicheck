package main

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
)

func redisConnect() *redis.Pool {
	connString := fmt.Sprintf("%s:%d", configuration.RedisHost, configuration.RedisPort)
	return &redis.Pool{
		MaxIdle:   1,
		MaxActive: 8, // max number of connections
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", connString)
			if err != nil {
				//panic(err.Error())
				return c, err
			}
			// Authentication if a password is present
			if configuration.RedisPassword != "" {
				if _, err := c.Do("AUTH", configuration.RedisPassword); err != nil {
					c.Close()
					return nil, err
				}
			}
			// Database selection
			if _, err := c.Do("SELECT", configuration.RedisDatabase); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
	}
}
