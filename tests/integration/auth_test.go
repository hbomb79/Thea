package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/http/websocket"
	"github.com/hbomb79/Thea/internal/user/permissions"
	"github.com/hbomb79/Thea/tests/gen"
	"github.com/hbomb79/Thea/tests/helpers"
	"github.com/hbomb79/go-chanassert"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

// This package performs HTTP REST API testing against
// this controller. It requires that an instance of
// Thea is running - externally to this test suite - on the
// URL provided.

func TestLogin_InvalidCredentials(t *testing.T) {
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	resp, err := srv.NewClient(t).LoginWithResponse(ctx,
		gen.LoginRequest{
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
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	testUser, authedClient := srv.NewClientWithRandomUser(t)
	assertUserValid := func(user *gen.User) {
		assert.NotNil(t, user, "Expected User payload to be non-nil")
		assert.Equal(t, testUser.User.Username, user.Username)

		// TODO: fix me
		// Assert createdAt < lastLoginAt < now
		// assert.NotNil(t, user.LastLogin)
		// assert.Less(t, *user.LastLogin, time.Now())
		// assert.Less(t, user.CreatedAt, *user.LastLogin)
	}

	assertUserValid(&testUser.User)

	currentUserResponse, err := authedClient.GetCurrentUserWithResponse(ctx)
	assert.Nil(t, err, "Failed to get current user")

	assertUserValid(currentUserResponse.JSON200)
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
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	_, authedClient := srv.NewClientWithRandomUser(t)

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
	srv := helpers.RequireDefaultThea(t)
	t.Parallel()

	assertClientState := func(t *testing.T, client *helpers.APIClient, expectedUser *helpers.TestUser) {
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
	testUser, client := srv.NewClientWithRandomUser(t)
	sameUserClients := make([]*helpers.APIClient, 0, 3)
	sameUserClients = append(sameUserClients, client)
	for i := 0; i < 2; i++ {
		// Login as the same user again to get a new set of tokens
		// Sleep as JWT expiry is only accurate to the second, and so two logins in
		// the same second generate the same payload (and therefore an identical token)
		_, cl := srv.NewClientWithUser(t, testUser)
		sameUserClients = append(sameUserClients, cl)
	}

	// Create another user and session to ensure that this user is unaffected by
	// our clients 'logout all'
	otherTestUser, unrelatedClient := srv.NewClientWithRandomUser(t)
	assertClientState(t, unrelatedClient, &otherTestUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, &testUser)
	}

	// Logout of all 'user' sessions
	resp, err := client.LogoutAllWithResponse(ctx)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	// Ensure all clients authenticated against that user are now all revoked, but the other user is still valid
	assertClientState(t, unrelatedClient, &otherTestUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, nil)
	}

	// Check that logging in again does not revert the previous revoking of the other sessions
	_, loggedInClient := srv.NewClientWithUser(t, testUser)
	assertClientState(t, unrelatedClient, &otherTestUser)
	assertClientState(t, loggedInClient, &testUser)
	for _, cl := range sameUserClients {
		assertClientState(t, cl, nil)
	}
}

// makeActivityListener builds a chanassert Expecter using the bools
// provided to conditionally add combiners pertaining to events for those
// resource types.
//
// For example, a listener with all bools as false, will expect only the 'welcome' message.
// Conversely, a listener with all bools as true will expect to see at least 1 message for media,
// ingests, and transcode updates (in addition to the welcome message).
func makeActivityListener(
	t *testing.T,
	service *helpers.TestService,
	user helpers.TestUser,
	media, ingests, transcodes bool,
) chanassert.Expecter[websocket.SocketMessage] {
	expecter := service.ActivityExpecter(t, user)
	combiners := make([]chanassert.Combiner[websocket.SocketMessage], 0)
	if media {
		combiners = append(combiners, chanassert.AtLeastNOf(1, helpers.MatchMessageTitle("MEDIA_UPDATE")))
	}

	if ingests {
		combiners = append(combiners, chanassert.AtLeastNOf(1, helpers.MatchMessageTitle("INGEST_UPDATE")))
	}

	if transcodes {
		combiners = append(combiners, chanassert.AtLeastNOfEach(1,
			helpers.MatchMessageTitle("TRANSCODE_TASK_UPDATE"),
			helpers.MatchMessageTitle("TRANSCODE_TASK_PROGRESS_UPDATE"),
		))
	}

	if len(combiners) == 0 {
		return expecter
	}

	return expecter.Expect(combiners...)
}

// TestPermissions_ActivityStream connects to Thea using multiple
// different users/permission sets, and ensures that activity streams
// only send information regarding a particular resource to clients which
// have sufficient permissions to access those resource(s).
func TestPermissions_ActivityStream(t *testing.T) {
	ingestDir, _ := helpers.TempDirWithFiles(t, map[string]string{
		"./testdata/validmedia/short-sample.mkv": "Shaun.of.the.Dead.2004.mkv",
	})

	req := helpers.NewTheaServiceRequest().
		WithIngestDirectory(ingestDir).
		RequiresTMDB().
		// Ensure the threshold here is long enough for all test permutations
		// to connect and start their channel expecter. This includes the time
		// we spend bootstrapping the target/workflow.
		WithEnvironmentVariable("INGEST_MODTIME_THRESHOLD_SECONDS", "5").
		WithEnvironmentVariable("FORMAT_DEFAULT_OUTPUT_DIR", t.TempDir())

	srv := helpers.RequireThea(t, req)

	_, client := srv.NewClientWithDefaultAdminUser(t)
	targets := client.CreateRandomTargets(t, 1)
	targetIDs := targets.IDs()

	// Create an enabled workflow with no criteria (which matches everything).
	_ = client.CreateWorkflow(t, nil, true, random.String(16, random.Alphanumeric), &targetIDs)

	// Permute the possible configurations, and create a user and expecter
	// for each.
	for _, media := range []bool{true, false} {
		for _, ingest := range []bool{true, false} {
			for _, transcode := range []bool{true, false} {
				t.Run(fmt.Sprintf("permutation_%v_%v_%v", media, ingest, transcode), func(t *testing.T) {
					// Create a user based on these permission requirements.
					perms := make([]string, 0)
					if ingest {
						perms = append(perms, permissions.AccessIngestsPermission)
					}
					if transcode {
						perms = append(perms, permissions.AccessTranscodePermission)
					}
					if media {
						perms = append(perms, permissions.AccessMediaPermission)
					}

					u, _ := srv.NewClientWithRandomUserPermissions(t, perms)
					exp := makeActivityListener(t, srv, u, media, ingest, transcode)

					// Listen and wait immediately as this test is in it's own goroutine.
					exp.Listen()

					// CAUTION: the positioning of this parallel call is intentional. When
					// called, the test is likely to be parked. If parked BEFORE connecting
					// to the websocket, then by the time it's resumed, it may have missed events
					// which are required for the test to pass. We parallelise it here
					// because we're just playing the waiting game now and need to let the
					// other permutations start too.
					t.Parallel()
					exp.AssertSatisfied(t, 10*time.Second)
				})
			}
		}
	}
}

// Test_PermissionsFromSpec uses the embedded Swagger spec in the
// generated API client to scrape the permission security requirements
// for each endpoint. Then, we use a raw HTTP client to test
// different permutations of user/permissions combinations to ensure
// that only a user with all the permissions specified in the security component
// of each path is actually able to access the endpoint
//
// We don't mock the request body, we simply assert whether we get a 403 or not, as that
// in sufficient.
//
// NOTE: that this test only touches endpoints documented in the OpenAPI spec (except
// for endpoints tagged with 'Auth', as they are special cases). Endpoints not documented
// in that spec (e.g. /activity/ws) will not be tested.
//
//nolint:gocognit
func TestPermissions_SwaggerSpec(t *testing.T) {
	sw, err := gen.GetSwagger()
	assert.NoError(t, err)

	pathsToPerms := make(map[string]map[string][]string)

	for k, p := range sw.Paths {
		ops := p.Operations()
		pathsToPerms[k] = make(map[string][]string)
		for method, op := range ops {
			// Ignore paths which impact authentication, as we have test
			// coverage for these above (and they'll mess up the tokens
			// for our clients).
			if slices.Contains(op.Tags, "Auth") {
				continue
			}

			if op.Security == nil || len(*op.Security) == 0 {
				pathsToPerms[k][method] = []string{}
			} else {
				pathsToPerms[k][method] = (*op.Security)[0]["permissionAuth"]
			}
		}
	}

	req := helpers.NewTheaServiceRequest()
	srv := helpers.RequireThea(t, req)
	allPermsUser, _ := srv.NewClientWithDefaultAdminUser(t)
	noPermsUser, _ := srv.NewClientWithRandomUserPermissions(t, []string{})

	// Create HTTP rawClient
	rawClient := http.Client{}
	baseURL, err := url.Parse(srv.GetServerBasePath())
	assert.NoError(t, err)

	assertAccessForUser := func(t *testing.T, user helpers.TestUser, method string, path string, canAccess bool) {
		// Path might have request parameters inside
		replacedPath := strings.ReplaceAll(path, "{id}", uuid.New().String())
		if replacedPath[0] == '/' {
			replacedPath = "." + replacedPath
		}

		if strings.Contains(replacedPath, "{") {
			t.Fatalf("replaced path (%s) still contains a placeholder", replacedPath)
		}

		url, err := baseURL.Parse(replacedPath)
		assert.NoError(t, err)

		// Build request
		req, err := http.NewRequest(method, url.String(), nil)
		assert.NoError(t, err)
		for _, cookie := range user.Cookies {
			req.AddCookie(cookie)
		}

		// Send request
		resp, err := rawClient.Do(req)
		assert.NoError(t, err)
		if canAccess {
			assert.NotEqualf(t, http.StatusForbidden, resp.StatusCode, "expected to be able to access path (%v)", req.URL)
			assert.NotEqualf(t, http.StatusUnauthorized, resp.StatusCode, "expected to be able to access path (%v)", req.URL)
		} else {
			assert.Equalf(t, http.StatusForbidden, resp.StatusCode, "expected to be unable to access path (%v)", req.URL)
		}
	}

	for path, methodAndPerms := range pathsToPerms {
		for method, requiredPermissions := range methodAndPerms {
			t.Run(fmt.Sprintf("%s%s", method, path), func(t *testing.T) {
				t.Parallel()
				t.Logf("Testing [%s] %s (%d perms)", method, path, len(requiredPermissions))

				switch len(requiredPermissions) {
				case 0:
					assertAccessForUser(t, allPermsUser, method, path, true)
					assertAccessForUser(t, noPermsUser, method, path, true)
				case 1:
					onlySinglePermUser, _ := srv.NewClientWithRandomUserPermissions(t, requiredPermissions)
					assertAccessForUser(t, onlySinglePermUser, method, path, true)
					assertAccessForUser(t, allPermsUser, method, path, true)
					assertAccessForUser(t, noPermsUser, method, path, false)
				default:
					// Ensure that having only a subset of the required permissions does NOT grant
					// access, the user must have ALL the specified permissions
					for i := range len(requiredPermissions) - 1 {
						subsetPermsUser, _ := srv.NewClientWithRandomUserPermissions(t, requiredPermissions[:i])
						assertAccessForUser(t, subsetPermsUser, method, path, false)
					}

					reqPermsUser, _ := srv.NewClientWithRandomUserPermissions(t, requiredPermissions)
					assertAccessForUser(t, reqPermsUser, method, path, true)
					assertAccessForUser(t, allPermsUser, method, path, true)
					assertAccessForUser(t, noPermsUser, method, path, false)
				}
			})
		}
	}
}
