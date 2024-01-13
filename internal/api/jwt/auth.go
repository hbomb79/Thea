package jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/api/gen"
	"github.com/hbomb79/Thea/internal/user"
	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/hbomb79/Thea/pkg/sync"
	"github.com/labstack/echo/v4"
	middleware "github.com/oapi-codegen/echo-middleware"
)

var (
	ErrUnknownSecurityScheme   = errors.New("request specifies an unknown security scheme and so cannot be validated")
	ErrAuthTokenMissing        = errors.New("request does not contain required auth token in cookies")
	ErrInsufficientPermissions = errors.New("authenticated user is missing required permissions")

	log = logger.Get("JWT-Auth")
)

const (
	PermissionAuthSecuritySchemeName = "permissionAuth"

	AuthTokenCookieName = "auth-token"
	AuthTokenLifespan   = time.Minute * 30

	RefreshTokenCookieName = "refresh-token"
	RefreshTokenLifespan   = time.Hour * 24 * 30 // 30 days
)

type (
	AuthenticatedUser struct {
		UserID      uuid.UUID
		Permissions []string
	}

	authTokenClaims struct {
		jwt.RegisteredClaims
		Permissions []string  `json:"permissions"`
		UserID      uuid.UUID `json:"user_id"`
	}

	refreshTokenClaims struct {
		jwt.RegisteredClaims
		UserID uuid.UUID `json:"user_id"`
	}

	Store interface {
		RecordUserLogin(userID uuid.UUID) error
		RecordUserRefresh(userID uuid.UUID) error
		GetUserWithUsernameAndPassword(username []byte, rawPassword []byte) (*user.User, error)
		GetUserWithID(ID uuid.UUID) (*user.User, error)
	}

	jwtAuthProvider struct {
		store                  Store
		authTokenSecret        []byte
		refreshTokenSecret     []byte
		refreshTokenCookiePath string

		// This map (acting as a set) is used to keep track of
		// any token which we have explicitly revoked (for example,
		// when a user logs out, the auth and refresh token are revoked).
		//
		// NB: Tokens are removed from this set when they are cleaned up
		// (which happens automatically some time after their expiration).
		blacklistedTokens *sync.TypedSyncMap[string, struct{}]

		// This map is used to keep track of which tokens are currently
		// 'active' for each user. This map is automatically monitored
		// by the auth provider to clear out tokens shortly after they expire.
		// When we wish to revoke all tokens associated with a specific user, we
		// can use this map to fetch the tokens.
		//
		// NB: A token does NOT need to exist here in order to be valid, this
		// is simply a mechanism to track active tokens for the purpose of
		// revocation if requested.
		//
		// NB': Tokens are removed from this map when they are cleaned up
		// (which happens automatically some time after their expiration).
		userTokens *sync.TypedSyncMap[uuid.UUID, []string]
	}
)

// NewJwtAuth creates a new authentication provider which
// uses JWT tokens to authenticate and authorize user actions.
// The constructor accepts a Store which is used for fetching
// user information during token generation, as well as the
// HTTP path which should restrict the transmission of the
// refresh token (it should only be sent to the server when it's going
// to be used).
// Finally, the two secrets which are used to sign the tokens. These two
// secrets should not match, and should be >= 256 bits in size
func NewJwtAuth(store Store, refreshRoutePath string, authTokenSecret []byte, refreshTokenSecret []byte) *jwtAuthProvider {
	return &jwtAuthProvider{
		store,
		authTokenSecret,
		refreshTokenSecret,
		refreshRoutePath,
		new(sync.TypedSyncMap[string, struct{}]),
		new(sync.TypedSyncMap[uuid.UUID, []string])}
}

