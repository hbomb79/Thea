package profile

import (
	"fmt"
	"github.com/floostack/transcoder/ffmpeg"
	"sync"
)

type Profile interface {
	Targets() []*Target
	InsertTarget(*Target) error
	MoveTarget(string, int) error
	EjectTarget(string) error
	FindTarget(string) *Target
	Tag() string
}

type Target struct {
	Label          string          `mapstructure:"label" json:"label"`
	FFmpegOptions  *ffmpeg.Options `mapstructure:"command" json:"command"`
	ThreadBlocking bool            `mapstructure:"blocking" json:"blocking"`
}

func NewTarget(label string) *Target {
	return &Target{
		Label:         label,
		FFmpegOptions: &ffmpeg.Options{},
	}
}

type profile struct {
	sync.Mutex
	FfmpegTargets []*Target `mapstructure:"targets" json:"targets"`
	ProfileTag    string    `mapstructure:"tag" json:"tag"`
}

// NewProfile accepts a single string argument (tag) and returns a new profile
// be reference to the caller with it's internal targets and tag set.
func NewProfile(tag string) Profile {
	return &profile{
		FfmpegTargets: make([]*Target, 0),
		ProfileTag:    tag,
	}
}

// Tag returns the profiles tag (name)
func (profile *profile) Tag() string {
	return profile.ProfileTag
}

// Targets returns the profiles available ffmpeg targets
func (profile *profile) Targets() []*Target {
	return profile.FfmpegTargets
}

// InsertTarget accepts a single Target as an argument, and will append this
// target to the profile.
func (profile *profile) InsertTarget(t *Target) error {
	profile.Lock()
	defer profile.Unlock()

	if idx, _ := profile.find(t.Label); idx != -1 {
		return fmt.Errorf("InsertTarget failed: cannot insert new target with label %v as this label already exists inside this profile.", t.Label)
	}

	profile.FfmpegTargets = append(profile.FfmpegTargets, t)
	return nil
}

func (profile *profile) FindTarget(label string) *Target {
	_, v := profile.find(label)
	return v
}

// MoveTarget accepts a target label to move, and a desiredIndex which it will
// be moved to. Error returned if the label specifies a target that cannot be found,
// or if desiredIndex is out of the profile list bounds.
func (profile *profile) MoveTarget(label string, desiredIndex int) error {
	index, target := profile.find(label)
	if index == -1 {
		return fmt.Errorf("MoveTarget failed: cannot move target with label %v as target cannot be found.", label)
	}
	if desiredIndex < 0 || desiredIndex >= len(profile.FfmpegTargets) {
		return fmt.Errorf("MoveTarget failed: cannot move target to index %d as destination index is out of bounds.", desiredIndex)
	}

	profile.Lock()
	defer profile.Unlock()

	l := append(profile.FfmpegTargets[:index], profile.FfmpegTargets[index+1:len(profile.FfmpegTargets)]...)
	profile.FfmpegTargets = append(l[:desiredIndex+1], l[desiredIndex:]...)
	profile.FfmpegTargets[desiredIndex] = target

	return nil
}

// EjectTarget accepts a single integer paramater (index) and removes the Target at this
// position in the profile targets list if the index provided is legal.
func (profile *profile) EjectTarget(label string) error {
	index, _ := profile.find(label)
	if index < 0 {
		return fmt.Errorf("EjectTarget failed: cannot eject target with label %v as it does not exist", label)
	}

	profile.Lock()
	defer profile.Unlock()

	profile.FfmpegTargets = append(profile.FfmpegTargets[:index], profile.FfmpegTargets[index+1:len(profile.FfmpegTargets)]...)

	return nil
}

func (profile *profile) find(label string) (int, *Target) {
	for k, v := range profile.FfmpegTargets {
		if v.Label == label {
			return k, v
		}
	}

	return -1, nil
}
