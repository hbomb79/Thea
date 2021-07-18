package processor

type queueCache struct {
	location string
	items    map[string]string
}

func (cache *queueCache) loadFromFile() error {
	return nil
}

func (cache *queueCache) save() error {
	return nil
}

func (cache *queueCache) isIn(path string) bool {
	return false
}

func (cache *queueCache) push(path string) error {
	return nil
}
