package profile

import (
	"fmt"
	"sync"
)

type ProfileFindCallback func(Profile) bool
type ProfileList interface {
	Profiles() []Profile
	InsertProfile(Profile) error
	RemoveProfile(string) error
	FindProfile(ProfileFindCallback) (int, Profile)
	FindProfileByTag(tag string) (int, Profile)
	MoveProfile(string, int) error
}

type safeList struct {
	sync.Mutex
	profiles []Profile
}

// NewList returns a new instance of profileList by address, with
// the slide of Profile instances created ready for use.
func NewList() ProfileList {
	return &safeList{
		profiles: make([]Profile, 0),
	}
}

// Profiles returns an array of all the profiles
// currently stored in this list.
func (list *safeList) Profiles() []Profile {
	return list.profiles
}

// InsertProfile accepts a Profile and appends it to this list, therefore
// making this profile available to the Processor. This method will take control
// of the composed Mutex lock while procesing this command.
func (list *safeList) InsertProfile(p Profile) error {
	if idx, _ := list.FindProfileByTag(p.Tag()); idx != -1 {
		return fmt.Errorf("InsertProfile failed: profile with this tag (%s) already exists", p.Tag())
	}

	list.Lock()
	defer list.Unlock()

	list.profiles = append(list.profiles, p)
	return nil
}

// RemoveProfile accepts an 'tag', searches for a profile in this ProfileList
// that matches the tag provided, and ejects it from the list
func (list *safeList) RemoveProfile(tag string) error {
	idx, _ := list.FindProfileByTag(tag)

	if idx == -1 {
		return fmt.Errorf("RemoveProfile failed: no profile with tag %s exists", tag)
	}

	list.Lock()
	defer list.Unlock()

	list.profiles = append(list.profiles[:idx], list.profiles[idx+1:len(list.profiles)]...)
	return nil
}

// MoveProfile accepts a string (tag) and an int (desiredIndex) paramater. The method
// moves the target (identified by the tag) to the 'desiredIndex' providing both the tag refers to a Profile that
// exists, and the desiredIndex is a legal index
func (list *safeList) MoveProfile(tag string, desiredIndex int) error {
	index, _ := list.FindProfileByTag(tag)
	if index == -1 {
		return fmt.Errorf("MoveProfile failed: tag refers to Profile that does not exist", index)
	} else if desiredIndex < 0 || desiredIndex >= len(list.profiles) {
		return fmt.Errorf("MoveProfile failed: cannot move target to index %d as destination index is out of bounds.", desiredIndex)
	}

	list.Lock()
	defer list.Unlock()

	target := list.profiles[index]
	l := append(list.profiles[:index], list.profiles[index+1:len(list.profiles)]...)

	list.profiles = append(l[:desiredIndex+1], l[desiredIndex:]...)
	list.profiles[desiredIndex] = target

	return nil
}

// FindProfile iterates over each profile stored inside this list
// and calls the 'cb' provided, passing in the Profile at that current iteration.
// Once the return from 'cb' is true, the iteration stops at the current Profile is returned
// to the caller.
// This method will take control of the mutex lock before searching for a profile
// to avoid searching while data is being manipulated elsewherre
func (list *safeList) FindProfile(cb ProfileFindCallback) (int, Profile) {
	list.Lock()
	defer list.Unlock()

	for index, currentProfile := range list.profiles {
		if cb(currentProfile) {
			return index, currentProfile
		}
	}

	return -1, nil
}

// FindProfileByTag is essentially shorthand for calling FindProfile and passing
// a simple callback that returns true if the tag of the Profile matches a tag provided.
func (list *safeList) FindProfileByTag(tag string) (int, Profile) {
	return list.FindProfile(func(p Profile) bool {
		return p.Tag() == tag
	})
}