// generateTokensAndSetCookies generates an auth token and a refresh token
// using the appropriate secrets and expiries, before storing both of the tokens
// in the requests cookies.
func (auth *jwtAuthProvider) GenerateTokenCookies(userID uuid.UUID) (*http.Cookie, *http.Cookie, error) {
	authToken, authTokenExp, err := auth.generateAccessToken(userID)
	if err != nil {
		return nil, nil, err
	}

	refreshToken, refreshTokenExp, err := auth.generateRefreshToken(userID)
	if err != nil {
		return nil, nil, err
	}

	// Don't block the request waiting for these
	go func() {
		if err := auth.store.RecordUserLogin(userID); err != nil {
			log.Warnf("Failed to record user login for %v: %v\n", userID, err)
		}
		if err := auth.store.RecordUserRefresh(userID); err != nil {
			log.Warnf("Failed to record user refresh for %v: %v\n", userID, err)
		}
	}()

	// Update our tracked list of tokens for this user, and schedule cleanup
	// of this token
	actual, loaded := auth.userTokens.LoadOrStore(userID, []string{authToken, refreshToken})
	if loaded {
		auth.userTokens.Store(userID, append(actual, authToken, refreshToken))
	}

	auth.scheduleUserTokenCleanup(userID, authToken, authTokenExp)
	auth.scheduleUserTokenCleanup(userID, refreshToken, refreshTokenExp)

	// Create and return the token cookies to be set in the response
	authTokenCookie := createTokenCookie(AuthTokenCookieName, "/", authToken, authTokenExp)
	refreshTokenCookie := createTokenCookie(RefreshTokenCookieName, auth.refreshTokenCookiePath, refreshToken, refreshTokenExp)
	return authTokenCookie, refreshTokenCookie, nil
}

// GetAuthenticatedUserFromContext provides a way for endpoints
// to extract the users ID and permissions from the context
// of their request. An error will be returned if no valid
// user can be found.
func (auth *jwtAuthProvider) GetAuthenticatedUserFromContext(ec echo.Context) (*AuthenticatedUser, error) {
	u, ok := ec.Get("user").(*AuthenticatedUser)
	if !ok {
		return nil, errors.New("no user found in request context")
	}

	return u, nil
}

// RevokeTokensInContext revokes the auth and refresh token in this
// request context, assuming they are provided. A missing token/cookie
// is ignored.
func (auth *jwtAuthProvider) RevokeTokensInContext(ec echo.Context) {
	if cookie, err := ec.Cookie(AuthTokenCookieName); err == nil && cookie != nil {
		auth.revokeToken(cookie.Value)
	}
	if cookie, err := ec.Cookie(RefreshTokenCookieName); err == nil && cookie != nil {
		auth.revokeToken(cookie.Value)
	}
}

// RevokeAllForUser finds all the tokens we've granted to a specified
// user ID and revokes all of them (if any). This will require that the
// specified user logs in again on all of their devices.
func (auth *jwtAuthProvider) RevokeAllForUser(userID uuid.UUID) error {
	grantedTokens, ok := auth.userTokens.Load(userID)
	if !ok || len(grantedTokens) == 0 {
		return nil
	}

	for _, granted := range grantedTokens {
		auth.revokeToken(granted)
	}

	return nil
}

// RefreshTokens generates new auth and refresh tokens and stores them in
// the request cookies IF the request contains a valid refresh token. The
// new cookies are returned to the caller on success
func (auth *jwtAuthProvider) RefreshTokens(allegedRefreshToken string) (*http.Cookie, *http.Cookie, error) {
	token, err := auth.validateJWT(allegedRefreshToken, auth.refreshTokenSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to refresh: %w", err)
	}

	claims := token.Claims.(*jwt.MapClaims)
	userID, err := auth.getUserIdFromClaims(*claims)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to refresh: %w", err)
	}

	return auth.GenerateTokenCookies(*userID)
}

// getSecurityValidator returns a middleware which uses the generated OpenAPI swagger spec to
// inspect incoming requests and determine whether or not they're
// valid. This includes ensuring the request meets the spec, and that
// the security scheme specified for that request is satisfied (authentication
// by way of JWT token, and authorization by way of permissions)
func (auth *jwtAuthProvider) GetSecurityValidatorMiddleware() echo.MiddlewareFunc {
	spec, err := gen.GetSwagger()
	if err != nil {
		panic(fmt.Sprintf("failed to extract swagger spec from generated spec: %s", err))
	}

	// Clear out the servers array in the spec, this skips validating
	// that server names match. We don't know how this thing will be run.
	// See https://github.com/deepmap/oapi-codegen/issues/882
	spec.Servers = nil

	auth.validateSpecSecurity(spec)

	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Skipper: func(ec echo.Context) bool {
			// We specifically allow OPTION requests to pass through un-encumbered
			// as they are not documented in our OpenAPI spec, and so they will
			// be rejected as 'method not allowed' if we don't skip them.
			//TODO: there is surely a better way to handle this?
			return ec.Request().Method == http.MethodOptions
		},
		ErrorHandler: func(_ echo.Context, err *echo.HTTPError) error {
			// The request validator constructs an Echo HTTPError using
			// the error our AuthenticationFunc returns. This is
			// unacceptable as it reveals far too much information about
			// why the validation failed.
			// Simple fix is to rewrite the error to the HTTP status text for
			// the code - the full error will still be logged as it's stored
			// inside the 'internal' field of the HTTPError.
			err.Message = http.StatusText(err.Code)
			return err
		},
		Options: openapi3filter.Options{AuthenticationFunc: auth.validateTokenFromAuthInput},
	})
}

