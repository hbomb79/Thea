package users

import (
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/user"
)

func userToDto(user *user.User) gen.User {
	return gen.User{
		Id:          user.ID,
		Username:    user.Username,
		Permissions: user.Permissions,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LastLogin:   user.LastLoginAt,
		LastRefresh: user.LastRefreshAt,
	}
}
