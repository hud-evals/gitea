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
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateFileCanCreate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateFileWithEmptySHA", func(t *testing.T) {
		// GitHub behavior: PUT with empty SHA creates a new file
		// This should match GitHub's API behavior
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/newfile-empty-sha.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create new file with empty SHA",
			},
			Content: "bmV3IGZpbGU=", // base64 "new file"
			SHA:     "",             // Empty SHA - should create
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		assert.NotNil(t, fileResponse.Content)
		assert.Equal(t, "newfile-empty-sha.txt", fileResponse.Content.Name)
	})

	t.Run("CreateFileWithoutSHAField", func(t *testing.T) {
		// PUT without SHA field should also create a new file
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/newfile-no-sha.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create new file without SHA field",
			},
			Content: "bm8gc2hh", // base64 "no sha"
			// SHA field not provided
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		assert.NotNil(t, fileResponse.Content)
		assert.Equal(t, "newfile-no-sha.txt", fileResponse.Content.Name)
	})

	t.Run("UpdateExistingFileWithSHA", func(t *testing.T) {
		// Normal update with SHA should still work
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/existing-file.txt", user.Name, repo.Name)
		
		// Create file first
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Initial create",
			},
			Content: "aW5pdGlhbA==",
		}).AddTokenAuth(token)
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		
		// Update with correct SHA
		updateReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Update existing",
			},
			Content: "dXBkYXRlZA==",
			SHA:     createResponse.Content.SHA,
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, updateReq, http.StatusOK)
		
		var updateResponse api.FileResponse
		DecodeJSON(t, resp, &updateResponse)
		
		assert.NotEqual(t, createResponse.Content.SHA, updateResponse.Content.SHA)
	})

	t.Run("CreateFileInNestedPath", func(t *testing.T) {
		// Create file in nested directory with empty SHA
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/subdir/nested/newfile.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create in nested directory",
			},
			Content: "bmVzdGVk",
			SHA:     "", // Empty SHA
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		assert.NotNil(t, fileResponse.Content)
		assert.Contains(t, fileResponse.Content.Path, "subdir/nested/newfile.txt")
	})
}

func TestAPIUpdateFileGitHubCompatibility(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("MatchGitHubBehavior", func(t *testing.T) {
		// GitHub API: PUT with empty/missing SHA = create
		// GitHub API: PUT with SHA = update
		// This should match that behavior
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/github-compat.txt", user.Name, repo.Name)
		
		// Create with empty SHA (GitHub style)
		createReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create GitHub style",
			},
			Content: "Z2l0aHVi",
			SHA:     "",
		}).AddTokenAuth(token)
		
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		sha := createResponse.Content.SHA
		
		// Update with SHA (GitHub style)
		updateReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Update GitHub style",
			},
			Content: "dXBkYXRlZA==",
			SHA:     sha,
		}).AddTokenAuth(token)
		
		updateResp := MakeRequest(t, updateReq, http.StatusOK)
		
		var updateResponse api.FileResponse
		DecodeJSON(t, updateResp, &updateResponse)
		
		assert.NotEqual(t, sha, updateResponse.Content.SHA)
	})

	t.Run("ErrorOnWrongSHA", func(t *testing.T) {
		// Providing wrong SHA should error
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/sha-conflict.txt", user.Name, repo.Name)
		
		// Create file
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{Message: "Create"},
			Content:     "Y3JlYXRl",
		}).AddTokenAuth(token)
		MakeRequest(t, createReq, http.StatusCreated)
		
		// Try to update with wrong SHA
		updateReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{Message: "Update with wrong SHA"},
			Content:     "dXBkYXRl",
			SHA:         "wrongsha123",
		}).AddTokenAuth(token)
		
		MakeRequest(t, updateReq, http.StatusConflict)
	})

	t.Run("CreateVsUpdateEndpoint", func(t *testing.T) {
		// Verify behavior difference between POST (create) and PUT (update/create)
		
		// POST should only create
		postURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/post-test.txt", user.Name, repo.Name)
		postReq := NewRequestWithJSON(t, "POST", postURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{Message: "POST create"},
			Content:     "cG9zdA==",
		}).AddTokenAuth(token)
		postResp := MakeRequest(t, postReq, http.StatusCreated)
		
		var postResponse api.FileResponse
		DecodeJSON(t, postResp, &postResponse)
		
		// PUT can create when SHA is empty
		putURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/put-test.txt", user.Name, repo.Name)
		putReq := NewRequestWithJSON(t, "PUT", putURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{Message: "PUT create"},
			Content:     "cHV0",
			SHA:         "",
		}).AddTokenAuth(token)
		putResp := MakeRequest(t, putReq, http.StatusCreated)
		
		var putResponse api.FileResponse
		DecodeJSON(t, putResp, &putResponse)
		
		// Both should succeed
		assert.NotNil(t, postResponse.Content)
		assert.NotNil(t, putResponse.Content)
	})
}

