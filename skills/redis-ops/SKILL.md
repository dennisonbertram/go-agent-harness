---
name: redis-ops
description: "Operate Redis with redis-cli: key patterns, pub/sub, Lua scripting, memory analysis, cluster operations. Trigger: when using Redis, redis-cli, Redis keys, pub/sub messaging, Redis scripting, Redis cluster, Redis memory, KEYS pattern, SCAN, Redis streams"
version: 1
argument-hint: "[cli|monitor|info|cluster] [command] [args]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Redis Operations

You are now operating in Redis management and operations mode.

## Installation and Connection

```bash
# macOS (via Homebrew)
brew install redis

# Start Redis server
redis-server

# Start with config file
redis-server /usr/local/etc/redis.conf

# Start Redis in background
redis-server --daemonize yes

# Connect with redis-cli (local default)
redis-cli

# Connect to remote server
redis-cli -h redis.example.com -p 6379

# Connect with password
redis-cli -h redis.example.com -p 6379 -a mypassword

# Connect with TLS
redis-cli --tls -h redis.example.com -p 6380

# Check server connectivity
redis-cli ping  # returns PONG

# Check Redis version
redis-cli info server | grep redis_version
```

## Basic Key Operations

```bash
# Set a key
redis-cli SET mykey "hello world"
redis-cli SET mykey "hello" EX 3600  # with TTL of 1 hour
redis-cli SET mykey "hello" PX 60000 # with TTL of 60 seconds (ms)
redis-cli SET mykey "hello" NX       # only set if not exists
redis-cli SET mykey "hello" XX       # only set if exists

# Get a key
redis-cli GET mykey

# Check if a key exists
redis-cli EXISTS mykey  # returns 1 (exists) or 0 (not found)

# Delete a key
redis-cli DEL mykey
redis-cli DEL key1 key2 key3  # delete multiple

# Get remaining TTL
redis-cli TTL mykey    # in seconds, -1 = no expiry, -2 = not found
redis-cli PTTL mykey   # in milliseconds

# Set TTL on an existing key
redis-cli EXPIRE mykey 3600    # seconds
redis-cli EXPIREAT mykey 1735689600  # Unix timestamp

# Remove TTL (make persistent)
redis-cli PERSIST mykey

# Rename a key
redis-cli RENAME oldkey newkey
redis-cli RENAMENX oldkey newkey  # only if newkey doesn't exist

# Get key type
redis-cli TYPE mykey  # string, list, set, zset, hash, stream

# Atomically get and set
redis-cli GETSET mykey "newvalue"
redis-cli GETDEL mykey  # get and delete
```

## Key Patterns and SCAN

```bash
# IMPORTANT: Never use KEYS in production — it blocks the server
# Use SCAN instead for safe iteration

# Scan through all keys (cursor-based, non-blocking)
redis-cli SCAN 0

# Scan with a pattern
redis-cli SCAN 0 MATCH "user:*"
redis-cli SCAN 0 MATCH "session:*" COUNT 100

# Full scan loop in bash
cursor=0
while true; do
  result=$(redis-cli SCAN $cursor MATCH "user:*" COUNT 100)
  cursor=$(echo "$result" | head -1)
  keys=$(echo "$result" | tail -n +2)
  if [ -n "$keys" ]; then
    echo "$keys"
  fi
  if [ "$cursor" = "0" ]; then break; fi
done

# Count keys matching a pattern (safe)
redis-cli --scan --pattern "user:*" | wc -l

# Find keys by type
redis-cli --scan --pattern "*" | while read key; do
  type=$(redis-cli TYPE "$key")
  if [ "$type" = "hash" ]; then
    echo "$key"
  fi
done

# Delete all keys matching a pattern (use carefully)
redis-cli --scan --pattern "session:*" | xargs redis-cli DEL

# Use SCAN with HSCAN, SSCAN, ZSCAN for collection iteration
redis-cli HSCAN myhash 0 MATCH "field*" COUNT 50
redis-cli SSCAN myset 0 COUNT 100
redis-cli ZSCAN myzset 0 MATCH "*" COUNT 100
```

## Data Structures

