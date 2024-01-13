package auth

import (
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/user"
)

func userToDto(u *user.User) gen.User {
	return gen.User{
		Id:          u.ID,
		Username:    u.Username,
		Permissions: u.Permissions,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		LastLogin:   u.LastLoginAt,
		LastRefresh: u.LastRefreshAt,
	}
}
