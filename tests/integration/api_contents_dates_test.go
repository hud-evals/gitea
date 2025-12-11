// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// TestAPIContentsDateDefaults tests that file operations use current time
// when dates are not specified.
//
// Bug (before fix): Dates were set on wrong variable (commonOpts instead of
// changeFileOpts), causing commits to use zero time (2001-01-01).
// Fix: Dates are now correctly set on changeFileOpts.
func TestAPIContentsDateDefaults(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateFileWithoutDates", func(t *testing.T) {
		// When creating a file via API without specifying dates, should use current time
		// Bug: Was using zero time (2001-01-01) instead
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-no-dates.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create file without dates",
			},
			ContentBase64: "dGVzdCBjb250ZW50", // base64 "test content"
		}).AddTokenAuth(token)
		
		before := time.Now()
		resp := MakeRequest(t, req, http.StatusCreated)
		after := time.Now()
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Verify dates are set to current time, not zero/2001
		assert.NotNil(t, fileResponse.Commit)
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDateStr := fileResponse.Commit.Committer.Date
			commitDate, err := time.Parse(time.RFC3339, commitDateStr)
			assert.NoError(t, err, "Date should be valid RFC3339")
			
			// Should NOT be year 2001 (zero time defaults to this in some cases)
			assert.NotEqual(t, 2001, commitDate.Year(), "Date should not default to zero time (2001)")
			
			// Should be within reasonable range of now
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, commitDate.Unix(), after.Add(time.Minute).Unix())
		}
	})

	t.Run("UpdateFileWithoutDates", func(t *testing.T) {
		// First create a file
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-update-dates.txt", user.Name, repo.Name)
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Initial create",
			},
			ContentBase64: "aW5pdGlhbA==",
		}).AddTokenAuth(token)
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		
		// Now update without specifying dates
		before := time.Now()
		updateReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Update file",
			},
			ContentBase64: "dXBkYXRlZA==",
			SHA:           createResponse.Content.SHA,
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, updateReq, http.StatusOK)
		after := time.Now()
		
		var updateResponse api.FileResponse
		DecodeJSON(t, resp, &updateResponse)
		
		// Verify update date is current, not zero/2001
		assert.NotNil(t, updateResponse.Commit)
		if updateResponse.Commit != nil && updateResponse.Commit.Committer != nil {
			commitDateStr := updateResponse.Commit.Committer.Date
			commitDate, err := time.Parse(time.RFC3339, commitDateStr)
			assert.NoError(t, err)
			
			assert.NotEqual(t, 2001, commitDate.Year(), "Update date should not be zero time")
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, commitDate.Unix(), after.Add(time.Minute).Unix())
		}
	})
}
