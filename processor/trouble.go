package processor

type TroubleType int

const (
	Warning TroubleType = iota
	Error
	Fatal
)

type Trouble struct {
	Message     string
	Type        TroubleType
	ResolveFunc func(*Trouble)
}
