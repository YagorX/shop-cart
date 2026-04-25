package lock

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("could not acquire lock")
var ErrLockNotOwned = errors.New("lock is not owned by us")

var unlockScript = redis.NewScript(`
    if redis.call("GET", KEYS[1]) == ARGV[1] then
        return redis.call("DEL", KEYS[1])
    else
        return 0
    end
`)

type DistributedLock struct {
	client        *redis.Client
	ttl           time.Duration
	retryInterval time.Duration
	maxRetries    int
}

func NewDistributedLock(client *redis.Client, ttl, retryInterval time.Duration, maxRetries int) *DistributedLock {
	return &DistributedLock{
		client:        client,
		ttl:           ttl,
		retryInterval: retryInterval,
		maxRetries:    maxRetries,
	}
}

func (l *DistributedLock) Lock(ctx context.Context, key string) (string, error) {
	lockValue := uuid.New().String()

	for attempt := 0; attempt < l.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		ok, err := l.client.SetNX(ctx, "lock:"+key, lockValue, l.ttl).Result()
		if err != nil {
			return "", err // Redis недоступен
		}

		if ok {
			return lockValue, nil
		}

		time.Sleep(l.retryInterval)
	}

	return "", ErrLockNotAcquired
}

func (l *DistributedLock) Unlock(ctx context.Context, key, lockValue string) error {
	result, err := unlockScript.Run(ctx, l.client, []string{"lock:" + key}, lockValue).Result()
	if err != nil {
		return err
	}

	if result.(int64) == 0 {
		return ErrLockNotOwned
	}

	return nil
}
