package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/hbomb79/Thea/test"
	"github.com/hbomb79/Thea/test/helpers"
	"github.com/stretchr/testify/assert"
)

const (
	loginCookiesCount = 2
)

var ctx = context.Background() // TODO: get these from env.

/*
This package performs HTTP REST API testing against
this controller. It requires that an instance of
Thea is running - externally to this test suite - on the
URL provided.
*/
func TestLogin_InvalidCredentials(t *testing.T) {
	resp, err := helpers.NewClient(t).LoginWithResponse(ctx,
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
	testUser, authedClient := helpers.NewClientWithRandomUser(t)
	assertUserValid := func(user *test.User) {
		assert.NotNil(t, user, "Expected User payload to be non-nil")
		assert.Equal(t, testUser.User.Username, user.Username)

		// TODO: fix me
		// Assert createdAt < lastLoginAt < now
		// assert.NotNil(t, user.LastLogin)
		// assert.Less(t, *user.LastLogin, time.Now())
		// assert.Less(t, user.CreatedAt, *user.LastLogin)
	}

	user := testUser.User
	assertUserValid(&user)

	currentUserResponse, err := authedClient.GetCurrentUserWithResponse(ctx)
	assert.Nil(t, err, "Failed to get current user")

	currentUser := currentUserResponse.JSON200
	assertUserValid(currentUser)
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
	_, authedClient := helpers.NewClientWithRandomUser(t)

	currentUserResponse, err := authedClient.GetCurrentUserWithResponse(ctx)
	assert.Nil(t, err, "Failed to get current user")
	assert.Equal(t, http.StatusOK, currentUserResponse.StatusCode())
	assert.NotNil(t, currentUserResponse.JSON200)

	logoutResp, err := authedClient.LogoutSessionWithResponse(ctx)
	assert.Nil(t, err, "Failed to logout session")
	assert.Equal(t, http.StatusOK, logoutResp.StatusCode())
	assert.NotNil(t, logoutResp)

	failedCurrentUserResponse, err := authedClient.GetCurrentUserWithResponse(ctx)
	assert.Nil(t, err, "Failed to get current user")
	assert.Nil(t, failedCurrentUserResponse.JSON200)
	helpers.AssertErrorResponse(t, *failedCurrentUserResponse, http.StatusForbidden, "", "")
}

// Ensures that all tokens for a specific user are blacklisted
// when 'LogoutAll' is called. Other users active on Thea should
// not be impacted by this.
func TestLogoutAll_BlacklistsAllTokens(t *testing.T) {
	assertClientState := func(t *testing.T, client *test.ClientWithResponses, expectedUser *helpers.TestUser) {
		resp, err := client.GetCurrentUserWithResponse(ctx)
		assert.Nil(t, err)

		if expectedUser != nil {
			assert.NotNilf(t, resp.JSON200, "Was expecting user %s to be returned from current user endpoint, but got nil", expectedUser)
			assert.Equal(t, resp.JSON200.Id, expectedUser.User.Id)
		} else {
			helpers.AssertErrorResponse(t, *resp, http.StatusForbidden, "", "")
		}
	}

	// Create a new testUser and login multiple times with the testUser
	testUser, client := helpers.NewClientWithRandomUser(t)
	sameUserClients := make([]*test.ClientWithResponses, 0, 3)
	sameUserClients = append(sameUserClients, client)
	for i := 0; i < 2; i++ {
		// Login as the same user again to get a new set of tokens
		// Sleep as JWT expiry is only accurate to the second, and so two logins in
		// the same second generate the same payload (and therefore an identical token)
		cl := helpers.NewClientWithUser(t, testUser)
		sameUserClients = append(sameUserClients, cl)
	}

	// Create another user and session to ensure that this user is unaffected by
	// our clients 'logout all'
	otherTestUser, unrelatedClient := helpers.NewClientWithRandomUser(t)
	assertClientState(t, unrelatedClient, &otherTestUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, &testUser)
	}

	// Logout of all 'user' sessions
	resp, err := client.LogoutAll(ctx)
	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	// Ensure all clients authenticated against that user are now all revoked, but the other user is still valid
	assertClientState(t, unrelatedClient, &otherTestUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, nil)
	}

	// Check that logging in again does not revert the previous revoking of the other sessions
	loggedInClient := helpers.NewClientWithUser(t, testUser)
	assertClientState(t, unrelatedClient, &otherTestUser)
	assertClientState(t, loggedInClient, &testUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, nil)
	}
}
