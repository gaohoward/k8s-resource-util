package k8sservice

import "time"

var DEFAULT_CACHE_TIMEOUT = 1 * 60 * time.Second

type CacheValue struct {
	value     any
	timestamp time.Time
	timeout   time.Duration
}

func newCacheValue(value any, ownTimeout time.Duration) *CacheValue {
	return &CacheValue{
		value:     value,
		timestamp: time.Now(),
		timeout:   ownTimeout,
	}
}

// The cache is used by k8sService to
// improve performance
type K8sClientCache struct {
	cache      map[string]*CacheValue
	defTimeout time.Duration
}

func NewK8sCache() *K8sClientCache {
	return &K8sClientCache{
		cache:      make(map[string]*CacheValue),
		defTimeout: DEFAULT_CACHE_TIMEOUT,
	}
}

func (c *K8sClientCache) Put(key string, value any) {
	cacheValue := newCacheValue(value, 0)
	if key == CACHE_KEY_API_RESOURCES {
		// hack. Need to change on api level to pass in specific timeout
		// Put(key string, value any, timeout time.Duration)
		cacheValue.timeout = 24 * time.Hour
	}
	c.cache[key] = cacheValue

}

func (c *K8sClientCache) IsTimedOut(value *CacheValue) bool {
	if value == nil {
		return true
	}
	if value.timeout > 0 {
		return time.Since(value.timestamp) > value.timeout
	}
	return time.Since(value.timestamp) > c.defTimeout
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
