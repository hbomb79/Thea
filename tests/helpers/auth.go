package helpers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

var (
	ctx = context.Background()

	ServerBasePath       = "http://localhost:8080/api/thea/v1/"
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin"
)

const LoginCookiesCount = 2

// Defines common functions which assist tests with
// creating test users and authenticated test clients

type TestUser struct {
	User     gen.User
	Password string
}

func NewClient(t *testing.T) *gen.ClientWithResponses {
	client, err := gen.NewClientWithResponses(ServerBasePath)
	assert.Nil(t, err)

	return client
}

func NewClientWithDefaultAdminUser(t *testing.T) *gen.ClientWithResponses {
	adminUser, client := NewClientWithCredentials(t, DefaultAdminUsername, DefaultAdminPassword)
	assert.Subset(t, adminUser.User.Permissions, permissions.All(), "Default admin user must contain all permissions")

	return client
}

// NewClientWithUser creates a new test client with the provided user authenticated.
func NewClientWithUser(t *testing.T, user TestUser) *gen.ClientWithResponses {
	_, client := NewClientWithCredentials(t, user.User.Username, user.Password)
	return client
}

func NewClientWithCredentials(t *testing.T, username string, password string) (TestUser, *gen.ClientWithResponses) {
	resp, err := NewClient(t).LoginWithResponse(ctx, gen.LoginRequest{Username: username, Password: password})
	assert.Nil(t, err, "Failed to perform login request")
	assert.NotNil(t, resp.JSON200)

	cookies := resp.HTTPResponse.Cookies()
	assert.Len(t, cookies, LoginCookiesCount)

	authClient, err := gen.NewClientWithResponses(ServerBasePath, gen.WithRequestEditorFn(WithCookies(cookies)))
	assert.Nil(t, err)
	return TestUser{User: *resp.JSON200, Password: password}, authClient
}

// NewClientWithRandomUser creates a new user using a default API client,
// and returns back a new client which has request editors to automatically
// inject the new users auth tokens in to requests made with the client.
// TODO: add cleanup task to testing context to delete user (t.Cleanup()).
func NewClientWithRandomUser(t *testing.T) (TestUser, *gen.ClientWithResponses) {
	usernameAndPassword := fmt.Sprintf("TestUser%s", random.String(16))
	createResponse, err := NewClientWithDefaultAdminUser(t).CreateUserWithResponse(ctx, gen.CreateUserRequest{
		Permissions: permissions.All(), // TODO allow specifying permissions
		Password:    usernameAndPassword,
		Username:    usernameAndPassword,
	})

	assert.Nil(t, err)
	createdUser := createResponse.JSON200
	assert.NotNil(t, createdUser)

	// Login as newly created user to get the cookies
	return NewClientWithCredentials(t, usernameAndPassword, usernameAndPassword)
}

func WithCookies(cookies []*http.Cookie) gen.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		for _, c := range cookies {
			req.AddCookie(c)
		}
		return nil
	}
}
