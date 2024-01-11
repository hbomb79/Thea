package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

var (
	errUnauthorized = echo.NewHTTPError(http.StatusUnauthorized)
)

const (
	authTokenCookieName    = "auth-token"
	refreshTokenCookieName = "refresh-token"

	authTokenLifespan    = time.Minute * 30
	refreshTokenLifespan = time.Hour * 24
)

type TypedSyncMap[K comparable, V any] struct {
	m sync.Map
}

func (m *TypedSyncMap[K, V]) Delete(key K) { m.m.Delete(key) }

func (m *TypedSyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.m.Load(key)
	if !ok {
		return value, ok
	}
	return v.(V), ok
}

func (m *TypedSyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		return value, loaded
	}
	return v.(V), loaded
}

func (m *TypedSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	a, loaded := m.m.LoadOrStore(key, value)
	return a.(V), loaded
}

func (m *TypedSyncMap[K, V]) Store(key K, value V) { m.m.Store(key, value) }

type (
	authTokenClaims struct {
		jwt.RegisteredClaims
		Permissions []string  `json:"permissions"`
		UserID      uuid.UUID `json:"user_id"`
	}

	refreshTokenClaims struct {
		jwt.RegisteredClaims
		UserID uuid.UUID `json:"user_id"`
	}

	jwtAuthProvider struct {
		store              Store
		authTokenSecret    []byte
		refreshTokenSecret []byte
		refreshRoutePath   string

		// This map (acting as a set) is used to keep track of
		// any token which we have explicitly revoked (for example,
		// when a user logs out, the auth and refresh token are revoked).
		blacklistedTokens *TypedSyncMap[string, struct{}]

		// This map is used to keep track of which tokens are currently
		// 'active' for each user. This map is automatically monitored
		// by the auth provider to clear out tokens shortly after they expire.
		// When we wish to revoke all tokens associated with a specific user, we
		// can use this map to fetch the tokens.
		userTokens *TypedSyncMap[uuid.UUID, []string]
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
		new(TypedSyncMap[string, struct{}]),
		new(TypedSyncMap[uuid.UUID, []string])}
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

// generateTokensAndSetCookies generates an auth token and a refresh token
// using the appropriate secrets and expiries, before storing both of the tokens
// in the requests cookies.
func (auth *jwtAuthProvider) GenerateTokensAndSetCookies(ec echo.Context, userID uuid.UUID) error {
	authToken, authTokenExp, err := auth.generateAccessToken(userID)
	if err != nil {
		return err
	}

	refreshToken, refreshTokenExp, err := auth.generateRefreshToken(userID)
	if err != nil {
		return err
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

	// Update client cookies
	setTokenCookie(ec, authTokenCookieName, "/", authToken, authTokenExp)
	setTokenCookie(ec, refreshTokenCookieName, auth.refreshRoutePath, refreshToken, refreshTokenExp)

	// Update our tracked list of tokens for this user, and schedule cleanup
	// of this token
	actual, loaded := auth.userTokens.LoadOrStore(userID, []string{authToken, refreshToken})
	if loaded {
		auth.userTokens.Store(userID, append(actual, authToken, refreshToken))
	}

	auth.scheduleUserTokenCleanup(userID, authToken, authTokenExp)
	auth.scheduleUserTokenCleanup(userID, refreshToken, refreshTokenExp)

	return nil
}

// RevokeTokensInContext revokes the auth and refresh token in this
// request context, assuming they are provided. A missing token/cookie
// is ignored.
func (auth *jwtAuthProvider) RevokeTokensInContext(ec echo.Context) {
	if cookie, err := ec.Cookie(authTokenCookieName); err == nil && cookie != nil {
		auth.revokeToken(cookie.Value)
	}
	if cookie, err := ec.Cookie(refreshTokenCookieName); err == nil && cookie != nil {
		auth.revokeToken(cookie.Value)
	}
}

func (auth *jwtAuthProvider) revokeToken(token string) {
	log.Debugf("Revoking token %s\n", token)
	auth.blacklistedTokens.Store(token, struct{}{})
}

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
// the request cookies IF the request contains a valid refresh token
func (auth *jwtAuthProvider) RefreshTokens(ec echo.Context) error {
	token, err := auth.validateTokenFromCookies(ec, refreshTokenCookieName, auth.refreshTokenSecret)
	if err != nil {
		return fmt.Errorf("failed to refresh: %w", err)
	}

	claims := auth.GetJwtClaimsFromToken(token)
	userID, err := auth.GetUserIdFromClaims(*claims)
	if err != nil {
		return fmt.Errorf("failed to refresh: %w", err)
	}

	return auth.GenerateTokensAndSetCookies(ec, *userID)
}

func (auth *jwtAuthProvider) GetUserIdFromClaims(claims jwt.MapClaims) (*uuid.UUID, error) {
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

func (auth *jwtAuthProvider) GetPermissionsFromClaims(claims jwt.MapClaims) ([]string, error) {
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

func (auth *jwtAuthProvider) GetJwtClaimsFromContext(ec echo.Context) (*jwt.MapClaims, error) {
	if ec.Get("user") == nil {
		return nil, errors.New("no user found in request context")
	}

	u := ec.Get("user").(*jwt.Token)
	claims := u.Claims.(*jwt.MapClaims)
	return claims, nil
}

func (auth *jwtAuthProvider) GetJwtClaimsFromToken(token *jwt.Token) *jwt.MapClaims {
	return token.Claims.(*jwt.MapClaims)
}

func (auth *jwtAuthProvider) GetUserIDFromContext(ec echo.Context) (*uuid.UUID, error) {
	claims, err := auth.GetJwtClaimsFromContext(ec)
	if err != nil {
		return nil, err
	}

	userID, err := auth.GetUserIdFromClaims(*claims)
	if err != nil {
		return nil, err
	}

	return userID, nil
}

func (auth *jwtAuthProvider) GetAuthenticatedMiddleware() echo.MiddlewareFunc {
	return echojwt.WithConfig(echojwt.Config{
		TokenLookup:  fmt.Sprintf("cookie:%s", authTokenCookieName),
		ErrorHandler: func(ec echo.Context, err error) error { return echo.NewHTTPError(http.StatusUnauthorized) },
		ParseTokenFunc: func(ec echo.Context, token string) (any, error) {
			return auth.validateToken(token, auth.authTokenSecret)
		},
	})
}

func (auth *jwtAuthProvider) GetPermissionAuthorizerMiddleware(requiredPermissions []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ec echo.Context) error {
			claims, err := auth.GetJwtClaimsFromContext(ec)
			if err != nil {
				log.Errorf("Failed to extract claims from user JWT: %s\n", err)
				return errUnauthorized
			}

			userID, err := auth.GetUserIdFromClaims(*claims)
			if err != nil {
				log.Errorf("Failed to extract user ID from user JWT claims: %s\n", err)
				return errUnauthorized
			}

			permissions, err := auth.GetPermissionsFromClaims(*claims)
			if err != nil {
				log.Errorf("Failed to extract user permissions from user JWT claims: %s\n", err)
				return errUnauthorized
			}

			for _, perm := range requiredPermissions {
				if !slices.Contains(permissions, perm) {
					log.Warnf("User %s failed permissions check while accessing %s: missing permission '%s'\n", userID, ec.Path(), perm)
					return echo.NewHTTPError(http.StatusForbidden)
				}
			}

			return next(ec)
		}
	}
}

// validateToken ensures that the provided token is:
//   - signed using the same secret/algorithm as we expect
//   - contains a valid userID
//   - not expired
//   - not blacklisted
func (auth *jwtAuthProvider) validateToken(token string, secret []byte) (*jwt.Token, error) {
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
	if _, err := auth.GetUserIdFromClaims(*tokenClaims); err != nil {
		return nil, fmt.Errorf("failed to extract userID from JWT: %w", err)
	}

	// Check we haven't revoked this token
	if _, ok := auth.blacklistedTokens.Load(token); ok {
		return nil, errors.New("failed to verify JWT: token has been revoked")
	}

	return tkn, nil
}

// validateTokenFromCookies is a convinience method to lookup
// the given token in the request's cookies before proceeding
// to validate the token providing it was found.
func (auth *jwtAuthProvider) validateTokenFromCookies(ec echo.Context, tokenName string, secret []byte) (*jwt.Token, error) {
	cookieToken, err := ec.Cookie(tokenName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookie %s: %w", tokenName, err)
	}

	return auth.validateToken(cookieToken.Value, secret)
}

func (auth *jwtAuthProvider) generateAccessToken(userID uuid.UUID) (string, time.Time, error) {
	user, err := auth.store.GetUserWithID(userID)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to fetch user %s during auth token generation: %w", userID, err)
	}

	exp := time.Now().Add(authTokenLifespan)
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

func (auth *jwtAuthProvider) generateRefreshToken(userID uuid.UUID) (string, time.Time, error) {
	_, err := auth.store.GetUserWithID(userID)
	if err != nil {
		return "", time.Now(), fmt.Errorf("failed to fetch user %s during refresh token generation: %w", userID, err)
	}

	exp := time.Now().Add(refreshTokenLifespan)
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

func setTokenCookie(ec echo.Context, name string, path string, token string, expiration time.Time) {
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = path
	cookie.HttpOnly = true

	ec.SetCookie(cookie)
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
