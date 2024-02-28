package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/test"
	"github.com/hbomb79/Thea/test/helpers"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

const (
	loginCookiesCount = 2
)

var (
	ctx = context.Background()

	// TODO: get these from env.
	serverBasePath       = "http://localhost:8080/api/thea/v1/"
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin"
)

type TestUser struct {
	User     test.User
	Password string
}

/*
	This package performs HTTP REST API testing against
	this controller. It requires that an instance of
	Thea is running - externally to this test suite - on the
	URL provided.
*/

func newClient(t *testing.T) *test.ClientWithResponses {
	client, err := test.NewClientWithResponses(serverBasePath)
	assert.Nil(t, err)

	return client
}

func newClientWithDefaultAdminUser(t *testing.T) *test.ClientWithResponses {
	resp, err := newClient(t).LoginWithResponse(ctx, test.LoginRequest{Username: defaultAdminUsername, Password: defaultAdminPassword})
	assert.Nil(t, err, "Failed to perform login request")
	assert.Contains(t, resp.JSON200.Permissions, permissions.CreateUserPermission, "Default admin user must contain CreateUser permission")

	cookies := resp.HTTPResponse.Cookies()
	assert.Len(t, cookies, loginCookiesCount)

	authClient, err := test.NewClientWithResponses(serverBasePath, test.WithRequestEditorFn(helpers.WithCookies(cookies)))
	assert.Nil(t, err)
	return authClient
}

// newClientWithUser creates a new user using a default API client,
// and returns back a new client which has request editors to automatically
// inject the new users auth tokens in to requests made with the client.
// TODO: add cleanup task to testing context to delete user (t.Cleanup()).
func newClientWithRandomUser(t *testing.T) (TestUser, *test.ClientWithResponses) {
	usernameAndPassword := fmt.Sprintf("TestUser%s", random.String(16))
	createResponse, err := newClientWithDefaultAdminUser(t).CreateUserWithResponse(ctx, test.CreateUserRequest{
		Permissions: permissions.All(), // TODO allow specifying permissions
		Password:    usernameAndPassword,
		Username:    usernameAndPassword,
	})

	assert.Nil(t, err)
	createdUser := createResponse.JSON200
	assert.NotNil(t, createdUser)

	// Login as newly created user to get the cookies
	loginResp, err := newClient(t).LoginWithResponse(ctx, test.LoginRequest{Username: usernameAndPassword, Password: usernameAndPassword})
	assert.Nil(t, err, "Failed to perform login request")
	loggedInUser := loginResp.JSON200
	assert.NotNil(t, loggedInUser)

	cookies := loginResp.HTTPResponse.Cookies()
	assert.Len(t, cookies, loginCookiesCount)

	authedClient, err := test.NewClientWithResponses(serverBasePath, test.WithRequestEditorFn(helpers.WithCookies(cookies)))
	assert.Nil(t, err)

	return TestUser{User: *loggedInUser, Password: usernameAndPassword}, authedClient
}

func TestLogin_InvalidCredentials(t *testing.T) {
	client, err := test.NewClientWithResponses(serverBasePath)
	assert.Nil(t, err, "Failed to create client")

	resp, err := client.LoginWithResponse(ctx,
		test.LoginRequest{
			Username: "notausername",
			Password: "definitelynotapassword",
		},
	)
	assert.Nil(t, err, "Failed to perform login request")
	assert.Nil(t, resp.JSON200, "Expected Nil User payload")
	assert.Len(t, resp.HTTPResponse.Cookies(), 0, "Expected HTTPResponse cookies to be empty")
	helpers.AssertErrorResponse(t, *resp, 401, "Unauthorized", "")
}

// Ensure that a successful login returns valid tokens
// which can be used in a subsequent request to fetch the user.
func TestLogin_ValidCredentials(t *testing.T) {
	testUser, authedClient := newClientWithRandomUser(t)

	assertUser := func(user *test.User) {
		assert.NotNil(t, user, "Expected User payload to be non-nil")
		assert.Equal(t, testUser.User.Username, user.Username)

		// TODO: fix me
		// Assert createdAt < lastLoginAt < now
		// assert.NotNil(t, user.LastLogin)
		// assert.Less(t, *user.LastLogin, time.Now())
		// assert.Less(t, user.CreatedAt, *user.LastLogin)
	}

	user := testUser.User
	assertUser(&user)

	currentUserResponse, err := authedClient.GetCurrentUserWithResponse(ctx)
	assert.Nil(t, err, "Failed to get current user")

	currentUser := currentUserResponse.JSON200
	assertUser(currentUser)
	// TODO: fixme
	// assert.Equal(t, *user.LastLogin, *currentUser.LastLogin)
	// assert.Equal(t, user.CreatedAt, currentUser.CreatedAt)
}

// Ensures that the tokens acquired from a successful login
// become un-usable following a logout (where said tokens are supplied
// in the request). If a client retains these tokens in spite
// of the response clearing the cookies, they should not work for
// secured endpoints.
func TestLogout_BlacklistsTokens(t *testing.T) {
	client, err := test.NewClientWithResponses(serverBasePath)
	assert.Nil(t, err, "Failed to create client")

	loginResp, err := client.LoginWithResponse(ctx, test.LoginRequest{Username: defaultAdminUsername, Password: defaultAdminPassword})
	assert.Nil(t, err, "Failed to perform login request")

	cookies := loginResp.HTTPResponse.Cookies()
	assert.Equal(t, http.StatusOK, loginResp.StatusCode())
	assert.NotNil(t, loginResp.JSON200)
	assert.Len(t, cookies, loginCookiesCount)

	currentUserResponse, err := client.GetCurrentUserWithResponse(ctx, helpers.WithCookies(cookies))
	assert.Nil(t, err, "Failed to get current user")
	assert.Equal(t, http.StatusOK, currentUserResponse.StatusCode())
	assert.NotNil(t, currentUserResponse.JSON200)

	logoutResp, err := client.LogoutSessionWithResponse(ctx, helpers.WithCookies(cookies))
	assert.Nil(t, err, "Failed to logout session")
	assert.Equal(t, http.StatusOK, logoutResp.StatusCode())
	assert.NotNil(t, logoutResp)

	// Provide the old cookies to ensure they're now invalid. Normal browsers/cookie jars
	// will have these cookies deleted by the logout session response.
	failedCurrentUserResponse, err := client.GetCurrentUserWithResponse(ctx, helpers.WithCookies(cookies))
	assert.Nil(t, err, "Failed to get current user")
	assert.Nil(t, failedCurrentUserResponse.JSON200)
	helpers.AssertErrorResponse(t, *failedCurrentUserResponse, http.StatusForbidden, "", "")
}

// Ensures that all tokens for a specific user are blacklisted
// when 'LogoutAll' is called. Other users active on Thea should
// not be impacted by this.
func TestLogoutAll_BlacklistsAllTokens(t *testing.T) {
	// Login as this user a bunch of times. We should see that logging out
	// via 'logout session' for a single user will leave the others authorized. However
	// using 'logout all' on any single user should revoke the tokens of all other
	// users.
	t.Skip()
}
