package processor

// When a processor task encounters an error that requires
// user intervention to continue - a 'trouble' is raised.
// This trouble is raised, and resolved, via the 'Trouble'
// struct. This struct mainly acts as a way for the
// task to continue working on other items whilst
// keeping track of the trouble(s) that are pending
type Trouble interface {
	Args() map[string]string
	Resolve(map[string]interface{}) error
	Item() *QueueItem
	Type() TroubleType
	ResolutionContext() map[string]interface{}
}
