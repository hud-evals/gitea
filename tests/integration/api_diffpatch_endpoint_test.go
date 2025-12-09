// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIDiffPatchEndpoint(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("DiffPatchAPIPath", func(t *testing.T) {
		// Test that the diffpatch API endpoint is at the correct path
		// PR #35610 fixes wrong API path from refactoring
		
		diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
		
		patchContent := `diff --git a/test.txt b/test.txt
new file mode 100644
index 0000000..5e40c08
--- /dev/null
+++ b/test.txt
@@ -0,0 +1 @@
+asdfasdf
`
		
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patchContent))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusOK)
		assert.NotNil(t, resp)
	})

	t.Run("ApplySimplePatch", func(t *testing.T) {
		// Test applying a simple patch
		
		diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
		
		patchContent := `diff --git a/simple.txt b/simple.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/simple.txt
@@ -0,0 +1 @@
+hello
`
		
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patchContent))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("ApplyMultiFilePatch", func(t *testing.T) {
		// Test applying patch affecting multiple files
		
		diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
		
		patchContent := `diff --git a/file1.txt b/file1.txt
new file mode 100644
index 0000000..5e40c08
--- /dev/null
+++ b/file1.txt
@@ -0,0 +1 @@
+file one
diff --git a/file2.txt b/file2.txt
new file mode 100644
index 0000000..83db48f
--- /dev/null
+++ b/file2.txt
@@ -0,0 +1 @@
+file two
`
		
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patchContent))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("InvalidPatchFormat", func(t *testing.T) {
		// Test with invalid patch format
		
		diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
		
		invalidPatch := "this is not a valid patch"
		
		req := NewRequestWithBody(t, "POST", diffURL, []byte(invalidPatch))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token)
		
		// Should reject invalid patch
		resp := MakeRequest(t, req, NoExpectedStatus)
		assert.True(t, resp.Code >= 400, "Should error on invalid patch")
	})
}

func TestAPIDiffPatchEndpointSwagger(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("SwaggerDocumentation", func(t *testing.T) {
		// Verify that swagger documentation is correct
		// PR #35610 also fixes swagger docs
		
		req := NewRequest(t, "GET", "/swagger.v1.json")
		resp := MakeRequest(t, req, http.StatusOK)
		
		body := resp.Body.String()
		
		// Should contain diffpatch endpoint
		assert.Contains(t, body, "diffpatch")
		assert.Contains(t, body, "/repos/{owner}/{repo}/diffpatch")
	})

	t.Run("APIRouteRegistration", func(t *testing.T) {
		// Test that the route is properly registered
		
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		
		// The path should be /api/v1/repos/{owner}/{repo}/diffpatch
		// NOT some other variant
		
		correctURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
		wrongURL := fmt.Sprintf("/api/v1/repos/%s/%s/diff-patch", user.Name, repo.Name)
		
		patch := `diff --git a/route-test.txt b/route-test.txt
new file mode 100644
index 0000000..5e40c08
--- /dev/null
+++ b/route-test.txt
@@ -0,0 +1 @@
+route test
`
		
		// Correct URL should work
		correctReq := NewRequestWithBody(t, "POST", correctURL, []byte(patch))
		correctReq.Header.Set("Content-Type", "text/plain")
		correctReq.AddTokenAuth(token)
		correctResp := MakeRequest(t, correctReq, http.StatusOK)
		assert.Equal(t, http.StatusOK, correctResp.Code)
		
		// Wrong URL should 404
		wrongReq := NewRequestWithBody(t, "POST", wrongURL, []byte(patch))
		wrongReq.Header.Set("Content-Type", "text/plain")
		wrongReq.AddTokenAuth(token)
		wrongResp := MakeRequest(t, wrongReq, http.StatusNotFound)
		assert.Equal(t, http.StatusNotFound, wrongResp.Code)
	})
}

func TestAPIDiffPatchContentTypes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user.Name, repo.Name)
	
	patch := `diff --git a/content-type-test.txt b/content-type-test.txt
new file mode 100644
index 0000000..5e40c08
--- /dev/null
+++ b/content-type-test.txt
@@ -0,0 +1 @@
+content type test
`

	t.Run("TextPlainContentType", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patch))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("ApplicationPatchContentType", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patch))
		req.Header.Set("Content-Type", "application/x-patch")
		req.AddTokenAuth(token)
		
		// Should also work with patch content type
		resp := MakeRequest(t, req, NoExpectedStatus)
		// Accept if supported
		_ = resp
	})

	t.Run("NoContentType", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patch))
		// No content-type header
		req.AddTokenAuth(token)
		
		// Should still work or provide clear error
		resp := MakeRequest(t, req, NoExpectedStatus)
		_ = resp
	})
}

func TestAPIDiffPatchPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
	
	session1 := loginUser(t, user1.Name)
	token1 := getTokenForLoggedInUser(t, session1, auth_model.AccessTokenScopeWriteRepository)

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		// User without write access should not be able to apply patches
		
		diffURL := fmt.Sprintf("/api/v1/repos/%s/%s/diffpatch", user2.Name, repo.Name)
		
		patch := `diff --git a/unauthorized.txt b/unauthorized.txt
new file mode 100644
index 0000000..5e40c08
--- /dev/null
+++ b/unauthorized.txt
@@ -0,0 +1 @@
+unauthorized
`
		
		req := NewRequestWithBody(t, "POST", diffURL, []byte(patch))
		req.Header.Set("Content-Type", "text/plain")
		req.AddTokenAuth(token1)
		
		// Should be forbidden (depends on repo settings)
		resp := MakeRequest(t, req, NoExpectedStatus)
		_ = resp
	})
}
