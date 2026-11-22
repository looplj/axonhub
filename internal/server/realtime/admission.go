// 职责: 原子限制实时会话的全局、项目、密钥、渠道并发及建连频率。
// 关联文件:
//
//	internal/server/api/realtime.go
//	internal/pkg/xredis/client.go
//
// 注意事项:
//
//	内存模式仅约束单进程；多实例必须共享 Redis。同槽 Lua 原子获取所有维度。
//	租约存活超过会话硬超时；进程崩溃后自动过期，释放按随机会话标识幂等执行。
//	断线交接最多等待一秒；等待不撤销旧租约、不提高上限，每个密钥仅允许一个等待者。

package realtime

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrAdmission = errors.New("realtime concurrency or connection rate limit exceeded")

var errConcurrentAdmission = errors.New("realtime active connection limit exceeded")

type Limits struct {
	Global               int `conf:"global" yaml:"global" json:"global"`
	Project              int `conf:"project" yaml:"project" json:"project"`
	APIKey               int `conf:"api_key" yaml:"api_key" json:"api_key"`
	Channel              int `conf:"channel" yaml:"channel" json:"channel"`
	ConnectionsPerMinute int `conf:"connections_per_minute" yaml:"connections_per_minute" json:"connections_per_minute"`
}

func (l Limits) defaults() Limits {
	if l.Global <= 0 {
		l.Global = 32
	}
	if l.Project <= 0 {
		l.Project = 8
	}
	if l.APIKey <= 0 {
		l.APIKey = 1
	}
	if l.Channel <= 0 {
		l.Channel = 8
	}
	if l.ConnectionsPerMinute <= 0 {
		l.ConnectionsPerMinute = 20
	}
	return l
}

