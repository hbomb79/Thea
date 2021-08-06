package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type cacheItemKey string
type cacheItemValue map[string]interface{}

// The Cache is a struct from the cache package that allows other parts
// of the TPA to store persistent information about items inside the queue.
// The primary use case for this cache is to allow us to store the current
// status of queue items to enable persistent memory over server-restarts
type Cache struct {
	filePath string
	content  map[cacheItemKey]cacheItemValue
}

// New constructs a new instance of a Cache struct, setting the
// location for loading/saving to the path provided. If a file already
// exists at the path provided, we will attempt to load the cache
// from that file. Errors resulting from a malformed/missing cache
// are handled by constructing a blank cache and overwriting this data.
func New(path string) *Cache {
	c := &Cache{
		filePath: path,
		content:  make(map[cacheItemKey]cacheItemValue),
	}

	if err := c.load(); err != nil {
		fmt.Printf("[Cache] (!!) Failed to load preexisting cache content from file: " + err.Error() + ". Defaulting to empty cache.\n")
	}

	return c
}

// HasKey will check the cache for an item in it's content with a matching key.
// True is returned if found, false otherwise.
func (cache *Cache) HasKey(key cacheItemKey) bool {
	_, ok := cache.content[key]

	return ok
}

// RetriveItem will return the value for a cache item at the key provided.
// If no item exists at the key provided, nil is returned.
func (cache *Cache) RetriveItem(key cacheItemKey) cacheItemValue {
	v, ok := cache.content[key]
	if !ok {
		return nil
	}

	return v
}

// PushItem will store new data at the key provided and saves the new cache data
// to file via 'save'.
// Note: This method will *overwrite* data already stored at the given key.
func (cache *Cache) PushItem(key cacheItemKey, content cacheItemValue) {
	cache.content[key] = content
}

// DeleteItem will remove cache data at the key provided, returning true
// if an item was deleted and false if there was no data to delete.
func (cache *Cache) DeleteItem(key cacheItemKey) bool {
	if !cache.HasKey(key) {
		return false
	}

	delete(cache.content, key)
	return true
}

// load is a private method that will load the cache data from the 'filePath' and
// attempt to unmarshal the string content back to the map[string]interface{}. If
// an error occurs (JSON failure, file not found, etc), it will be returned from this method.
func (cache *Cache) load() error {
	handle, err := os.Open(cache.filePath)
	if err != nil {
		return err
	}
	defer handle.Close()

	fileCnt, err := ioutil.ReadAll(handle)
	json.Unmarshal(fileCnt, &cache.content)
	return nil
}

// save is a private method that will encode the data for this cache to JSON
// and saves the output string to the 'filePath' of this Cache.
func (cache *Cache) save() error {
	handle, err := os.OpenFile(cache.filePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	defer handle.Close()

	cnt, err := json.Marshal(cache.content)
	if err != nil {
		return err
	}

	if _, err = handle.Write(cnt); err != nil {
		return err
	}

	return nil
}
