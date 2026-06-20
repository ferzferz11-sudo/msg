package main

// ai_tool_cache.go — LRU cache with TTL for AI tool results

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type toolCacheEntry struct {
	result    string
	createdAt time.Time
}

type toolCache struct {
	mu      sync.RWMutex
	entries map[string]toolCacheEntry
	ttl     time.Duration
	maxSize int
}

func newToolCache(ttl time.Duration, maxSize int) *toolCache {
	c := &toolCache{
		entries: make(map[string]toolCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go c.evictLoop()
	return c
}

func (c *toolCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.createdAt) > c.ttl {
		return "", false
	}
	return entry.result, true
}

func (c *toolCache) Set(key, result string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[key] = toolCacheEntry{result: result, createdAt: time.Now()}
}

func (c *toolCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range c.entries {
		if oldestKey == "" || v.createdAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.createdAt
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *toolCache) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.entries {
			if now.Sub(v.createdAt) > c.ttl {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

type cachedTool struct {
	inner Tool
	cache *toolCache
	keyFn func(args map[string]any) string
}

func (t *cachedTool) Name() string              { return t.inner.Name() }
func (t *cachedTool) Description() string        { return t.inner.Description() }
func (t *cachedTool) Parameters() map[string]any { return t.inner.Parameters() }
func (t *cachedTool) RequiredRole() string       { return t.inner.RequiredRole() }

func (t *cachedTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	key := t.keyFn(args)
	if key != "" {
		if cached, ok := t.cache.Get(key); ok {
			return cached, nil
		}
	}
	result, err := t.inner.Execute(ctx, args)
	if err == nil && key != "" {
		t.cache.Set(key, result)
	}
	return result, err
}

func newCachedSearchMessagesTool(db *sql.DB, cache *toolCache) Tool {
	return &cachedTool{
		inner: &searchMessagesTool{db: db},
		cache: cache,
		keyFn: func(args map[string]any) string {
			query, _ := args["query"].(string)
			chatID, _ := args["chat_id"].(string)
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			return fmt.Sprintf("sm:%s:%s:%d", chatID, query, limit)
		},
	}
}

func newCachedSearchUsersTool(db *sql.DB, cache *toolCache) Tool {
	return &cachedTool{
		inner: &searchUsersTool{db: db},
		cache: cache,
		keyFn: func(args map[string]any) string {
			query, _ := args["query"].(string)
			limit := 10
			if l, ok := args["limit"].(float64); ok {
				limit = int(l)
			}
			return fmt.Sprintf("su:%s:%d", query, limit)
		},
	}
}

func newCachedGetChatInfoTool(db *sql.DB, cache *toolCache) Tool {
	return &cachedTool{
		inner: &getChatInfoTool{db: db},
		cache: cache,
		keyFn: func(args map[string]any) string {
			chatID, _ := args["chat_id"].(string)
			return "ci:" + chatID
		},
	}
}
