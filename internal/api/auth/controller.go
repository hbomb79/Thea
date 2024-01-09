package auth

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/user"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

type (
	Store interface {
		RecordUserLogin(userID uuid.UUID) error
		RecordUserRefresh(userID uuid.UUID) error
		GetUserWithUsernameAndPassword(username []byte, rawPassword []byte) (*user.User, error)
		GetUserWithID(ID uuid.UUID) (*user.User, error)
	}

	LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	RefreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}

	Controller struct {
		store   Store
		jwtAuth *jwtAuthProvider
	}
)

func New(store Store) *Controller {
	return &Controller{store, newJwtAuth(store, "myspecialsecret", "myevenmorespecialsecret")}
}

func (controller *Controller) GetJwtRefreshMiddleware() echo.MiddlewareFunc {
	return controller.jwtAuth.jwtTokenRefresherMiddleware
}

func (controller *Controller) GetJwtVerifierMiddleware() echo.MiddlewareFunc {
	return echojwt.WithConfig(echojwt.Config{
		SigningKey:   []byte(controller.jwtAuth.authTokenSecret),
		TokenLookup:  fmt.Sprintf("cookie:%s", authTokenCookieName),
		ErrorHandler: func(ec echo.Context, err error) error { return echo.NewHTTPError(http.StatusUnauthorized) },
	})
}

func (controller *Controller) SetRoutes(eg *echo.Group) {
	eg.POST("/login/", controller.login)
	eg.POST("/refresh/", controller.refresh)
	eg.GET("/current-user/", controller.currentUser, controller.GetJwtVerifierMiddleware())
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

	if err := controller.jwtAuth.generateTokensAndSetCookies(ec, user.ID); err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return errUnauthorized
	}

	return ec.JSON(http.StatusOK, user)
}

// refresh allows a client to obtain a new auth and refresh token by
// providing a valid refresh token. The new tokens are stored
// in the requests cookies, same as login.
func (controller *Controller) refresh(ec echo.Context) error {
	if err := controller.jwtAuth.refresh(ec); err != nil {
		log.Errorf("Failed to refresh: %s\n", err)
		return errUnauthorized
	}

	return ec.NoContent(http.StatusOK)
}

func (controller *Controller) currentUser(ec echo.Context) error {
	userID, err := controller.jwtAuth.getUserIDFromContext(ec)
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
