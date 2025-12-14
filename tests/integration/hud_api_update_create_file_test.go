// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TestAPIUpdateCanCreate tests that PUT endpoint can create files when SHA is empty.
//
// Bug (before fix): PUT endpoint required SHA, couldn't create new files.
// Fix: PUT endpoint now creates files when SHA is empty (GitHub API compatible).
func TestAPIUpdateCanCreate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		t.Run("PutWithEmptySHACreates", func(t *testing.T) {
			// PUT with empty SHA should create file and return 201 Created
			fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/put-create-test.txt", user.Name, repo.Name)

			// Use map to avoid struct compatibility issues between baseline/golden
			reqBody := map[string]interface{}{
				"message": "Create via PUT with empty SHA",
				"content": "bmV3IGZpbGUgY29udGVudA==", // base64 "new file content"
				"sha":     "",                          // Empty SHA = create
			}

			req := NewRequestWithJSON(t, "PUT", fileURL, reqBody).AddTokenAuth(token)

			// On baseline: fails (SHA required) - returns 422 or 400
			// On golden: succeeds - returns 201 Created
			resp := MakeRequest(t, req, http.StatusCreated)

			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)

			assert.NotNil(t, fileResponse.Content)
			assert.Equal(t, "put-create-test.txt", fileResponse.Content.Name)
			assert.NotEmpty(t, fileResponse.Content.SHA)
		})

		t.Run("PutWithSHAUpdates", func(t *testing.T) {
			// First create a file via POST
			fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/put-update-test.txt", user.Name, repo.Name)

			createBody := map[string]interface{}{
				"message": "Initial create",
				"content": "aW5pdGlhbCBjb250ZW50", // base64 "initial content"
			}
			createReq := NewRequestWithJSON(t, "POST", fileURL, createBody).AddTokenAuth(token)
			createResp := MakeRequest(t, createReq, http.StatusCreated)

			var createResponse api.FileResponse
			DecodeJSON(t, createResp, &createResponse)
			originalSHA := createResponse.Content.SHA

			// PUT with SHA should update and return 200 OK
			updateBody := map[string]interface{}{
				"message": "Update via PUT with SHA",
				"content": "dXBkYXRlZCBjb250ZW50", // base64 "updated content"
				"sha":     originalSHA,
			}
			updateReq := NewRequestWithJSON(t, "PUT", fileURL, updateBody).AddTokenAuth(token)
			updateResp := MakeRequest(t, updateReq, http.StatusOK)

			var updateResponse api.FileResponse
			DecodeJSON(t, updateResp, &updateResponse)

			assert.NotNil(t, updateResponse.Content)
			assert.NotEqual(t, originalSHA, updateResponse.Content.SHA, "SHA should change after update")
		})
	})
}
