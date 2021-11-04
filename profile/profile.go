package profile

import (
	"fmt"
	"sync"
)

type Profile interface {
	Targets() []*ffmpegTarget
	InsertTarget(*ffmpegTarget)
	MoveTarget(int, int) error
	EjectTarget(int) error
	Tag() string
}

type ffmpegTarget struct {
	Label string `mapstructure:"label" json:"label"`
}

func NewTarget(label string) *ffmpegTarget {
	return &ffmpegTarget{
		Label: label,
	}
}

type profile struct {
	sync.Mutex
	FfmpegTargets []*ffmpegTarget `mapstructure:"targets" json:"targets"`
	ProfileTag    string          `mapstructure:"tag" json:"tag"`
}

// NewProfile accepts a single string argument (tag) and returns a new profile
// be reference to the caller with it's internal targets and tag set.
func NewProfile(tag string) Profile {
	return &profile{
		FfmpegTargets: make([]*ffmpegTarget, 0),
		ProfileTag:    tag,
	}
}

// Tag returns the profiles tag (name)
func (profile *profile) Tag() string {
	return profile.ProfileTag
}

// Targets returns the profiles available ffmpeg targets
func (profile *profile) Targets() []*ffmpegTarget {
	return profile.FfmpegTargets
}

// InsertTarget accepts a single Target as an argument, and will append this
// target to the profile.
func (profile *profile) InsertTarget(t *ffmpegTarget) {
	profile.Lock()
	defer profile.Unlock()

	profile.FfmpegTargets = append(profile.FfmpegTargets, t)
}

// MoveTarget accepts two integer paramaters; index, and desiredIndex. This method will
// move the target at 'index' to the 'desiredIndex' providing both index and desiredIndex are
// legal list indexes.
func (profile *profile) MoveTarget(index int, desiredIndex int) error {
	if index < 0 || index >= len(profile.FfmpegTargets) {
		return fmt.Errorf("MoveTarget failed: cannot move target at index %d as this index is out of bounds.", index)
	} else if desiredIndex < 0 || desiredIndex >= len(profile.FfmpegTargets) {
		return fmt.Errorf("MoveTarget failed: cannot move target to index %d as destination index is out of bounds.", desiredIndex)
	}

	profile.Lock()
	defer profile.Unlock()

	target := profile.FfmpegTargets[index]
	l := append(profile.FfmpegTargets[:index], profile.FfmpegTargets[index+1:len(profile.FfmpegTargets)]...)

	profile.FfmpegTargets = append(l[:desiredIndex+1], l[desiredIndex:]...)
	profile.FfmpegTargets[desiredIndex] = target

	return nil
}

// EjectTarget accepts a single integer paramater (index) and removes the Target at this
// position in the profile targets list if the index provided is legal.
func (profile *profile) EjectTarget(index int) error {
	if index < 0 || index >= len(profile.FfmpegTargets) {
		return fmt.Errorf("EjectTarget failed: cannot eject target at out-of-bounds index %d", index)
	}

	profile.Lock()
	defer profile.Unlock()

	profile.FfmpegTargets = append(profile.FfmpegTargets[:index], profile.FfmpegTargets[index+1:len(profile.FfmpegTargets)]...)

	return nil
}
