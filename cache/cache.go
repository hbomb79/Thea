package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// The Cache is a struct from the cache package that allows other parts
// of the TPA to store persistent information about items inside the queue.
// The primary use case for this cache is to allow us to store the current
// status of queue items to enable persistent memory over server-restarts
type Cache struct {
	filePath string
	content  map[string]interface{}
}

// New constructs a new instance of a Cache struct, setting the
// location for loading/saving to the path provided. If a file already
// exists at the path provided, we will attempt to load the cache
// from that file. Errors resulting from a malformed/missing cache
// are handled by constructing a blank cache and overwriting this data.
func New(path string) *Cache {
	if path == "" {
		panic("[Cache] (!!) Cannot instantiate new Cache - provided filesystem path is empty!")
	}

	c := &Cache{
		filePath: path,
		content:  make(map[string]interface{}),
	}

	c.Load()
	return c
}

// HasItem will check the cache for an item in it's content with a matching key.
// True is returned if found, false otherwise.
func (cache *Cache) HasItem(key string) bool {
	_, ok := cache.content[key]

	return ok
}

// RetriveItem will return the value for a cache item at the key provided.
// If no item exists at the key provided, nil is returned.
func (cache *Cache) RetriveItem(key string) interface{} {
	v, ok := cache.content[key]
	if !ok {
		return nil
	}

	return v
}

// PushItem will store new data at the key provided and saves the new cache data
// to file via 'save'.
// Note: This method will *overwrite* data already stored at the given key.
func (cache *Cache) PushItem(key string, content interface{}) {
	cache.content[key] = content
	cache.Save()
}

// DeleteItem will remove cache data at the key provided, returning true
// if an item was deleted and false if there was no data to delete.
func (cache *Cache) DeleteItem(key string) bool {
	if !cache.HasItem(key) {
		return false
	}

	delete(cache.content, key)
	cache.Save()
	return true
}

// IterItems runs the provided callback function for each item
// in the caches content map as long as the callback returns true.
// If the callback returns false, it essentially 'breaks' the loop and
// this method will return.
func (cache *Cache) IterItems(cb func(*Cache, string, interface{}) bool) {
	for k, v := range cache.content {
		if !cb(cache, k, v) {
			return
		}
	}
}

// Save will attempt to save the cache to file, errors will be reported in console
// This calls the private 'save' method to execute the save, this method just acts
// as a public wrapper
func (cache *Cache) Save() {
	if err := cache.save(); err != nil {
		fmt.Printf("[Cache] (!!) Failed to save cache! %s\n", err.Error())
	}
}

// Load will check the local filesystem (cache.filePath) for an existing cache
// and loads it in to memory if found. If not found, an empty cache map is
// constructed which can be saved to the filesystem with 'Save'
func (cache *Cache) Load() {
	if err := cache.load(); err != nil {
		fmt.Printf("[Cache] (!!) Unable to load cache file(%s): %s. Using empty cache!\n", cache.filePath, err.Error())

		cache.content = make(map[string]interface{})
	}
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
