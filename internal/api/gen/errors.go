package gen

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/labstack/echo/v4"
)

type APIError struct {
	// Human readable error display message
	Message string `json:"message"`

	// A machine readable and stable identifier for the error case being represented
	Code string `json:"code"`

	// Used to alter the HTTP response status in accordance with the error
	Status int `json:"-"`

	// Additional message for internal logging only. Will not be included in the message
	// sent to the user.
	InternalMessage string `json:"-"`
}

// Error satisifies the Go error interface and simply exposes the
// message contained by this APIError.
func (err APIError) Error() string {
	return fmt.Sprintf("api error: %s", err.Message)
}

var ErrAPIUnauthorized APIError = APIError{Status: 401}

// GetHTTPErrorHandler returns an echo HTTP error handler
// which understands how to interpret APIError. If an error is
// provided which is not recognized, it will be passed off to the
// fallback HTTP handler provided.
func GetHTTPErrorHandler(fallbackHandler echo.HTTPErrorHandler) echo.HTTPErrorHandler {
	logger := logger.Get("API")
	return func(err error, ctx echo.Context) {
		var apiErr APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.Status == 0 {
				apiErr.Status = 500
			}
			if len(apiErr.Message) == 0 {
				apiErr.Message = http.StatusText(apiErr.Status)
			}
			if len(apiErr.Code) == 0 {
				apiErr.Code = http.StatusText(apiErr.Status)
			}
			if len(apiErr.InternalMessage) > 0 {
				logger.Errorf("Request failure, internal error: %s\n", apiErr.InternalMessage)
			}

			if err := ctx.JSON(apiErr.Status, apiErr); err == nil {
				return
			}
		}

		// This is not an APIError, just let Echo handle it as it normally would
		// TODO: Consider just 500'ing here, and enforcing that our routes MUST
		// use the APIError if they want to expose error information.
		logger.Warnf(
			"%s request to %s caused error response, however the response does not satisfy the APIError interface. Falling back to default HTTP error handling\n",
			ctx.Request().Method, ctx.Request().RequestURI,
		)
		fallbackHandler(err, ctx)
	}
}
