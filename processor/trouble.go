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
	Message  string
	Err      error
	Item     *QueueItem `json:"-"`
	Args     map[string]string
	Tag      troubleTag
	resolver troubleResolver
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
