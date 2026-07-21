# Backend correctness and performance pass

Version 1.0.0 → 1.5.0 (backend and frontend versions realigned).

## Bug fixes

### `Uptime` was reported in the wrong unit

`main.go` computed `time.Duration(time.Since(startTime).Seconds())`, converting a
count of seconds into a `Duration`, which is expressed in nanoseconds. After one
hour of uptime `/health` returned `3600` instead of `3600000000000`. The frontend
health dashboard divides the field by 1e9 (as documented in README.md), so it
displayed `0s` permanently. Now uses `time.Since(startTime)` directly.

### Data race on the `blacklisted` flag

`checkIPDNS()` and `checkDomainDNS()` wrote `*blacklisted = true` outside the
mutex that already protected the neighbouring map write, while every goroutine of
the fan-out can reach that branch at once. The write moved inside the existing
`mu.Lock()`. `go test -race` now passes.

### Data races on handler-written globals

`endpoints` (written by `RootHandler`) and `uptime` (written by
`HealthCheckHandler`) were package-level variables mutated per request:
concurrent clients raced on them. Both are now locals and the globals are gone.

### Redis errors were silently discarded

All four check handlers did:

```go
errors = append(errors, "Redis: "+err.Error())   // Redis unavailable
...
ip.BlackListed, ip.BlackList, errors = checkBlacklistIP(ipAddress)  // overwrites
```

The assignment dropped every error collected earlier, so in the exact scenario
the project documents (Redis down, service degrades gracefully) the client never
saw the Redis error. The check errors are now collected in a separate variable
and appended.

### Possible nil-pointer panic on Redis status checks

Seven call sites used `if err != nil || reply != "PONG" { ... err.Error() }`. An
unexpected PING reply with a `nil` error dereferenced nil and crashed the whole
process. Replaced by `redisErrorMessage(reply, err)`, which handles both cases.

### `log.Fatal` reachable from a request path

`createCustomResolver()` terminated the process when handed an empty nameserver
list. It now returns an `error`; the POST handlers answer HTTP 500 and `main()`
keeps the fatal behaviour, which is appropriate at startup.

### Resolver ignored the requested network

The `Dial` closure hardcoded `"udp"`, discarding its `network` argument. When a
DNSBL answer came back truncated, the Go resolver's TCP retry was served a UDP
connection instead. The argument is now forwarded.

### Partial results were cached

If part of the fan-out timed out, the incomplete result was still stored for the
full `redisCacheTTL`. Cache writes now happen only when the check returned no
errors.

## Performance

### `runtime.ReadMemStats` removed from the request path

`logRequest()` called `MemUsage()` on every request, and `runtime.ReadMemStats`
stops the world. Memory is now sampled by a background ticker every
`memStatsInterval` seconds; `MemUsage()` reads the cached value from atomics.

### Redis `PING` removed from the request path

Every handler opened with `PING` + `ActiveCount()` purely to populate a log
field, adding round-trips before the cache was even consulted. A background
monitor probes Redis every `redisHealthCheckInterval` seconds and stores the
result in atomics, read via `redisStatus()`. `/health` still pings live, since
reporting the current state is its purpose.

Both monitors are started by `startBackgroundMonitors()` (guarded by `sync.Once`)
and are primed synchronously, so the first request already sees real values.

### Single-command cache writes

`setRedisKey()` used a `SET` + `EXPIRE` pipeline whose `SET` reply was discarded,
so a failed `SET` was invisible and a crash between the two commands could leave
a key without a TTL. Replaced by `SET key value EX ttl`.

### Connection pool tuning

`MaxIdle: 1` / `MaxActive: 8` with `Wait: false` meant the ninth concurrent
request received `ErrPoolExhausted`, which the callers cannot distinguish from a
cache miss — producing a redundant DNS fan-out exactly under peak load. `Dial`
also had no timeouts, so an unresponsive Redis blocked the handler until the HTTP
write timeout fired. The pool now uses configurable sizing (defaults 8 / 64),
`Wait: true`, connect/read/write timeouts, `IdleTimeout`, `MaxConnLifetime` and
`TestOnBorrow`. AUTH and SELECT moved to redigo dial options.

## New configuration parameters

All optional, with defaults applied by `applyConfigDefaults()` so existing
`config.toml` files keep working:

| Parameter | Default | Purpose |
|-----------|---------|---------|
| `redisMaxIdle` | 8 | Idle connections kept in the pool |
| `redisMaxActive` | 64 | Upper bound on concurrent Redis connections |
| `redisConnTimeout` | 2 | Connect/read/write timeout for Redis (seconds) |
| `redisHealthCheckInterval` | 5 | Redis availability probe interval (seconds) |
| `memStatsInterval` | 10 | Memory sampling interval (seconds) |

## Compatibility

No change to any endpoint, request body or response field. The only observable
differences are the corrected `Uptime` unit and the fact that `MemoryAlloc`,
`NumGC`, `Redis` and `RedisConnections` in the request logs may be up to one
monitor interval old.

## Testing

`setupTestWithResolver()` now calls `startBackgroundMonitors()` and builds its
resolver through `createCustomResolver()` instead of duplicating it. Full suite
passes with `go test -race`, against Valkey and live DNSBLs.
