package auth

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/user"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

var (
	errUnauthorized = echo.NewHTTPError(http.StatusUnauthorized)
	log             = logger.Get("AuthController")
)

type (
	Store interface {
		RecordUserLogin(userID uuid.UUID) error
		RecordUserRefresh(userID uuid.UUID) error
		GetUserWithUsernameAndPassword(username []byte, rawPassword []byte) (*user.User, error)
		GetUserWithID(ID uuid.UUID) (*user.User, error)
	}

	AuthProvider interface {
		GetAuthenticatedMiddleware() echo.MiddlewareFunc
		GetPermissionAuthorizerMiddleware(requiredPermissions []string) echo.MiddlewareFunc
		RefreshTokens(echo.Context) error
		GenerateTokensAndSetCookies(ec echo.Context, userID uuid.UUID) error
		GetUserIDFromContext(ec echo.Context) (*uuid.UUID, error)
		RevokeTokensInContext(ec echo.Context)
		RevokeAllForUser(userID uuid.UUID) error
	}

	LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	Controller struct {
		store        Store
		authProvider AuthProvider
	}
)

func New(authProvider AuthProvider, store Store) *Controller {
	return &Controller{store, authProvider}
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/login/", controller.login)
	eg.POST("/refresh/", controller.refresh)

	authenticated := controller.authProvider.GetAuthenticatedMiddleware()
	eg.GET("/logout/", controller.logoutSession, authenticated)
	eg.GET("/logout-all/", controller.logoutAll, authenticated)
	eg.GET("/current-user/", controller.currentUser, authenticated)
}

// login accepts a POST request containing the
// alleged username and password in the body and:
//   - Asserts that the user with the username provided exists
//   - The provided password is valid
//   - Generates an auth token, and a refresh token, and stores
//     these in the requests cookies
func (controller *Controller) login(ec echo.Context) error {
	var request LoginRequest
	if err := ec.Bind(&request); err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return errUnauthorized
	}

	user, err := controller.store.GetUserWithUsernameAndPassword([]byte(request.Username), []byte(request.Password))
	if err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return errUnauthorized
	}

	if err := controller.authProvider.GenerateTokensAndSetCookies(ec, user.ID); err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return errUnauthorized
	}

	return ec.JSON(http.StatusOK, user)
}

func (controller *Controller) logoutSession(ec echo.Context) error {
	controller.authProvider.RevokeTokensInContext(ec)
	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) logoutAll(ec echo.Context) error {
	id, err := controller.authProvider.GetUserIDFromContext(ec)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	controller.authProvider.RevokeAllForUser(*id)
	return ec.NoContent(http.StatusOK)
}

// refresh allows a client to obtain a new auth and refresh token by
// providing a valid refresh token. The new tokens are stored
// in the requests cookies, same as login.
func (controller *Controller) refresh(ec echo.Context) error {
	if err := controller.authProvider.RefreshTokens(ec); err != nil {
		log.Errorf("Failed to refresh: %s\n", err)
		return errUnauthorized
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) currentUser(ec echo.Context) error {
	userID, err := controller.authProvider.GetUserIDFromContext(ec)
	if err != nil {
		log.Errorf("Failed to get current user due to error %v\n", err)
		return errUnauthorized
	}

	u, err := controller.store.GetUserWithID(*userID)
	if err != nil {
		log.Errorf("Failed to get current user due to error: %v\n", err)
		return errUnauthorized
	}

	return ec.JSON(http.StatusOK, u)
}
