package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

var (
	log             = logger.Get("Auth")
	errUnauthorized = echo.NewHTTPError(http.StatusUnauthorized)
)

const (
	authTokenCookieName    = "auth-token"
	refreshTokenCookieName = "refresh-token"

	authTokenLifespan    = time.Hour * 1
	refreshTokenLifespan = time.Hour * 24
	autoRefreshThreshold = time.Minute * 15
)

type (
	customClaims struct {
		jwt.RegisteredClaims
		UserID uuid.UUID `json:"user_id"`
	}

	jwtAuthProvider struct {
		store              Store
		authTokenSecret    []byte
		refreshTokenSecret []byte
	}
)

func newJwtAuth(userStore Store, authTokenSecret string, refreshTokenSecret string) *jwtAuthProvider {
	return &jwtAuthProvider{userStore, []byte(authTokenSecret), []byte(refreshTokenSecret)}
}

// generateTokensAndSetCookies generates an auth token and a refresh token
// using the appropriate secrets and expiries, before storing both of the tokens
// in the requests cookies.
func (auth *jwtAuthProvider) generateTokensAndSetCookies(ec echo.Context, userID uuid.UUID) error {
	accessToken, exp, err := auth.generateAccessToken(userID)
	if err != nil {
		return err
	}
	setTokenCookie(ec, authTokenCookieName, accessToken, exp)

	refreshToken, exp, err := auth.generateRefreshToken(userID)
	if err != nil {
		return err
	}
	setTokenCookie(ec, refreshTokenCookieName, refreshToken, exp)

	// Don't block the request waiting for these
	go func() {
		if err := auth.store.RecordUserLogin(userID); err != nil {
			log.Warnf("Failed to record user login for %v: %v\n", userID, err)
		}
		if err := auth.store.RecordUserRefresh(userID); err != nil {
			log.Warnf("Failed to record user refresh for %v: %v\n", userID, err)
		}
	}()

	return nil
}

func (auth *jwtAuthProvider) validateToken(ec echo.Context, tokenName string, secret []byte) (*jwt.Token, error) {
	// Parses token and checks if it valid.
	cookieToken, err := ec.Cookie(tokenName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookie %s: %w", tokenName, err)
	}

	tokenClaims := &jwt.MapClaims{}
	tkn, err := jwt.ParseWithClaims(
		cookieToken.Value,
		tokenClaims,
		func(token *jwt.Token) (interface{}, error) { return secret, nil },
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse %s JWT: %w", tokenName, err)
	}

	if tkn == nil || !tkn.Valid {
		return nil, errors.New("failed to verify %s JWT: token is expired or invalid")
	}

	return tkn, nil
}

// refresh generates new auth and refresh tokens and stores them in
// the request cookies IF the request contains a valid refresh token
func (auth *jwtAuthProvider) refresh(ec echo.Context) error {
	token, err := auth.validateToken(ec, refreshTokenCookieName, auth.refreshTokenSecret)
	if err != nil {
		return fmt.Errorf("failed to refresh: %w", err)
	}

	claims := auth.getJwtClaimsFromToken(token)
	userID, err := auth.getUserIdFromClaims(*claims)
	if err != nil {
		return fmt.Errorf("failed to refresh: %w", err)
	}

	return auth.generateTokensAndSetCookies(ec, *userID)
}

// TokenRefresherMiddleware is an echo middleware which automatically
// refreshes the auth/refresh JWT tokens if the access token is nearing
// expiry.
func (auth *jwtAuthProvider) jwtTokenRefresherMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ec echo.Context) error {
		claims, err := auth.getJwtClaimsFromContext(ec)
		if err != nil {
			log.Errorf("Failed to extract claims from user JWT: %s\n", err)
			return errUnauthorized
		}

		allegedUserID, err := auth.getUserIdFromClaims(*claims)
		if err != nil {
			log.Errorf("Failed to extract user ID from user JWT claims: %s\n", err)
			return errUnauthorized
		}

		exp, err := claims.GetExpirationTime()
		if err != nil {
			log.Errorf("Failed to extract expiration time from user JWT: %s", err)
		}

		if time.Until(exp.Time) < autoRefreshThreshold {
			_, err := auth.validateToken(ec, refreshTokenCookieName, auth.refreshTokenSecret)
			if err != nil {
				log.Errorf("Unable to auto-refresh token for allegedUserID %s due to error: %s\n", allegedUserID, err)
				return errUnauthorized
			}

			log.Debugf("Auth token for user %s is nearing expiry, automatically refreshing...\n", allegedUserID)
			auth.generateTokensAndSetCookies(ec, *allegedUserID)
		}

		return next(ec)
	}
}

func (auth *jwtAuthProvider) generateAccessToken(userID uuid.UUID) (string, time.Time, error) {
	return generateToken(userID, time.Now().Add(authTokenLifespan), auth.authTokenSecret)
}

func (auth *jwtAuthProvider) generateRefreshToken(userID uuid.UUID) (string, time.Time, error) {
	return generateToken(userID, time.Now().Add(refreshTokenLifespan), auth.refreshTokenSecret)
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

func (auth *jwtAuthProvider) getJwtClaimsFromContext(ec echo.Context) (*jwt.MapClaims, error) {
	if ec.Get("user") == nil {
		return nil, errors.New("no user found in request context")
	}

	u := ec.Get("user").(*jwt.Token)
	claims := u.Claims.(jwt.MapClaims)
	return &claims, nil
}

func (auth *jwtAuthProvider) getJwtClaimsFromToken(token *jwt.Token) *jwt.MapClaims {
	return token.Claims.(*jwt.MapClaims)
}

func (auth *jwtAuthProvider) getUserIDFromContext(ec echo.Context) (*uuid.UUID, error) {
	claims, err := auth.getJwtClaimsFromContext(ec)
	if err != nil {
		return nil, err
	}

	userID, err := auth.getUserIdFromClaims(*claims)
	if err != nil {
		return nil, err
	}

	return userID, nil
}

func setTokenCookie(ec echo.Context, name, token string, expiration time.Time) {
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true

	ec.SetCookie(cookie)
}

func generateToken(userID uuid.UUID, expirationTime time.Time, secret []byte) (string, time.Time, error) {
	// Create the JWT claims, which includes the username and expiry time
	claims := &customClaims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expirationTime)},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", time.Now(), err
	}

	return tokenString, expirationTime, nil
}
