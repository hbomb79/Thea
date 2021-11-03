package profile

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Profile interface {
	Targets() []Target
	InsertTarget(Target)
	MoveTarget(int, int) error
	EjectTarget(int) error
	Tag() string
}

type profile struct {
	sync.Mutex
	targets []Target
	tag     string
}

// NewProfile accepts a single string argument (tag) and returns a new profile
// be reference to the caller with it's internal targets and tag set.
func NewProfile(tag string) Profile {
	return &profile{
		targets: make([]Target, 0),
		tag:     tag,
	}
}

// Tag returns the profiles tag (name)
func (profile *profile) Tag() string {
	return profile.tag
}

// Targets returns the profiles available ffmpeg targets
func (profile *profile) Targets() []Target {
	return profile.targets
}

// InsertTarget accepts a single Target as an argument, and will append this
// target to the profile.
func (profile *profile) InsertTarget(t Target) {
	profile.Lock()
	defer profile.Unlock()

	profile.targets = append(profile.targets, t)
}

// MoveTarget accepts two integer paramaters; index, and desiredIndex. This method will
// move the target at 'index' to the 'desiredIndex' providing both index and desiredIndex are
// legal list indexes.
func (profile *profile) MoveTarget(index int, desiredIndex int) error {
	if index < 0 || index >= len(profile.targets) {
		return fmt.Errorf("MoveTarget failed: cannot move target at index %d as this index is out of bounds.", index)
	} else if desiredIndex < 0 || desiredIndex >= len(profile.targets) {
		return fmt.Errorf("MoveTarget failed: cannot move target to index %d as destination index is out of bounds.", desiredIndex)
	}

	profile.Lock()
	defer profile.Unlock()

	target := profile.targets[index]
	l := append(profile.targets[:index], profile.targets[index+1:len(profile.targets)]...)

	profile.targets = append(l[:desiredIndex+1], l[desiredIndex:]...)
	profile.targets[desiredIndex] = target

	return nil
}

// EjectTarget accepts a single integer paramater (index) and removes the Target at this
// position in the profile targets list if the index provided is legal.
func (profile *profile) EjectTarget(index int) error {
	if index < 0 || index >= len(profile.targets) {
		return fmt.Errorf("EjectTarget failed: cannot eject target at out-of-bounds index %d", index)
	}

	profile.Lock()
	defer profile.Unlock()

	profile.targets = append(profile.targets[:index], profile.targets[index+1:len(profile.targets)]...)

	return nil
}

func (profile *profile) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tag     string   `json:"tag"`
		Targets []Target `json:"targets"`
	}{
		profile.Tag(),
		profile.Targets(),
	})
}
