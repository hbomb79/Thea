package helpers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	theaws "github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/go-chanassert"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

const (
	ServerBasePathTemplate = "%s://localhost:%d/api/thea/v1/"
	LoginCookiesCount      = 2
	ActivityPath           = "activity/ws"
)

var (
	ctx = context.Background()

	DefaultAdminUsername = EnvVarOrDefault("ADMIN_USERNAME", "admin")
	DefaultAdminPassword = EnvVarOrDefault("ADMIN_PASSWORD", "admin")
)

func EnvVarOrDefault(key string, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	} else {
		return def
	}
}

func WithCookies(cookies []*http.Cookie) gen.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		for _, c := range cookies {
			req.AddCookie(c)
		}
		return nil
	}
}

type TestUser struct {
	User     gen.User
	Password string
	Cookies  []*http.Cookie
}

// TestService holds information about a
// provisioned (or reused) Thea service
// which a test can request resources from (typically
// test clients for making requests, which handle
// port mappings).
type TestService struct {
	Port         int
	DatabaseName string

	cleanup func(t *testing.T)
}

func (service *TestService) GetServerBasePath() string {
	return fmt.Sprintf(ServerBasePathTemplate, "http", service.Port)
}

func (service *TestService) GetActivityURL() string {
	return fmt.Sprintf("%s%s", fmt.Sprintf(ServerBasePathTemplate, "ws", service.Port), ActivityPath)
}

func (service *TestService) ConnectToActivitySocket(t *testing.T) *websocket.Conn {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to connect to activity socket: failed to create cookie jar: %s", err)
	}

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Jar: jar}
	ws, _, err := dialer.Dial(service.GetActivityURL(), make(map[string][]string))
	if err != nil {
		t.Fatalf("failed to connect to activity socket: %s", err)
	}
	return ws
}

// ActivityExpecter returns a chanassert expecter which will be setup
// with a single layer which expects the [theaws.Welcome] message.
//
// Tests can add additional layers to the expecter before calling .Listen on it
// to start the expecter.
func (service *TestService) ActivityExpecter(t *testing.T) chanassert.Expecter[theaws.SocketMessage] {
	activityChan := service.ActivityChannel(t)

	return chanassert.
		NewChannelExpecter(activityChan).
		Debug().
		ExpectTimeout(
			time.Millisecond*500,
			chanassert.OneOf(MatchSocketMessage("CONNECTION_ESTABLISHED", theaws.Welcome)),
		)
}

func (service *TestService) ActivityChannel(t *testing.T) chan theaws.SocketMessage {
	ws := service.ConnectToActivitySocket(t)
	assert.NotNil(t, ws, "expected websocket connection to be non-nil")

	output := make(chan theaws.SocketMessage, 5)
	go func() {
		for {
			var message theaws.SocketMessage
			err := ws.ReadJSON(&message)
			if err == nil {
				t.Logf("WS: received message: %+v\n", message)
				output <- message
				continue
			}

			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
				t.Logf("WS: warning: websocket connection abnormally closed (error %s). This is expected if the Thea instance was abruptly closed.", err)
			} else {
				t.Errorf("WS: unexpected error: %+v (%T)", err, err)
			}

			return
		}
	}()

	return output
}

func (service *TestService) String() string {
	return fmt.Sprintf("TestService{port=%d database=%s}", service.Port, service.DatabaseName)
}

func makeClient(client *gen.Client) error {
	client.Client = &http.Client{
		Timeout: 10 * time.Second,
	}

	return nil
}

func (service *TestService) NewClient(t *testing.T) *APIClient {
	client, err := gen.NewClientWithResponses(service.GetServerBasePath(), makeClient)
	assert.Nil(t, err)

	return &APIClient{client}
}

func (service *TestService) NewClientWithDefaultAdminUser(t *testing.T) *APIClient {
	adminUser, client := service.NewClientWithCredentials(t, DefaultAdminUsername, DefaultAdminPassword)
	assert.Subset(t, adminUser.User.Permissions, permissions.All(), "Default admin user must contain all permissions")

	return &APIClient{client}
}

// NewClientWithUser creates a new test client with the provided user authenticated.
func (service *TestService) NewClientWithUser(t *testing.T, user TestUser) *APIClient {
	_, client := service.NewClientWithCredentials(t, user.User.Username, user.Password)
	return &APIClient{client}
}

func (service *TestService) NewClientWithCredentials(t *testing.T, username string, password string) (TestUser, *APIClient) {
	resp, err := service.NewClient(t).LoginWithResponse(ctx, gen.LoginRequest{Username: username, Password: password})
	assert.NoError(t, err, "Failed to perform login request")
	assert.NotNil(t, resp.JSON200)

	cookies := resp.HTTPResponse.Cookies()
	assert.Len(t, cookies, LoginCookiesCount)

	authClient, err := gen.NewClientWithResponses(service.GetServerBasePath(), makeClient, gen.WithRequestEditorFn(WithCookies(cookies)))
	assert.Nil(t, err)
	return TestUser{User: *resp.JSON200, Password: password, Cookies: cookies}, &APIClient{authClient}
}

// NewClientWithRandomUserPermissions creates a new user with a random username
// and password (both are the same), with the permissions specified. The user is returned
// alongside an API client which will automatically inject the authentication tokens
// for outgoing requests.
// TODO: add cleanup task to testing context to delete user (t.Cleanup()).
func (service *TestService) NewClientWithRandomUserPermissions(t *testing.T, permissions []string) (TestUser, *APIClient) {
	usernameAndPassword := fmt.Sprintf("TestUser%s", random.String(16))
	createResponse, err := service.NewClientWithDefaultAdminUser(t).CreateUserWithResponse(ctx, gen.CreateUserRequest{
		Permissions: permissions,
		Password:    usernameAndPassword,
		Username:    usernameAndPassword,
	})

	assert.Nil(t, err)
	createdUser := createResponse.JSON200
	assert.NotNil(t, createdUser)

	// Login as newly created user to get the cookies
	return service.NewClientWithCredentials(t, usernameAndPassword, usernameAndPassword)
}

// NewClientWithRandomUser creates a new user which has a random username/password,
// and has *all* permissions. The user is returned alongside an API client which
// will automatically inject the authentication tokens for outgoing requests.
func (service *TestService) NewClientWithRandomUser(t *testing.T) (TestUser, *APIClient) {
	return service.NewClientWithRandomUserPermissions(t, permissions.All())
}

// waitForHealthy will ping the service (every pollFrequency) until the timeout is reached.
//
// If no successful request has been made when the timeout is reached, then the most
// recent error is returned to the caller, indicating that the service failed to become
// healthy (i.e. the service is not accepting HTTP connections).
func (service *TestService) waitForHealthy(t *testing.T, pollFrequency time.Duration, timeout time.Duration) error {
	t.Logf("Waiting for Thea process to become healthy (poll every %s, timeout %s)...", pollFrequency, timeout)

	client := service.NewClient(t)
	attempts := timeout.Milliseconds() / pollFrequency.Milliseconds()
	for attempt := range attempts {
		_, err := client.GetCurrentUserWithResponse(ctx)
		if err != nil {
			if attempt == attempts-1 {
				return err
			}

			time.Sleep(pollFrequency)
			continue
		}

		break
	}

	return nil
}
