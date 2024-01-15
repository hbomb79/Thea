package users

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/util"
	"github.com/hbomb79/Thea/internal/user"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		ListUsers() ([]*user.User, error)
		GetUserWithID(userID uuid.UUID) (*user.User, error)
		UpdateUserPermissions(userID uuid.UUID, newPermissions []string) error
	}

	UserController struct{ store Store }
)

func NewController(store Store) *UserController {
	return &UserController{store: store}
}

func (controller *UserController) ListUsers(ec echo.Context, _ gen.ListUsersRequestObject) (gen.ListUsersResponseObject, error) {
	users, err := controller.store.ListUsers()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return gen.ListUsers200JSONResponse(util.ApplyConversion(users, userToDto)), nil
}

func (controller *UserController) GetUser(ec echo.Context, request gen.GetUserRequestObject) (gen.GetUserResponseObject, error) {
	user, err := controller.store.GetUserWithID(request.Id)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return gen.GetUser200JSONResponse(userToDto(user)), nil
}

func (controller *UserController) UpdateUserPermissions(ec echo.Context, request gen.UpdateUserPermissionsRequestObject) (gen.UpdateUserPermissionsResponseObject, error) {
	if err := controller.store.UpdateUserPermissions(request.Id, request.Body.Permissions); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to apply new permissions for user: %s", err))
	}

	return gen.UpdateUserPermissions200Response{}, nil
}