```bash
# --- Strings ---
redis-cli INCR counter          # atomic increment
redis-cli INCRBY counter 10
redis-cli DECR counter
redis-cli APPEND mykey " world" # append to string

# --- Hashes ---
redis-cli HSET user:1 name "Alice" email "alice@example.com" age 30
redis-cli HGET user:1 name
redis-cli HGETALL user:1         # get all fields
redis-cli HMSET user:2 name "Bob" email "bob@example.com"
redis-cli HMGET user:1 name email
redis-cli HDEL user:1 age
redis-cli HEXISTS user:1 name
redis-cli HLEN user:1            # number of fields
redis-cli HKEYS user:1           # all field names
redis-cli HVALS user:1           # all values

# --- Lists ---
redis-cli LPUSH mylist a b c    # push left (c b a order)
redis-cli RPUSH mylist d e f    # push right (a b c d e f)
redis-cli LPOP mylist           # pop from left
redis-cli RPOP mylist           # pop from right
redis-cli LRANGE mylist 0 -1    # all elements
redis-cli LRANGE mylist 0 9     # first 10 elements
redis-cli LLEN mylist
redis-cli LINDEX mylist 0       # element at index
redis-cli LINSERT mylist BEFORE "b" "x"
redis-cli BLPOP mylist 5        # blocking pop with 5s timeout

# --- Sets ---
redis-cli SADD myset a b c d
redis-cli SREM myset a
redis-cli SISMEMBER myset b     # check membership
redis-cli SMEMBERS myset        # all members
redis-cli SCARD myset           # cardinality
redis-cli SUNION set1 set2      # union
redis-cli SINTER set1 set2      # intersection
redis-cli SDIFF set1 set2       # difference

# --- Sorted Sets ---
redis-cli ZADD leaderboard 100 "player1" 200 "player2" 150 "player3"
redis-cli ZSCORE leaderboard "player1"
redis-cli ZRANK leaderboard "player1"   # rank (0-based, lowest first)
redis-cli ZREVRANK leaderboard "player1" # rank (highest first)
redis-cli ZRANGE leaderboard 0 -1 WITHSCORES
redis-cli ZREVRANGE leaderboard 0 9 WITHSCORES  # top 10
redis-cli ZRANGEBYSCORE leaderboard 100 200 WITHSCORES
redis-cli ZINCRBY leaderboard 50 "player1"
redis-cli ZREM leaderboard "player1"
redis-cli ZCARD leaderboard
```

## Pub/Sub Messaging

```bash
# Subscribe to a channel (blocks, waiting for messages)
redis-cli SUBSCRIBE my-channel

# Subscribe to multiple channels
redis-cli SUBSCRIBE channel1 channel2 channel3

# Subscribe with pattern matching
redis-cli PSUBSCRIBE "user:*" "event:*"

# Publish a message (from another terminal/connection)
redis-cli PUBLISH my-channel "Hello, subscribers!"
redis-cli PUBLISH user:123 '{"event":"login","timestamp":1700000000}'

# List active subscriptions (from another connection)
redis-cli PUBSUB CHANNELS        # all active channels
redis-cli PUBSUB CHANNELS "user:*"  # channels matching pattern
redis-cli PUBSUB NUMSUB channel1 channel2  # subscriber count per channel
redis-cli PUBSUB NUMPAT          # number of pattern subscriptions
```

```bash
# Pub/Sub in a script (subscriber)
redis-cli SUBSCRIBE events &
SUB_PID=$!

# Publish messages
redis-cli PUBLISH events "message1"
redis-cli PUBLISH events "message2"

# Stop subscriber
kill $SUB_PID
```

## Lua Scripting

```bash
# Run a Lua script inline (EVAL)
redis-cli EVAL "return 'hello'" 0

# Script with key and argument access
redis-cli EVAL "return redis.call('GET', KEYS[1])" 1 mykey

# Atomic increment with expiry (set-if-not-exists pattern)
redis-cli EVAL "
  local current = redis.call('GET', KEYS[1])
  if not current then
    redis.call('SET', KEYS[1], ARGV[1])
    redis.call('EXPIRE', KEYS[1], ARGV[2])
    return 1
  end
  return 0
" 1 my-lock-key "locked" 30

# Load script and get SHA (for reuse)
SHA=$(redis-cli SCRIPT LOAD "return redis.call('GET', KEYS[1])")
redis-cli EVALSHA "$SHA" 1 mykey

# Rate limiting Lua script
redis-cli EVAL "
  local key = KEYS[1]
  local limit = tonumber(ARGV[1])
  local window = tonumber(ARGV[2])
  local current = redis.call('INCR', key)
  if current == 1 then
    redis.call('EXPIRE', key, window)
  end
  if current > limit then
    return 0
  end
  return 1
" 1 "ratelimit:user:123" 100 60

# Flush all loaded scripts
redis-cli SCRIPT FLUSH

# Check if a script exists
redis-cli SCRIPT EXISTS $SHA
```

## Memory Analysis

```bash
# Get memory usage stats
redis-cli INFO memory

# Key memory statistics:
# used_memory: total allocated memory
# used_memory_human: human-readable
# used_memory_peak: peak memory usage
# mem_fragmentation_ratio: >1.5 indicates fragmentation

# Get memory usage of a specific key (in bytes)
redis-cli MEMORY USAGE mykey
redis-cli MEMORY USAGE myhash SAMPLES 5

# Find largest keys (approximate)
redis-cli --bigkeys

# Analyze memory distribution
redis-cli MEMORY DOCTOR  # get memory analysis

# Memory usage by data type
redis-cli INFO keyspace

# Debug object memory (for individual keys)
redis-cli DEBUG OBJECT mykey

# Get LRU idle time for a key
redis-cli OBJECT IDLETIME mykey

# Get encoding used for a key
redis-cli OBJECT ENCODING mykey

# Purge expired keys immediately
redis-cli DEBUG SLEEP 0
```

