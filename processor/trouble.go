package processor

type troubleResolver func(*Trouble, map[string]interface{}) error
type taskErrorHandler func(*QueueItem, error) error
type troubleTag int

const (
	TitleFailure troubleTag = iota
	OmdbResponseFailure
	OmdbMultipleOptions
	FormatError
)

// When a processor task encounters an error that requires
// user intervention to continue - a 'trouble' is raised.
// This trouble is raised, and resolved, via the 'Trouble'
// struct. This struct mainly acts as a way for the
// task to continue working on other items whilst
// keeping track of the trouble(s) that are pending
type Trouble struct {
	msg      string
	err      error
	item     *QueueItem `json:"-"`
	args     map[string]string
	resolver troubleResolver
	tag      troubleTag
}

// validate accepts a map of arguments and checks to ensure
// that all the arguments required by this trouble instance
// are present. Returns an error if not.
func (trouble *Trouble) validate(args map[string]interface{}) error {
	return nil
}

// Resolve is a method that is used to initiate the resolution of
// a trouble instance. The args provided are first validated before
// being passed to the Trouble's 'resolver' for processing.
func (trouble *Trouble) Resolve(args map[string]interface{}) error {
	if err := trouble.validate(args); err != nil {
		return err
	}

	return trouble.resolver(trouble, args)
}

// Args returns the arguments required by this trouble
// in order to resolve this trouble instance.
func (trouble *Trouble) Args() map[string]string {
	return trouble.args
}

// Tag returns the 'tag' for this trouble, which is used by
// resolving functions to tell which type of trouble they've received.
func (trouble *Trouble) Tag() troubleTag {
	return trouble.tag
}

// Item returns the QueueItem that this trouble is attached to
func (trouble *Trouble) Item() *QueueItem {
	return trouble.item
}

// Message returns the text that describes this trouble case
func (trouble *Trouble) Message() string {
	return trouble.msg
}

// Err returns the underlying error that triggered this trouble
func (trouble *Trouble) Err() error {
	return trouble.err
}
