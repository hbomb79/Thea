package internal

import "github.com/hbomb79/Thea/internal/profile"

type ProfileService interface {
	GetAllProfiles() []profile.Profile
	InsertProfile(profile.Profile) error
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

func (service *profileService) InsertProfile(profile profile.Profile) error {
	if err := service.thea.profiles().InsertProfile(profile); err != nil {
		return err
	}

	service.notifyUpdate()
	return nil
}

func (service *profileService) GetProfileByTag(tag string) profile.Profile {
	_, profile := service.thea.profiles().FindProfileByTag(tag)
	return profile
}

func (service *profileService) DeleteProfileByTag(tag string) error {
	if err := service.thea.profiles().RemoveProfile(tag); err != nil {
		return err
	}

	service.notifyUpdate()
	return nil
}

func (service *profileService) MoveProfile(tag string, position int) error {
	if err := service.thea.profiles().MoveProfile(tag, position); err != nil {
		return err
	}

	service.notifyUpdate()
	return nil
}

func (service *profileService) notifyUpdate() {
	service.thea.profiles().Save()
	service.thea.NotifyProfileUpdate()
}

func NewProfileService(thea Thea) ProfileService {
	return &profileService{
		thea: thea,
	}
}
