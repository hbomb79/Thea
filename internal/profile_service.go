package internal

import "github.com/hbomb79/TPA/internal/profile"

type ProfileService interface {
	GetAllProfiles() []profile.Profile
	CreateProfile(profile.Profile) error
	GetProfileByTag(string) profile.Profile
	DeleteProfileByTag(string) error
	MoveProfile(string, int) error
}

type profileService struct {
	tpa TPA
}

func (service *profileService) GetAllProfiles() []profile.Profile {
	return service.tpa.profiles().Profiles()
}

func (service *profileService) CreateProfile(profile profile.Profile) error {
	return service.tpa.profiles().InsertProfile(profile)
}

func (service *profileService) GetProfileByTag(tag string) profile.Profile {
	_, profile := service.tpa.profiles().FindProfileByTag(tag)
	return profile
}

func (service *profileService) DeleteProfileByTag(tag string) error {
	return service.tpa.profiles().RemoveProfile(tag)
}

func (service *profileService) MoveProfile(tag string, position int) error {
	return service.tpa.profiles().MoveProfile(tag, position)
}

func NewProfileService(tpa TPA) ProfileService {
	return &profileService{
		tpa: tpa,
	}
}
