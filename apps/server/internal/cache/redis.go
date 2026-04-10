package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	raw *redis.Client
}

func New(addr string) *Client {
	return &Client{
		raw: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func (c *Client) SetRefreshToken(ctx context.Context, userID string, token string) error {
	if token == "" {
		return c.raw.Del(ctx, "refresh:"+userID).Err()
	}
	return c.raw.Set(ctx, "refresh:"+userID, token, 7*24*time.Hour).Err()
}

func (c *Client) GetRefreshToken(ctx context.Context, userID string) (string, error) {
	return c.raw.Get(ctx, "refresh:"+userID).Result()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.raw.Ping(ctx).Err()
}