// validateSpecSecurity ensures that the security requirements of the
// provided OpenAPI spec do not reference any permissions or security schems
// which we don't know about. This would make those endpoints unreachable
// and so is likely a mistake.
func (auth *jwtAuthProvider) validateSpecSecurity(spec *openapi3.T) {
	referencedPermissions := make(map[string]struct{})
	for _, security := range spec.Security {
		if perms, ok := security[PermissionAuthSecuritySchemeName]; ok {
			for _, perm := range perms {
				referencedPermissions[perm] = struct{}{}
			}
		} else {
			panic("validation of OpenAPI spec failed: top-level security types specify one or more disallowed types")
		}
	}
	for _, path := range spec.Paths {
		for _, operation := range path.Operations() {
			if operation.Security != nil {
				for _, security := range *operation.Security {
					if perms, ok := security[PermissionAuthSecuritySchemeName]; ok {
						for _, perm := range perms {
							referencedPermissions[perm] = struct{}{}
						}
					} else {
						panic(fmt.Sprintf("validation of OpenAPI spec failed: security for operation %s specifies one or more disallowed types", operation.OperationID))
					}
				}
			}
		}
	}

	knownPermissions := permissions.All()
	knownPermissionsMap := make(map[string]struct{})
	for _, p := range knownPermissions {
		knownPermissionsMap[p] = struct{}{}
	}
	for referenced := range referencedPermissions {
		if _, ok := knownPermissionsMap[referenced]; !ok {
			panic(fmt.Sprintf("validation of OpenAPI spec failed: permission '%s' is referenced by one or more security requirements, however this permission is not recognized by Thea", referenced))
		}
	}

}

// validateTokenFromAuthInput accepts an OpenAPI authentication input
// and returns an error if we're unable to extract a valid JWT
// from the requests cookies.
// If we CAN extract a valid token, then said token is also
// checked to ensure it contains the correct permissions.
func (auth *jwtAuthProvider) validateTokenFromAuthInput(ctx context.Context, authInput *openapi3filter.AuthenticationInput) error {
	if authInput.SecuritySchemeName != PermissionAuthSecuritySchemeName {
		return ErrUnknownSecurityScheme
	}

	tokenCookie, err := authInput.RequestValidationInput.Request.Cookie(AuthTokenCookieName)
	if err != nil {
		return ErrAuthTokenMissing
	}

	token, err := auth.validateJWT(tokenCookie.Value, auth.authTokenSecret)
	if err != nil {
		return fmt.Errorf("validation of auth token failed: %w", err)
	}

	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok {
		return errors.New("failed to cast JWT claims to MapClaims")
	}

	// Extract user information (ID and permissions) from JWT
	userID, err := auth.getUserIdFromClaims(*claims)
	if err != nil {
		return err
	}

	// Check that the permissiosn specified by the request scopes
	// are all present inside of the users permissions
	userPermissions, err := auth.getPermissionsFromClaims(*claims)
	if err != nil {
		return err
	}
	for _, perm := range authInput.Scopes {
		if !slices.Contains(userPermissions, perm) {
			log.Warnf("User %s failed permissions check while accessing %s: missing permission '%s'\n", userID, authInput.RequestValidationInput.Request.RequestURI, perm)
			return ErrInsufficientPermissions
		}
	}

	// Insert user info inside of request context to allow for
	// endpoint handlers to extract user information
	eCtx := middleware.GetEchoContext(ctx)
	eCtx.Set("user", &AuthenticatedUser{UserID: *userID, Permissions: userPermissions})

	return nil
}

func (auth *jwtAuthProvider) getPermissionsFromClaims(claims jwt.MapClaims) ([]string, error) {
	if permissions, ok := claims["permissions"]; ok {
		perms, ok := permissions.([]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to extract permissions from JWT claims: not of type []string")
		}

		outputPerms := make([]string, len(perms))
		for k, v := range perms {
			p, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("failed to extract permissions from JWT claims: value %v could not be cast to string", v)
			}

			outputPerms[k] = p
		}

		return outputPerms, nil
	} else {
		return nil, errors.New("failed to extract permissions from JWT claims: missing")
	}
}

