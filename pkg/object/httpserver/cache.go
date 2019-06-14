package httpserver

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/megaease/easegateway/pkg/logger"
)

type (
	cache struct {
		arc *lru.ARCCache
	}

	cacheItem struct {
		cached bool

		notFound         bool
		methodNotAllowed bool
		backend          string
	}
)

func newCache(size uint32) *cache {
	arc, err := lru.NewARC(int(size))
	if err != nil {
		logger.Errorf("BUG: new arc cache failed: %v", err)
		return nil
	}

	return &cache{
		arc: arc,
	}
}

func (c *cache) get(key string) *cacheItem {
	value, ok := c.arc.Get(key)
	if !ok {
		return nil
	}

	return value.(*cacheItem)
}

func (c *cache) put(key string, ci *cacheItem) {
	c.arc.Add(key, ci)
}