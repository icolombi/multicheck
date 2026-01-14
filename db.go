package main

import (
	"fmt"
	"multicheck/credentialstore"

	"github.com/gomodule/redigo/redis"
)

func redisConnect() *redis.Pool {
	// var redisPassword string = credentialstore.RedisPassword
	var redisDatabase int = credentialstore.RedisDatabase
	connString := fmt.Sprintf("%s:%d", credentialstore.RedisHost, credentialstore.RedisPort)
	return &redis.Pool{
		MaxIdle:   1,
		MaxActive: 8, // max number of connections
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", connString)
			if err != nil {
				//panic(err.Error())
				return c, err
			}
			// if _, err := c.Do("AUTH", redisPassword); err != nil {
			// 	c.Close()
			// 	return nil, err
			// }
			if _, err := c.Do("SELECT", redisDatabase); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
	}
}
