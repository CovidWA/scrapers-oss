package csg

import (
	"math"
	"sync"
	"time"
)

//simple im memory cache with TTL and limited use count

var cacheInstance *CacheInstance
var cacheSingletonLock = new(sync.Mutex)

var Cache = initCache()

type CacheInstance struct {
	entries    map[string]*CacheEntry
	globalLock *sync.Mutex
}

func initCache() *CacheInstance {
	cacheSingletonLock.Lock()
	defer cacheSingletonLock.Unlock()

	if cacheInstance == nil {
		cacheInstance = new(CacheInstance)
		cacheInstance.entries = make(map[string]*CacheEntry)
		cacheInstance.globalLock = new(sync.Mutex)
	}

	return cacheInstance
}

type CacheEntry struct {
	Value    interface{}
	Expiry   int64
	UsesLeft int
	Lock     *sync.Mutex
}

func newCacheEntry() *CacheEntry {
	newEntry := new(CacheEntry)
	newEntry.Value = nil
	newEntry.Expiry = math.MaxInt64
	newEntry.UsesLeft = math.MaxInt32
	newEntry.Lock = new(sync.Mutex)

	return newEntry
}

func (c *CacheInstance) getOrCreate(key string) *CacheEntry {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()

	var entry *CacheEntry

	if _, exists := c.entries[key]; !exists {
		c.entries[key] = newCacheEntry()
	}
	entry = c.entries[key]

	return entry
}

func (c *CacheInstance) GetOrLock(key string) interface{} {
	entry := c.getOrCreate(key)

	entry.Lock.Lock()

	if entry.Value == nil || entry.UsesLeft <= 0 || entry.Expiry < time.Now().Unix() {
		entry.Value = nil
		return nil //return with lock held, presumably caller will put and unlock
	} else {
		defer entry.Lock.Unlock()
		entry.UsesLeft--
		return entry.Value
	}
}

func (c *CacheInstance) UsesLeft(key string) int {
	entry := c.getOrCreate(key)

	entry.Lock.Lock()
	defer entry.Lock.Unlock()

	if entry.UsesLeft <= 0 {
		return 0
	} else {
		return entry.UsesLeft
	}
}

func (c *CacheInstance) Clear(key string) interface{} {
	entry := c.getOrCreate(key)

	entry.Lock.Lock()
	defer entry.Lock.Unlock()

	value := entry.Value
	entry.Value = nil

	return value
}

func (c *CacheInstance) Unlock(key string) {
	entry := c.getOrCreate(key)

	entry.Lock.Unlock()
}

func (c *CacheInstance) Put(key string, value interface{}, ttl int64, useLimit int) {
	entry := c.getOrCreate(key)

	entry.Value = value

	if ttl > 0 {
		entry.Expiry = time.Now().Unix() + ttl
	} else {
		entry.Expiry = math.MaxInt64
	}

	if useLimit > 0 {
		entry.UsesLeft = useLimit
	} else {
		entry.UsesLeft = math.MaxInt32
	}
}

func (c *CacheInstance) Destroy() {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()

	c.entries = make(map[string]*CacheEntry)
}
