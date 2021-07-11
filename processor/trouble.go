package processor

type TroubleType int
type TroubleResolve func(map[string]interface{}) error

const (
	Warning TroubleType = iota
	Error
	Fatal
)

type QueueTrouble struct {
	Title     string                 `json:"title"`
	Details   string                 `json:"details"`
	Arguments map[string]interface{} `json:"arguments"`
	Type      TroubleType            `json:"type"`
	Resolve   TroubleResolve         `json:"-"`
}
