package discovery

import (
	"sync"
	"time"

	"k8s.io/client-go/discovery"
)

// DefaultCacheTTL is how long discovery results are considered fresh.
// Resource lists rarely change; 5 minutes is a safe balance between
// freshness and avoiding repeated discovery API round-trips.
const DefaultCacheTTL = 5 * time.Minute

type cachedEntry struct {
	resources []string
	fetchedAt time.Time
}

// ResourceCache caches GetAllResources results per kubeconfig path with a TTL.
// It is safe for concurrent use.
//
// Typical usage:
//
//	var cache = discovery.NewResourceCache(discovery.DefaultCacheTTL)
//	resources, err := cache.Get(kubeconfig, clientset.Discovery())
type ResourceCache struct {
	mu    sync.RWMutex
	store map[string]cachedEntry
	ttl   time.Duration
}

// NewResourceCache creates a ResourceCache with the given TTL.
func NewResourceCache(ttl time.Duration) *ResourceCache {
	return &ResourceCache{
		store: make(map[string]cachedEntry),
		ttl:   ttl,
	}
}

// Get returns cached resources for the given key if still within the TTL,
// otherwise calls GetAllResources, stores the result, and returns it.
// key should be the kubeconfig file path, or "__default__" for the default config.
func (c *ResourceCache) Get(key string, dc discovery.DiscoveryInterface) ([]string, error) {
	if key == "" {
		key = "__default__"
	}

	c.mu.RLock()
	entry, ok := c.store[key]
	c.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < c.ttl {
		return entry.resources, nil
	}

	resources, err := GetAllResources(dc)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.store[key] = cachedEntry{resources: resources, fetchedAt: time.Now()}
	c.mu.Unlock()

	return resources, nil
}

// Invalidate removes the cached entry for key, forcing the next Get to re-fetch.
func (c *ResourceCache) Invalidate(key string) {
	if key == "" {
		key = "__default__"
	}
	c.mu.Lock()
	delete(c.store, key)
	c.mu.Unlock()
}