// validateToken ensures that the provided token is:
//   - signed using the same secret/algorithm as we expect
//   - contains a valid userID
//   - not expired
//   - not blacklisted
func (auth *jwtAuthProvider) validateJWT(token string, secret []byte) (*jwt.Token, error) {
	// Parse token using secret
	tokenClaims := &jwt.MapClaims{}
	tkn, err := jwt.ParseWithClaims(
		token,
		tokenClaims,
		func(token *jwt.Token) (interface{}, error) { return secret, nil },
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	// Fail if the token has expired
	if tkn == nil || !tkn.Valid {
		return nil, errors.New("failed to verify JWT: token is expired or invalid")
	}

	// Ensure the user ID is present
	if _, err := auth.getUserIdFromClaims(*tokenClaims); err != nil {
		return nil, fmt.Errorf("failed to extract userID from JWT: %w", err)
	}

	// Check we haven't revoked this token
	if _, ok := auth.blacklistedTokens.Load(token); ok {
		return nil, errors.New("failed to verify JWT: token has been revoked")
	}

	return tkn, nil
}

// generateAccessToken accepts a userID and generates a short-term token
// which can be used to authenticate against protected API endpoints. This
// token also includes the associated user permissions at the time of
// generation, which allows for the server to restrict access to certain
// endpoints if the user does not have the required permissions.
//
// (Shortly) before this token expires, it is expected that the client will
// refresh their tokens using their refreshToken.
func (auth *jwtAuthProvider) generateAccessToken(userID uuid.UUID) (string, time.Time, error) {
	user, err := auth.store.GetUserWithID(userID)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to fetch user %s during auth token generation: %w", userID, err)
	}

	exp := time.Now().Add(AuthTokenLifespan)
	claims := &authTokenClaims{
		UserID:           userID,
		Permissions:      user.Permissions,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)},
	}

	token, err := generateToken(claims, auth.authTokenSecret)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to generate auth token: %w", err)
	}

	return token, exp, nil
}

// generateRefreshToken accepts a userID and generates a long-life token
// which can be used to generate more auth tokens by the client.
func (auth *jwtAuthProvider) generateRefreshToken(userID uuid.UUID) (string, time.Time, error) {
	_, err := auth.store.GetUserWithID(userID)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to fetch user %s during refresh token generation: %w", userID, err)
	}

	exp := time.Now().Add(RefreshTokenLifespan)
	claims := &refreshTokenClaims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)},
	}

	token, err := generateToken(claims, auth.refreshTokenSecret)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return token, exp, nil
}

// scheduleUserTokenCleanup will remove the specified token from the users token map
// at the time specified. This allows for us to store any newly generated
// user tokens inside the map without worrying about the size of the map
// growing with no limit
func (auth *jwtAuthProvider) scheduleUserTokenCleanup(userID uuid.UUID, token string, expiry time.Time) {
	until := time.Until(expiry.Add(time.Second * 5))
	log.Debugf("Scheduling cleanup of a token for user %s in %s\n", userID, until)

	time.AfterFunc(until, func() {
		log.Debugf("Cleaning up token %s for user %s as it has expired (~5 seconds ago)\n", token, userID)

		// Clear from blacklist as it won't be accepted now due to expiring anyway
		auth.blacklistedTokens.Delete(token)

		// Clear from our user tokens mapping as the token will not need to be revoked now that it has expired
		userTokens, ok := auth.userTokens.Load(userID)
		if ok && len(userTokens) > 0 {
			newUserTokens := slices.DeleteFunc(userTokens, func(tk string) bool { return tk == token })
			auth.userTokens.Store(userID, newUserTokens)
		}
	})
}

func (auth *jwtAuthProvider) getUserIdFromClaims(claims jwt.MapClaims) (*uuid.UUID, error) {
	if userID, ok := claims["user_id"]; ok {
		if id, err := uuid.Parse(userID.(string)); err != nil {
			return nil, fmt.Errorf("failed to extract user ID from JWT claims: %w", err)
		} else {
			return &id, nil
		}
	} else {
		return nil, errors.New("failed to extract user ID from JWT claims: missing")
	}
}

func (auth *jwtAuthProvider) revokeToken(token string) {
	log.Debugf("Revoking token %s\n", token)
	auth.blacklistedTokens.Store(token, struct{}{})
}

func createTokenCookie(name string, path string, token string, expiration time.Time) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = path
	cookie.HttpOnly = true

	return cookie
}

func generateToken(claims jwt.Claims, secret []byte) (string, error) {
	// Create the JWT claims, which includes the username and expiry time
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
