package media

type MediaStore struct{}

// GetAllSourcePaths returns all the source paths related
// to media that is currently known to Thea by polling the database.
func (store *MediaStore) GetAllSourcePaths() []string {
	return make([]string, 0)
}
