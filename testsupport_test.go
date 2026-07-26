package main

import (
	"net"
	"os"
	"testing"
)

// TestMain installs a hermetic environment for the unit tests: no config.toml on
// disk, no Redis server and no DNS traffic are required, so `go test` works on any
// machine and in CI.
//
// Integration tests override this in setupTestWithResolver, which deliberately
// reads the real configuration instead.
func TestMain(m *testing.M) {
	setupUnitEnvironment()
	os.Exit(m.Run())
}

// setupUnitEnvironment builds the package-level state handlers depend on, without
// touching the filesystem or the network.
func setupUnitEnvironment() {
	configuration = Config{
		ipBlacklist:     []string{"zen.spamhaus.org"},
		domainBlacklist: []string{"multi.uribl.com"},
	}
	applyConfigDefaults(&configuration)

	// redigo pools connect lazily, so this never dials: the Redis health monitor
	// simply records "unavailable", which is exactly the degraded mode the
	// validation-path tests exercise.
	c = redisConnect()
	resolver = net.DefaultResolver
	nameservers = nil

	startBackgroundMonitors()
}