## Persistence and Configuration

```bash
# Check persistence config
redis-cli CONFIG GET save
redis-cli CONFIG GET appendonly

# Trigger RDB snapshot
redis-cli BGSAVE          # background save
redis-cli SAVE            # synchronous save (blocks)
redis-cli LASTSAVE        # timestamp of last save

# Trigger AOF rewrite
redis-cli BGREWRITEAOF

# Flush all keys (DANGER: irreversible)
redis-cli FLUSHDB          # flush current database
redis-cli FLUSHDB ASYNC    # async flush
redis-cli FLUSHALL         # flush all databases (VERY DANGEROUS)

# Switch database (0-15 by default)
redis-cli -n 1 GET mykey  # use database 1

# Move key to another database
redis-cli MOVE mykey 1

# Config rewrite (persist current config)
redis-cli CONFIG REWRITE
```

## Cluster Operations

```bash
# Check cluster info
redis-cli CLUSTER INFO

# Check cluster nodes
redis-cli CLUSTER NODES

# Check which node owns a key slot
redis-cli -c CLUSTER KEYSLOT mykey  # returns slot number (0-16383)

# Follow cluster redirects (use -c flag)
redis-cli -c -h redis-cluster-node1 -p 7000 GET mykey

# Get nodes for a slot
redis-cli CLUSTER SLOTS

# Reshard cluster (redistribute slots)
redis-cli --cluster reshard redis-node1:7000

# Add a node to cluster
redis-cli --cluster add-node new-node:7006 existing-node:7000

# Check cluster health
redis-cli --cluster check redis-node1:7000

# Rebalance cluster
redis-cli --cluster rebalance redis-node1:7000
```

## Monitoring and Diagnostics

```bash
# Watch all commands in real-time (debug only, high overhead)
redis-cli MONITOR

# Get server stats
redis-cli INFO
redis-cli INFO server    # server info
redis-cli INFO clients   # connected clients
redis-cli INFO memory    # memory stats
redis-cli INFO stats     # throughput stats
redis-cli INFO replication  # master/replica info
redis-cli INFO keyspace  # key counts per database

# Get slow log (commands taking > slowlog-log-slower-than microseconds)
redis-cli SLOWLOG GET 10  # last 10 slow commands
redis-cli SLOWLOG LEN     # number of slow log entries
redis-cli SLOWLOG RESET   # clear slow log

# Get client list
redis-cli CLIENT LIST

# Kill a client connection
redis-cli CLIENT KILL ID 123

# Get command stats
redis-cli INFO commandstats | grep -E "cmd_(get|set|hget):"

# Latency monitoring
redis-cli LATENCY LATEST
redis-cli LATENCY HISTORY event-name
redis-cli LATENCY RESET
```

## Common Patterns

```bash
# Cache-aside pattern check
check_cache() {
  local key="$1"
  local value=$(redis-cli GET "$key")
  if [ -n "$value" ]; then
    echo "$value"
  else
    echo "CACHE_MISS"
  fi
}

# Distributed lock (simple)
acquire_lock() {
  local key="lock:$1"
  local ttl=30
  redis-cli SET "$key" "1" NX EX "$ttl"  # returns OK or nil
}

release_lock() {
  local key="lock:$1"
  redis-cli DEL "$key"
}

# Session store pattern
redis-cli HSET "session:abc123" \
  user_id 42 \
  username "alice" \
  role "admin" \
  created_at 1700000000
redis-cli EXPIRE "session:abc123" 3600

# Counter with sliding window rate limit
redis-cli MULTI
redis-cli INCR "ratelimit:user:42:$(date +%s)"
redis-cli EXPIRE "ratelimit:user:42:$(date +%s)" 60
redis-cli EXEC
```

## Troubleshooting

```bash
# Debug connection issues
redis-cli -h redis.example.com ping

# Check why a key doesn't exist
redis-cli DEBUG SLEEP 0  # process pending expires
redis-cli TTL mykey       # check if expired

# Diagnose high memory usage
redis-cli --bigkeys
redis-cli MEMORY DOCTOR

# Check replication lag
redis-cli INFO replication | grep master_repl_offset
redis-cli INFO replication | grep slave_repl_offset

# Common issues:
# "WRONGTYPE" error — key type mismatch; check TYPE before operating
# High memory — scan for large keys with --bigkeys
# Slow commands — check SLOWLOG; use SCAN instead of KEYS
# Connection refused — check redis-server is running: redis-cli ping
# Max clients reached — check CLIENT LIST, increase maxclients config
```
