package helpers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
)

const (
	ServerBasePathTemplate = "http://localhost:%d/api/thea/v1/"
	LoginCookiesCount      = 2
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

// TestService holds information about a
// provisioned (or reused) Thea service
// which a test can request resources from (typically
// test clients for making requests, which handle
// port mappings).
type TestService struct {
	Port         int
	DatabaseName string

	container testcontainers.Container
	cleanup   func(t *testing.T)
}

func (service *TestService) GetServerBasePath() string {
	return fmt.Sprintf(ServerBasePathTemplate, service.Port)
}

func (service *TestService) String() string {
	return fmt.Sprintf("TestContainer{port=%d database=%s}", service.Port, service.DatabaseName)
}

// Defines common functions which assist tests with
// creating test users and authenticated test clients

type TestUser struct {
	User     gen.User
	Password string
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
	return TestUser{User: *resp.JSON200, Password: password}, authClient
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
