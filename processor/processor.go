package processor

type Processor struct {
	Config TPAConfig
}

/**
 * Instantiates a new processor by creating the
 * bare struct, and loading in the configuration
 */
func New() (proc Processor) {
	proc = Processor{}
	proc.Config.LoadConfig()

	return
}
