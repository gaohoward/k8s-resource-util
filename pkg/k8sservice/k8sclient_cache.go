package k8sservice

import "time"

var DEFAULT_CACHE_TIMEOUT = 2 * 60 * time.Second

type CacheValue struct {
	value     any
	timestamp time.Time
}

func newCacheValue(value any) *CacheValue {
	return &CacheValue{
		value:     value,
		timestamp: time.Now(),
	}
}

// The cache is used by k8sService to
// improve performance
type K8sClientCache struct {
	cache   map[string]*CacheValue
	timeout time.Duration
}

func NewK8sCache() *K8sClientCache {
	return &K8sClientCache{
		cache:   make(map[string]*CacheValue),
		timeout: DEFAULT_CACHE_TIMEOUT,
	}
}

func (c *K8sClientCache) Put(key string, value any) {
	cacheValue := newCacheValue(value)
	c.cache[key] = cacheValue
}

func (c *K8sClientCache) IsTimedOut(value *CacheValue) bool {
	if value == nil {
		return true
	}
	return time.Since(value.timestamp) > c.timeout
}

// return nil means no cache, true means the cached value is outdated
func (c *K8sClientCache) GetString(key string) (*string, bool) {
	if cacheValue, ok := c.cache[key]; ok {
		if value, ok := cacheValue.value.(string); ok {
			return &value, c.IsTimedOut(cacheValue)
		}
	}
	return nil, true
}

func (c *K8sClientCache) GetObject(key string) (any, bool) {
	if cacheValue, ok := c.cache[key]; ok {
		return cacheValue.value, c.IsTimedOut(cacheValue)
	}
	return nil, true
}

func (c *K8sClientCache) GetBool(key string) (*bool, bool) {
	if cacheValue, ok := c.cache[key]; ok {
		if value, ok := cacheValue.value.(bool); ok {
			return &value, c.IsTimedOut(cacheValue)
		}
	}
	return nil, true
}
