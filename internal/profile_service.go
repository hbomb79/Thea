package internal

import "github.com/hbomb79/Thea/internal/profile"

type ProfileService interface {
	GetAllProfiles() []profile.Profile
	CreateProfile(profile.Profile) error
	GetProfileByTag(string) profile.Profile
	DeleteProfileByTag(string) error
	MoveProfile(string, int) error
}

type profileService struct {
	thea Thea
}

func (service *profileService) GetAllProfiles() []profile.Profile {
	return service.thea.profiles().Profiles()
}

func (service *profileService) CreateProfile(profile profile.Profile) error {
	return service.thea.profiles().InsertProfile(profile)
}

func (service *profileService) GetProfileByTag(tag string) profile.Profile {
	_, profile := service.thea.profiles().FindProfileByTag(tag)
	return profile
}

func (service *profileService) DeleteProfileByTag(tag string) error {
	return service.thea.profiles().RemoveProfile(tag)
}

func (service *profileService) MoveProfile(tag string, position int) error {
	return service.thea.profiles().MoveProfile(tag, position)
}

func NewProfileService(thea Thea) ProfileService {
	return &profileService{
		thea: thea,
	}
}
