package cache

import (
	"sync"
	"time"

	resource "github.com/leucos/go-toggl/resource"
)

const (
	hits = iota
	misses
)

type ResourcesCache struct {
	caches    map[resource.Type]map[int]map[int]any
	timestamp map[resource.Type]time.Time
	stats     map[resource.Type]map[int]int
	mutex     *sync.Mutex
	ttl       time.Duration
}

// New creates a new cache
func New(ttl time.Duration) ResourcesCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	r := ResourcesCache{
		caches:    make(map[resource.Type]map[int]map[int]any),
		timestamp: make(map[resource.Type]time.Time),
		stats:     make(map[resource.Type]map[int]int),
		mutex:     &sync.Mutex{},
		ttl:       ttl,
	}

	for rt := range resource.TypeMap {
		r.timestamp[rt] = time.Now()
		r.stats[rt] = make(map[int]int)
		r.Clear(rt)
	}

	return r
}

// Clear clears the cache for a given resource type
func (c *ResourcesCache) Clear(rt resource.Type) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.caches[rt] = make(map[int]map[int]any)
}

// GetEntry gets a resource from the cache
func (c *ResourcesCache) Get(rt resource.Type, wid int, id int) (any, bool) {
	c.expireCacheIf()

	if c.caches[rt] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	if c.caches[rt][wid] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	data, ok := c.caches[rt][wid][id]
	c.stats[rt][hits]++
	return data, ok
}

// GetMap gets a full map cache for a given resource in the workspace
func (c *ResourcesCache) GetMap(rt resource.Type, wid int) (map[int]any, bool) {
	c.expireCacheIf()

	if c.caches[rt] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	if c.caches[rt][wid] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	c.stats[rt][hits]++
	return c.caches[rt][wid], true
}

// GetList gets a full list cache for a given resource
func (c *ResourcesCache) GetList(rt resource.Type, wid int) ([]any, bool) {
	c.expireCacheIf()

	if c.caches[rt] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	if c.caches[rt][wid] == nil {
		c.stats[rt][misses]++
		return nil, false
	}

	list := make([]any, 0, len(c.caches[rt][wid]))
	for _, v := range c.caches[rt][wid] {
		list = append(list, v)
	}

	c.stats[rt][hits]++
	return list, true
}

// Set sets a resource in the cache
func (c *ResourcesCache) Set(rt resource.Type, wid int, id int, data any) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.caches[rt] == nil {
		c.caches[rt] = make(map[int]map[int]any)
	}

	if c.caches[rt][wid] == nil {
		c.caches[rt][wid] = make(map[int]any)
	}

	c.caches[rt][wid][id] = data
}

// GetTTL returns the cache TTL
func (c *ResourcesCache) GetTTL() time.Duration {
	return c.ttl
}

// SetTTL sets the cache TTL
func (c *ResourcesCache) SetTTL(ttl time.Duration) {
	c.ttl = ttl
	// recheck cache expiration
	c.expireCacheIf()
}

func (c *ResourcesCache) Stats(rt resource.Type) (int, int, int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return len(c.caches[rt]),c.stats[rt][hits], c.stats[rt][misses]
}

// expireCacheIf clears the project cache if it is older than the cache TTL
func (c *ResourcesCache) expireCacheIf() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for rt, ts := range c.timestamp {
		if time.Since(ts) > c.ttl {
			c.caches[rt] = make(map[int]map[int]any)
			c.timestamp[rt] = time.Now()
		}
	}
}
