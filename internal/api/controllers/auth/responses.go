package auth

import (
	"encoding/json"
	"net/http"

	"github.com/hbomb79/Thea/internal/api/gen"
)

// The responses generated from our OpenAPI spec
// don't handle the duplicated nature of Set-Cookie headers,
// which we need for our auth/refresh cookies. It's simple
// enough to just implement our own response which
// correctly adds multiple of the headers

type LoginResponse struct {
	User         gen.User
	AuthToken    http.Cookie `json:"auth_token"`
	RefreshToken http.Cookie `json:"refresh_token"`
}
type SetTokenCookiesResponse struct {
	AuthToken    http.Cookie `json:"auth_token"`
	RefreshToken http.Cookie `json:"refresh_token"`
}

func (response LoginResponse) VisitLoginResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	http.SetCookie(w, &response.AuthToken)
	http.SetCookie(w, &response.RefreshToken)
	w.WriteHeader(http.StatusOK)

	return json.NewEncoder(w).Encode(response.User)
}

func (response SetTokenCookiesResponse) setTokensInResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &response.AuthToken)
	http.SetCookie(w, &response.RefreshToken)
	w.WriteHeader(http.StatusOK)

	return nil
}

func (response SetTokenCookiesResponse) VisitRefreshResponse(w http.ResponseWriter) error {
	return response.setTokensInResponse(w)
}

func (response SetTokenCookiesResponse) VisitLogoutSessionResponse(w http.ResponseWriter) error {
	return response.setTokensInResponse(w)
}

func (response SetTokenCookiesResponse) VisitLogoutAllResponse(w http.ResponseWriter) error {
	return response.setTokensInResponse(w)
}
