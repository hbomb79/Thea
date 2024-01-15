package auth

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/api/jwt"
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
		RefreshTokens(allegedRefreshToken string) (*http.Cookie, *http.Cookie, error)
		GenerateTokenCookies(userID uuid.UUID) (*http.Cookie, *http.Cookie, error)
		GetAuthenticatedUserFromContext(ec echo.Context) (*jwt.AuthenticatedUser, error)
		RevokeTokensInContext(ec echo.Context) (*http.Cookie, *http.Cookie)
		RevokeAllForUser(userID uuid.UUID) (*http.Cookie, *http.Cookie)
	}

	AuthController struct {
		store        Store
		authProvider AuthProvider
	}
)

func New(authProvider AuthProvider, store Store) *AuthController {
	return &AuthController{store, authProvider}
}

// Login accepts a POST request containing the
// alleged username and password in the body and:
//   - Asserts that the user with the username provided exists
//   - The provided password is valid
//   - Generates an auth token, and a refresh token, and stores
//     these in the requests cookies
func (controller *AuthController) Login(ec echo.Context, request gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	user, err := controller.store.GetUserWithUsernameAndPassword([]byte(request.Body.Username), []byte(request.Body.Password))
	if err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return nil, errUnauthorized
	}

	authTokenCookie, refreshTokenCookie, err := controller.authProvider.GenerateTokenCookies(user.ID)
	if err != nil {
		log.Warnf("Failed to authenticate due to error: %v\n", err)
		return nil, errUnauthorized
	}
	return LoginResponse{User: userToDto(user), AuthToken: *authTokenCookie, RefreshToken: *refreshTokenCookie}, nil
}

func (controller *AuthController) LogoutSession(ec echo.Context, request gen.LogoutSessionRequestObject) (gen.LogoutSessionResponseObject, error) {
	auth, refresh := controller.authProvider.RevokeTokensInContext(ec)
	return SetTokenCookiesResponse{*auth, *refresh}, nil
}

func (controller *AuthController) LogoutAll(ec echo.Context, request gen.LogoutAllRequestObject) (gen.LogoutAllResponseObject, error) {
	user, err := controller.authProvider.GetAuthenticatedUserFromContext(ec)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err)
	}

	authTokenCookie, refreshTokenCookie := controller.authProvider.RevokeAllForUser(user.UserID)
	return SetTokenCookiesResponse{*authTokenCookie, *refreshTokenCookie}, nil
}

// Refresh allows a client to obtain a new auth and Refresh token by
// providing a valid Refresh token. The new tokens are added
// to the responses cookies, same as login.
func (controller *AuthController) Refresh(ec echo.Context, request gen.RefreshRequestObject) (gen.RefreshResponseObject, error) {
	cookieToken, err := ec.Cookie(jwt.RefreshTokenCookieName)
	if err != nil {
		return nil, echo.ErrUnauthorized
	}

	authTokenCookie, refreshTokenCookie, err := controller.authProvider.RefreshTokens(cookieToken.Value)
	if err != nil {
		log.Errorf("Failed to refresh: %s\n", err)
		return nil, echo.ErrForbidden
	}

	return SetTokenCookiesResponse{*authTokenCookie, *refreshTokenCookie}, nil
}

func (controller *AuthController) GetCurrentUser(ec echo.Context, request gen.GetCurrentUserRequestObject) (gen.GetCurrentUserResponseObject, error) {
	authUser, err := controller.authProvider.GetAuthenticatedUserFromContext(ec)
	if err != nil {
		log.Errorf("Failed to get current user due to error %v\n", err)
		return nil, errUnauthorized
	}

	u, err := controller.store.GetUserWithID(authUser.UserID)
	if err != nil {
		log.Errorf("Failed to get current user due to error: %v\n", err)
		return nil, errUnauthorized
	}

	return gen.GetCurrentUser200JSONResponse(userToDto(u)), nil
}
