package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// JsonMarshal is an API function that is used to avoid
// repeating boilerplate code for marhshalling structures
// for use with HTTP requests. This method will marshal the
// structure 's', and will write the output to 'w' if successful.
// If not successful, the response will have no content and will
// have it's status set to InternalServerError
func JsonMarshal(w http.ResponseWriter, s interface{}) {
	marshalled, err := json.Marshal(s)
	if err != nil {
		JsonMessage(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(marshalled)
}

// JsonMessage is an API function that is used to write
// an error state or simple message to a given http ResponseWriter (w)
// in the form {Status: status, Reason: e}. This can
// be used to display informative error messages to the
// API caller
func JsonMessage(w http.ResponseWriter, e string, status int) {
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

// trimTrailingSlashesMiddleware is a middleware function
// used to trim any trailing slashes from the incoming HTTP
// request. This allows the route (/api/test) to match
// the URL "/api/test/" and "/api/test" with the same
// mux handler.
func trimTrailingSlashesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}
