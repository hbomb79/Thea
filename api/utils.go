package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func JsonMarshal(w http.ResponseWriter, s interface{}) {
	marshalled, err := json.Marshal(s)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(marshalled)
}

func JsonError(w http.ResponseWriter, e string, status int) {
	marshalled, err := json.Marshal(struct {
		Status int    `json:"status"`
		Reason string `json:"reason"`
	}{Status: status, Reason: e})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	w.Write(marshalled)
}

func trimTrailingSlashesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}
