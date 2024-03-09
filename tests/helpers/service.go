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
	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/tests/gen"
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
	ws, resp, err := dialer.Dial(service.GetActivityURL(), make(map[string][]string))
	if err != nil {
		t.Fatalf("failed to connect to activity socket: %s", err)
	}
	t.Logf("Connected: %v [%v]", ws, resp)
	return ws
}

func (service *TestService) String() string {
	return fmt.Sprintf("TestService{port=%d database=%s}", service.Port, service.DatabaseName)
}

func (service *TestService) NewClient(t *testing.T) *gen.ClientWithResponses {
	client, err := gen.NewClientWithResponses(service.GetServerBasePath())
	assert.Nil(t, err)

	return client
}

func (service *TestService) NewClientWithDefaultAdminUser(t *testing.T) *gen.ClientWithResponses {
	adminUser, client := service.NewClientWithCredentials(t, DefaultAdminUsername, DefaultAdminPassword)
	assert.Subset(t, adminUser.User.Permissions, permissions.All(), "Default admin user must contain all permissions")

	return client
}

// NewClientWithUser creates a new test client with the provided user authenticated.
func (service *TestService) NewClientWithUser(t *testing.T, user TestUser) *gen.ClientWithResponses {
	_, client := service.NewClientWithCredentials(t, user.User.Username, user.Password)
	return client
}

func (service *TestService) NewClientWithCredentials(t *testing.T, username string, password string) (TestUser, *gen.ClientWithResponses) {
	resp, err := service.NewClient(t).LoginWithResponse(ctx, gen.LoginRequest{Username: username, Password: password})
	assert.Nil(t, err, "Failed to perform login request")
	assert.NotNil(t, resp.JSON200)

	cookies := resp.HTTPResponse.Cookies()
	assert.Len(t, cookies, LoginCookiesCount)

	authClient, err := gen.NewClientWithResponses(service.GetServerBasePath(), gen.WithRequestEditorFn(WithCookies(cookies)))
	assert.Nil(t, err)
	return TestUser{User: *resp.JSON200, Password: password, Cookies: cookies}, authClient
}

// NewClientWithRandomUser creates a new user using a default API client,
// and returns back a new client which has request editors to automatically
// inject the new users auth tokens in to requests made with the client.
// TODO: add cleanup task to testing context to delete user (t.Cleanup()).
func (service *TestService) NewClientWithRandomUser(t *testing.T) (TestUser, *gen.ClientWithResponses) {
	usernameAndPassword := fmt.Sprintf("TestUser%s", random.String(16))
	createResponse, err := service.NewClientWithDefaultAdminUser(t).CreateUserWithResponse(ctx, gen.CreateUserRequest{
		Permissions: permissions.All(), // TODO allow specifying permissions
		Password:    usernameAndPassword,
		Username:    usernameAndPassword,
	})

	assert.Nil(t, err)
	createdUser := createResponse.JSON200
	assert.NotNil(t, createdUser)

	// Login as newly created user to get the cookies
	return service.NewClientWithCredentials(t, usernameAndPassword, usernameAndPassword)
}

// waitForHealthy will ping the service (every pollFrequency) until the timeout is reached.
// If no successful request has been made when the timeout is reached, then the most
// recent error is returned to the caller, indicating that the service failed to become
// healthy (i.e. the service is not accepting HTTP connections).
func (service *TestService) waitForHealthy(t *testing.T, pollFrequency time.Duration, timeout time.Duration) error {
	client := service.NewClient(t)
	attempts := timeout.Milliseconds() / pollFrequency.Milliseconds()
	for attempt := range attempts {
		_, err := client.GetCurrentUser(ctx)
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
