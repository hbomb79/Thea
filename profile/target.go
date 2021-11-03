package profile

import (
	"encoding/json"
)

type Target interface {
	Label() string
}

type ffmpegTarget struct {
	label string
}

func (target *ffmpegTarget) Label() string {
	return ""
}

func NewTarget(label string) Target {
	return &ffmpegTarget{
		label: label,
	}
}

func (target *ffmpegTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Label string `json:"tag"`
	}{
		target.Label(),
	})
}
