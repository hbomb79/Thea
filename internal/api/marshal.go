package api

import (
	"encoding/json"
	"net/http"

	"github.com/liip/sheriff"
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

// sheriffApiMarshal is a method that will marshal
// the provided argument using Sheriff to remove
// items from the struct that aren't exposed to the API (i.e. removes
// struct fields that lack the `groups:"api"` tag)
func sheriffApiMarshal(target interface{}, groups ...string) (interface{}, error) {
	o := &sheriff.Options{Groups: groups}

	data, err := sheriff.Marshal(o, target)
	if err != nil {
		return nil, err
	}

	return data, nil
}
