package users

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/user"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		ListUsers() ([]*user.User, error)
		GetUserWithID(userID uuid.UUID) (*user.User, error)
		UpdateUserPermissions(userID uuid.UUID, newPermissions []string) error
	}

	UpdatePermissionsRequest struct {
		Permissions []string `json:"permissions"`
	}

	controller struct{ store Store }
)

func NewController(store Store) *controller { return &controller{store} }

func (controller *controller) SetRoutes(eg *echo.Group) {
	eg.GET("/", controller.list)
	eg.GET("/:id/", controller.get)
	eg.POST("/:id/permissions/", controller.updatePermissions)
}

func (controller *controller) list(ec echo.Context) error {
	users, err := controller.store.ListUsers()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ec.JSON(http.StatusOK, users)
}

func (controller *controller) get(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "User ID is not a valid UUID")
	}

	user, err := controller.store.GetUserWithID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ec.JSON(http.StatusOK, user)
}

func (controller *controller) updatePermissions(ec echo.Context) error {
	id, err := uuid.Parse(ec.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "User ID is not a valid UUID")
	}

	var request UpdatePermissionsRequest
	if err := ec.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("JSON body invalid: %s", err))
	}

	if err := controller.store.UpdateUserPermissions(id, request.Permissions); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to apply new permissions for user: %s", err))
	}

	return ec.NoContent(http.StatusOK)
}