type Config struct {
	Limits           Limits        `conf:"limits" yaml:"limits" json:"limits"`
	IdleTimeout      time.Duration `conf:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	MaxDuration      time.Duration `conf:"max_duration" yaml:"max_duration" json:"max_duration"`
	HandshakeTimeout time.Duration `conf:"handshake_timeout" yaml:"handshake_timeout" json:"handshake_timeout"`
}

func (c Config) Defaults() Config {
	c.Limits = c.Limits.defaults()
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 2 * time.Minute
	}
	if c.MaxDuration <= 0 {
		c.MaxDuration = 15 * time.Minute
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 15 * time.Second
	}
	return c
}

type Admission struct {
	client  redis.UniversalClient
	limits  Limits
	lease   time.Duration
	mu      sync.Mutex
	entries map[string]map[string]int64
	waiting map[int]struct{}
}

func NewAdmission(client redis.UniversalClient, limits Limits, maxDuration time.Duration) *Admission {
	if maxDuration <= 0 {
		maxDuration = 15 * time.Minute
	}
	return &Admission{client: client, limits: limits.defaults(), lease: maxDuration + time.Minute, entries: make(map[string]map[string]int64), waiting: make(map[int]struct{})}
}

// AcquireForReconnect allows one bounded handoff while a disconnected socket's
// reader catches EOF and its workers join. It never evicts a live connection or
// retries a rate denial/backend failure. Redis remains the atomic authority.
func (a *Admission) AcquireForReconnect(ctx context.Context, key, project, channel int, sessionID string) (func(), error) {
	release, err := a.Acquire(ctx, key, project, channel, sessionID)
	if !errors.Is(err, errConcurrentAdmission) {
		return release, err
	}
	a.mu.Lock()
	_, duplicate := a.waiting[key]
	if duplicate || len(a.waiting) >= a.limits.Global {
		a.mu.Unlock()
		return nil, err
	}
	a.waiting[key] = struct{}{}
	a.mu.Unlock()
	defer func() { a.mu.Lock(); delete(a.waiting, key); a.mu.Unlock() }()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, err
		case <-tick.C:
			release, err = a.Acquire(ctx, key, project, channel, sessionID)
			if !errors.Is(err, errConcurrentAdmission) {
				return release, err
			}
		}
	}
}

func admissionLimitError(index int) error {
	dimensions := [...]string{"global connections", "project connections", "API key connections", "channel connections", "API key connections per minute"}
	if index < 0 || index >= len(dimensions) {
		return ErrAdmission
	}
	if index < 4 {
		return fmt.Errorf("%w (%s): %w", ErrAdmission, dimensions[index], errConcurrentAdmission)
	}
	return fmt.Errorf("%w: %s", ErrAdmission, dimensions[index])
}

func (a *Admission) Acquire(ctx context.Context, key, project, channel int, sessionID ...string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefix := "axonhub:{realtime}:"
	keys := []string{prefix + "global", prefix + "project:" + strconv.Itoa(project), prefix + "key:" + strconv.Itoa(key), prefix + "channel:" + strconv.Itoa(channel), prefix + "rate:" + strconv.Itoa(key)}
	caps := []int{a.limits.Global, a.limits.Project, a.limits.APIKey, a.limits.Channel, a.limits.ConnectionsPerMinute}
	token := uuid.NewString()
	if len(sessionID) > 0 && sessionID[0] != "" {
		token = sessionID[0]
	}
	if a.client != nil {
		args := []any{token, a.lease.Milliseconds()}
		for _, cap := range caps {
			args = append(args, cap)
		}
		result, err := a.client.Eval(ctx, acquireScript, keys, args...).Int()
		if err != nil {
			return nil, fmt.Errorf("realtime shared admission unavailable: %w", err)
		}
		if result != 1 {
			return nil, admissionLimitError(-result - 1)
		}
	} else {
		now := time.Now().UnixMilli()
		a.mu.Lock()
		// Sweep all keys so one-off API keys cannot grow the local map forever.
		for k, entries := range a.entries {
			for id, expiry := range entries {
				if expiry <= now {
					delete(entries, id)
				}
			}
			if len(entries) == 0 {
				delete(a.entries, k)
			}
		}
		for i, k := range keys {
			if len(a.entries[k]) >= caps[i] {
				a.mu.Unlock()
				return nil, admissionLimitError(i)
			}
		}
		for i, k := range keys {
			if a.entries[k] == nil {
				a.entries[k] = map[string]int64{}
			}
			ttl := a.lease
			if i == 4 {
				ttl = time.Minute
			}
			a.entries[k][token] = now + ttl.Milliseconds()
		}
		a.mu.Unlock()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if a.client != nil {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = a.client.Eval(releaseCtx, releaseScript, keys[:4], token).Err()
				return
			}
			a.mu.Lock()
			defer a.mu.Unlock()
			for _, k := range keys[:4] {
				delete(a.entries[k], token)
				if len(a.entries[k]) == 0 {
					delete(a.entries, k)
				}
			}
		})
	}, nil
}

//nolint:dupword,intrange // This is Redis Lua; Go loop linters do not apply to its syntax.
const acquireScript = `
local t=redis.call('TIME')
local now=t[1]*1000+math.floor(t[2]/1000)
for i,key in ipairs(KEYS) do
 redis.call('ZREMRANGEBYSCORE',key,'-inf',now)
 if redis.call('ZCARD',key)>=tonumber(ARGV[i+2]) then return -i end
end
for i,key in ipairs(KEYS) do
 local ttl=tonumber(ARGV[2]);if i==5 then ttl=60000 end
 redis.call('ZADD',key,now+ttl,ARGV[1])
 redis.call('PEXPIRE',key,ttl+1000)
end
return 1`
const releaseScript = `for _,key in ipairs(KEYS) do redis.call('ZREM',key,ARGV[1]) end return 1`

// Validate keeps leases and bounded shutdown within the declared operational limits.
func (c Config) Validate() error {
	if c.HandshakeTimeout < 0 || c.HandshakeTimeout > time.Minute {
		return errors.New("realtime handshake timeout must be between zero and 1m")
	}
	if c.MaxDuration < 0 || c.MaxDuration > time.Hour {
		return errors.New("realtime maximum duration must be between zero and 1h")
	}
	if c.IdleTimeout < 0 || c.IdleTimeout > 10*time.Minute {
		return errors.New("realtime idle timeout must be between zero and 10m")
	}
	for _, limit := range []int{c.Limits.Global, c.Limits.Project, c.Limits.APIKey, c.Limits.Channel} {
		if limit < 0 || limit > 1024 {
			return errors.New("realtime concurrency limits must be between zero and 1024")
		}
	}
	if c.Limits.ConnectionsPerMinute < 0 || c.Limits.ConnectionsPerMinute > 10000 {
		return errors.New("realtime connection rate must be between zero and 10000")
	}
	return nil
}

// IsActive identifies still-owned accounting reservations across gateway instances.
func (a *Admission) IsActive(ctx context.Context, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	if a.client != nil {
		value, err := a.client.Eval(ctx, `local t=redis.call('TIME'); local n=t[1]*1000+math.floor(t[2]/1000); local s=redis.call('ZSCORE',KEYS[1],ARGV[1]); if s and tonumber(s)>n then return 1 end return 0`, []string{"axonhub:{realtime}:global"}, sessionID).Int()
		return value == 1, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.entries["axonhub:{realtime}:global"][sessionID] > time.Now().UnixMilli(), nil
}