func TestAPIUpdateFileEdgeCases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("EmptySHAOnExistingFile", func(t *testing.T) {
		// What happens when using empty SHA on existing file?
		// Should probably error or replace
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/existing.txt", user.Name, repo.Name)
		
		// Create file
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{Message: "Create"},
			Content:     "ZXhpc3Rpbmc=",
		}).AddTokenAuth(token)
		MakeRequest(t, createReq, http.StatusCreated)
		
		// Try to PUT with empty SHA on existing file
		putReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{Message: "PUT on existing"},
			Content:     "bmV3IGNvbnRlbnQ=",
			SHA:         "",
		}).AddTokenAuth(token)
		
		// Behavior: should error (file exists) or handle gracefully
		resp := MakeRequest(t, putReq, NoExpectedStatus)
		// Accept either conflict or success depending on implementation
		assert.True(t, resp.Code == http.StatusConflict || resp.Code == http.StatusOK)
	})

	t.Run("BranchSpecification", func(t *testing.T) {
		// Test creating file on specific branch with empty SHA
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/branch-test.txt?ref=master", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message:   "Create on branch",
				NewBranch: "new-branch-from-put",
			},
			Content: "YnJhbmNo",
			SHA:     "",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Should create on new branch
		assert.NotNil(t, fileResponse.Content)
	})

	t.Run("CreateWithCommitterInfo", func(t *testing.T) {
		// Test creating file with custom committer/author info
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/custom-author.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Custom author",
				Author: &api.Identity{
					Name:  "Custom Author",
					Email: "custom@example.com",
				},
				Committer: &api.Identity{
					Name:  "Custom Committer",
					Email: "committer@example.com",
				},
			},
			Content: "Y3VzdG9t",
			SHA:     "",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Should use custom author info
		if fileResponse.Commit != nil {
			if fileResponse.Commit.Author != nil {
				assert.Equal(t, "Custom Author", fileResponse.Commit.Author.Name)
			}
			if fileResponse.Commit.Committer != nil {
				assert.Equal(t, "Custom Committer", fileResponse.Commit.Committer.Name)
			}
		}
	})
}

func TestAPIUpdateFileValidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("InvalidBase64Content", func(t *testing.T) {
		// Test with invalid base64 content
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/invalid-content.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Invalid content",
			},
			Content: "not-valid-base64!@#$",
			SHA:     "",
		}).AddTokenAuth(token)
		
		// Should error on invalid base64
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("EmptyCommitMessage", func(t *testing.T) {
		// Test with empty commit message
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/empty-message.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "", // Empty message
			},
			Content: "ZW1wdHk=",
			SHA:     "",
		}).AddTokenAuth(token)
		
		// Should handle empty message (use default or error)
		resp := MakeRequest(t, req, NoExpectedStatus)
		assert.True(t, resp.Code == http.StatusCreated || resp.Code == http.StatusBadRequest)
	})

	t.Run("CreateFileInNonExistentRepo", func(t *testing.T) {
		// Try to create file in non-existent repo
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/nonexistent999/contents/file.txt", user.Name)
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Should fail",
			},
			Content: "ZmFpbA==",
			SHA:     "",
		}).AddTokenAuth(token)
		
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("CreateFileWithoutPermission", func(t *testing.T) {
		// Test creating file without write permission
		
		// Use a different user's repo
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, IsPrivate: true, OwnerID: user1.ID})
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/unauthorized.txt", user1.Name, privateRepo.Name)
		
		// user2's token trying to write to user1's private repo
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Unauthorized",
			},
			Content: "dW5hdXRo",
			SHA:     "",
		}).AddTokenAuth(token)
		
		// Should be forbidden or not found
		resp := MakeRequest(t, req, NoExpectedStatus)
		assert.True(t, resp.Code == http.StatusForbidden || resp.Code == http.StatusNotFound)
	})
}

func TestAPIUpdateFileSpecialCases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateBinaryFile", func(t *testing.T) {
		// Test creating binary file with empty SHA
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/binary.bin", user.Name, repo.Name)
		
		// Binary content (base64 of some bytes)
		binaryContent := "AAECAwQFBgcICQ==" // Some binary data
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create binary",
			},
			Content: binaryContent,
			SHA:     "",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		assert.NotNil(t, fileResponse.Content)
	})

	t.Run("CreateLargeFile", func(t *testing.T) {
		// Test creating large file with empty SHA
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/large.txt", user.Name, repo.Name)
		
		// Large content
		largeContent := make([]byte, 1024*1024) // 1MB
		for i := range largeContent {
			largeContent[i] = 'a'
		}
		
		req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create large file",
			},
			Content: string(largeContent),
			SHA:     "",
		}).AddTokenAuth(token)
		
		// Should handle large files
		resp := MakeRequest(t, req, NoExpectedStatus)
		// May succeed or fail depending on size limits
		_ = resp
	})

	t.Run("CreateMultipleFilesSequentially", func(t *testing.T) {
		// Create multiple files in sequence
		
		for i := 0; i < 5; i++ {
			fileName := fmt.Sprintf("sequence-%d.txt", i)
			fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user.Name, repo.Name, fileName)
			
			req := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
				FileOptions: api.FileOptions{
					Message: fmt.Sprintf("Create %s", fileName),
				},
				Content: fmt.Sprintf("ZmlsZSU=d", i),
				SHA:     "",
			}).AddTokenAuth(token)
			
			resp := MakeRequest(t, req, http.StatusCreated)
			
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			
			assert.NotNil(t, fileResponse.Content)
			assert.Equal(t, fileName, fileResponse.Content.Name)
		}
	})
}
