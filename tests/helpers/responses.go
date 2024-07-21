package helpers

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/hbomb79/Thea/internal/api/gen"
	"gotest.tools/v3/assert"
)

func AssertErrorResponse[T any](t *testing.T, response T, expectedStatusCode int, expectedMessage string, expectedErrorCode string) {
	valueOf := reflect.ValueOf(response)
	responseValue := valueOf.FieldByName("HTTPResponse")
	bodyValue := valueOf.FieldByName("Body")
	if !responseValue.IsValid() {
		t.Error("Field 'HTTPResponse' does not exist in the error response")
	}
	if !bodyValue.IsValid() {
		t.Error("Field 'Body' does not exist in the error response")
	}

	if httpResponse, ok := responseValue.Interface().(*http.Response); ok {
		assert.Equal(t, httpResponse.StatusCode, expectedStatusCode, "HTTPResponse status code did not match expected")
	}

	if bodyBytes, ok := bodyValue.Interface().([]byte); ok {
		apiErr := ExtractErrorResponse(t, bodyBytes)
		if expectedMessage == "" {
			assert.Equal(t, apiErr.Message, http.StatusText(expectedStatusCode))
		} else {
			assert.Equal(t, apiErr.Message, expectedMessage)
		}
		if expectedErrorCode != "" {
			assert.Equal(t, apiErr.Code, expectedErrorCode)
		}
		assert.Equal(t, apiErr.InternalMessage, "") // Internal message should never leak
		assert.Equal(t, apiErr.Status, 0)           // Status should not be included
	}
}

func ExtractErrorResponse(t *testing.T, body []byte) gen.APIError {
	var apiError gen.APIError
	if err := json.Unmarshal(body, &apiError); err != nil {
		t.Errorf("Could not extract APIError from HTTP response body: %s", err)
	}

	return apiError
}
