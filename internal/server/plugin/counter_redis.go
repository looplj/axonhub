package plugin

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Each series has a hash of occupied buckets and a running total, plus a sorted
// set indexing bucket expiry. A call visits only expired buckets, not the whole
// window. Both keys share a Redis Cluster hash tag; escaped components cannot
// introduce braces or collide with the separators.
func counterRedisKeys(pluginID, key string, window int) []string {
	series := strings.TrimPrefix(counterSeries(pluginID, key, window), pluginCounterPrefix)
	prefix := pluginCounterPrefix + "{" + series + "}"
	return []string{prefix + ":counts", prefix + ":expires"}
}

// Use Redis time so instances with different clocks agree on the window. The
// script makes expiry, increment and returned total a single atomic operation.
// Reads never extend TTLs and missing reads never allocate persistent keys.
var redisCounterScript = redis.NewScript(`
local counts, expires = KEYS[1], KEYS[2]
local window, size, delta = tonumber(ARGV[1]), tonumber(ARGV[2]), tonumber(ARGV[3])
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
local current = math.floor(now / (size * 1000)) * size
local width = math.ceil(window / size) * size

-- Redis may evict either key independently. Reset an incomplete pair instead
-- of retaining a total whose expiry index has been lost.
if redis.call('EXISTS', counts) ~= redis.call('EXISTS', expires) then
  redis.call('DEL', counts, expires)
end
local total = tonumber(redis.call('HGET', counts, 'total') or '0')
local expired = redis.call('ZRANGEBYSCORE', expires, '-inf', now)
for _, bucket in ipairs(expired) do
  total = total - tonumber(redis.call('HGET', counts, bucket) or '0')
  redis.call('HDEL', counts, bucket)
  redis.call('ZREM', expires, bucket)
end

if delta == 1 then
  local bucket = tostring(current)
  redis.call('HINCRBY', counts, bucket, 1)
  total = total + 1
  -- Match the memory backend: a bucket leaves at the earlier of the aligned
  -- window boundary and window+1 seconds after its last increment.
  local deadline = math.min((current + width) * 1000, now + (window + 1) * 1000)
  redis.call('ZADD', expires, deadline, bucket)
end

if total == 0 then
  redis.call('DEL', counts, expires)
  return 0
end
if delta == 1 or #expired > 0 then
  redis.call('HSET', counts, 'total', total)
end
if delta == 1 then
  redis.call('PEXPIRE', counts, (window + 1) * 1000)
  redis.call('PEXPIRE', expires, (window + 1) * 1000)
end
return total
`)

func redisCounter(ctx context.Context, client redis.UniversalClient, pluginID, key string, window int, add bool) (int64, error) {
	delta := 0
	if add {
		delta = 1
	}
	return redisCounterScript.Run(ctx, client, counterRedisKeys(pluginID, key, window),
		window, counterBucketSize(window), delta).Int64()
}
