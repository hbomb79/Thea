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
type Login200JSONResponse struct {
	User         gen.User
	AuthToken    http.Cookie `json:"auth-token"`
	RefreshToken http.Cookie `json:"refresh-token"`
}
type Refresh200JSONResponse struct {
	AuthToken    http.Cookie `json:"auth-token"`
	RefreshToken http.Cookie `json:"refresh-token"`
}

func (response Login200JSONResponse) VisitLoginResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	http.SetCookie(w, &response.AuthToken)
	http.SetCookie(w, &response.RefreshToken)
	w.WriteHeader(200)

	return json.NewEncoder(w).Encode(response.User)
}

func (response Refresh200JSONResponse) VisitRefreshResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &response.AuthToken)
	http.SetCookie(w, &response.RefreshToken)
	w.WriteHeader(200)

	return nil
}
