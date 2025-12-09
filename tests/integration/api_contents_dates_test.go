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

func TestAPIContentsDateDefaults(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateFileWithoutDates", func(t *testing.T) {
		// When creating a file via API without specifying dates, should use current time
		// Bug: Was setting to 2001-01-01T00:00:00Z instead
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-no-dates.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create file without dates",
			},
			Content: "dGVzdCBjb250ZW50", // base64 "test content"
			// Dates NOT specified
		}).AddTokenAuth(token)
		
		before := time.Now()
		resp := MakeRequest(t, req, http.StatusCreated)
		after := time.Now()
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Verify dates are set to current time, not 2001-01-01
		assert.NotNil(t, fileResponse.Commit)
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDate := fileResponse.Commit.Committer.Date
			
			// Should be within reasonable range of now
			assert.False(t, commitDate.Year() == 2001, "Date should not default to 2001")
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, commitDate.Unix(), after.Add(time.Minute).Unix())
		}
	})

	t.Run("CreateFileWithSpecifiedDates", func(t *testing.T) {
		// When dates are explicitly specified, should use those dates
		
		specificDate := time.Date(2020, 6, 15, 10, 30, 0, 0, time.UTC)
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-with-dates.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create file with specific dates",
				Dates: &api.CommitDateOptions{
					Author:    specificDate,
					Committer: specificDate,
				},
			},
			Content: "dGVzdCBjb250ZW50",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Should use the specified date
		assert.NotNil(t, fileResponse.Commit)
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDate := fileResponse.Commit.Committer.Date
			assert.Equal(t, specificDate.Year(), commitDate.Year())
			assert.Equal(t, specificDate.Month(), commitDate.Month())
			assert.Equal(t, specificDate.Day(), commitDate.Day())
		}
	})

	t.Run("UpdateFileWithoutDates", func(t *testing.T) {
		// Updating a file without dates should also use current time
		
		// First create a file
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-update.txt", user.Name, repo.Name)
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Initial create",
			},
			Content: "aW5pdGlhbA==", // base64 "initial"
		}).AddTokenAuth(token)
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		
		// Now update it without specifying dates
		before := time.Now()
		updateReq := NewRequestWithJSON(t, "PUT", fileURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Update file",
			},
			Content: "dXBkYXRlZA==", // base64 "updated"
			SHA:     createResponse.Content.SHA,
			// Dates NOT specified
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, updateReq, http.StatusOK)
		after := time.Now()
		
		var updateResponse api.FileResponse
		DecodeJSON(t, resp, &updateResponse)
		
		// Verify update date is current, not 2001
		assert.NotNil(t, updateResponse.Commit)
		if updateResponse.Commit != nil && updateResponse.Commit.Committer != nil {
			commitDate := updateResponse.Commit.Committer.Date
			assert.False(t, commitDate.Year() == 2001)
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, commitDate.Unix(), after.Add(time.Minute).Unix())
		}
	})

	t.Run("DeleteFileWithoutDates", func(t *testing.T) {
		// Deleting a file should also use current time for commit date
		
		// Create file first
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-delete.txt", user.Name, repo.Name)
		createReq := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create for deletion",
			},
			Content: "ZGVsZXRl", // base64 "delete"
		}).AddTokenAuth(token)
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		
		// Delete it
		before := time.Now()
		deleteReq := NewRequestWithJSON(t, "DELETE", fileURL, &api.DeleteFileOptions{
			FileOptions: api.FileOptions{
				Message: "Delete file",
			},
			SHA: createResponse.Content.SHA,
			// Dates NOT specified
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, deleteReq, http.StatusOK)
		after := time.Now()
		
		var deleteResponse api.FileResponse
		DecodeJSON(t, resp, &deleteResponse)
		
		// Verify deletion commit date is current
		if deleteResponse.Commit != nil && deleteResponse.Commit.Committer != nil {
			commitDate := deleteResponse.Commit.Committer.Date
			assert.False(t, commitDate.Year() == 2001)
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
		}
	})
}

func TestAPIContentsDateEdgeCases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("PartialDateSpecification", func(t *testing.T) {
		// Test when only one date is specified
		
		authorDate := time.Date(2021, 3, 15, 14, 0, 0, 0, time.UTC)
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-partial-dates.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create with only author date",
				Dates: &api.CommitDateOptions{
					Author: authorDate,
					// Committer date NOT specified
				},
			},
			Content: "cGFydGlhbA==",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Author date should be set, committer should default to now
		if fileResponse.Commit != nil {
			if fileResponse.Commit.Author != nil {
				assert.Equal(t, authorDate.Year(), fileResponse.Commit.Author.Date.Year())
			}
			if fileResponse.Commit.Committer != nil {
				// Committer should be current time, not 2001
				assert.NotEqual(t, 2001, fileResponse.Commit.Committer.Date.Year())
			}
		}
	})

	t.Run("FutureDates", func(t *testing.T) {
		// Test with future dates
		
		futureDate := time.Now().Add(365 * 24 * time.Hour) // 1 year in future
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-future.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create with future date",
				Dates: &api.CommitDateOptions{
					Author:    futureDate,
					Committer: futureDate,
				},
			},
			Content: "ZnV0dXJl",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Should accept future dates
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			assert.GreaterOrEqual(t, fileResponse.Commit.Committer.Date.Year(), time.Now().Year())
		}
	})

	t.Run("PastDates", func(t *testing.T) {
		// Test with past dates
		
		pastDate := time.Date(2015, 1, 1, 12, 0, 0, 0, time.UTC)
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-past.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create with past date",
				Dates: &api.CommitDateOptions{
					Author:    pastDate,
					Committer: pastDate,
				},
			},
			Content: "cGFzdA==",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Should accept past dates
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			assert.Equal(t, 2015, fileResponse.Commit.Committer.Date.Year())
		}
	})

	t.Run("ZeroValueDate", func(t *testing.T) {
		// Test with zero-value time
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-zero.txt", user.Name, repo.Name)
		
		zeroTime := time.Time{}
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Create with zero time",
				Dates: &api.CommitDateOptions{
					Author:    zeroTime,
					Committer: zeroTime,
				},
			},
			Content: "emVybw==",
		}).AddTokenAuth(token)
		
		before := time.Now()
		resp := MakeRequest(t, req, http.StatusCreated)
		after := time.Now()
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Zero time should be treated as "not specified" and use current time
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDate := fileResponse.Commit.Committer.Date
			// Should NOT be 0001-01-01 (zero time) or 2001-01-01 (bug)
			assert.Greater(t, commitDate.Year(), 2010)
			assert.GreaterOrEqual(t, commitDate.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, commitDate.Unix(), after.Add(time.Minute).Unix())
		}
	})
}

func TestAPIContentsDateConsistency(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("ConsistentDefaultBehavior", func(t *testing.T) {
		// Create multiple files without dates - all should use current time
		
		before := time.Now()
		
		fileNames := []string{"file1.txt", "file2.txt", "file3.txt"}
		var commitDates []time.Time
		
		for _, fileName := range fileNames {
			fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user.Name, repo.Name, fileName)
			
			req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
				FileOptions: api.FileOptions{
					Message: fmt.Sprintf("Create %s", fileName),
				},
				Content: "dGVzdA==",
			}).AddTokenAuth(token)
			
			resp := MakeRequest(t, req, http.StatusCreated)
			
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			
			if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
				commitDates = append(commitDates, fileResponse.Commit.Committer.Date)
			}
		}
		
		after := time.Now()
		
		// All dates should be current, not 2001
		for i, date := range commitDates {
			assert.Greater(t, date.Year(), 2010, "File %d should have current year", i)
			assert.GreaterOrEqual(t, date.Unix(), before.Add(-time.Minute).Unix())
			assert.LessOrEqual(t, date.Unix(), after.Add(time.Minute).Unix())
		}
	})

	t.Run("AuthorVsCommitterDates", func(t *testing.T) {
		// When not specified, author and committer dates should both be current
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-both-dates.txt", user.Name, repo.Name)
		
		before := time.Now()
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Test both dates",
			},
			Content: "Ym90aA==",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		after := time.Now()
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		if fileResponse.Commit != nil {
			// Both author and committer should have current dates
			if fileResponse.Commit.Author != nil {
				assert.Greater(t, fileResponse.Commit.Author.Date.Year(), 2010)
				assert.GreaterOrEqual(t, fileResponse.Commit.Author.Date.Unix(), before.Add(-time.Minute).Unix())
			}
			if fileResponse.Commit.Committer != nil {
				assert.Greater(t, fileResponse.Commit.Committer.Date.Year(), 2010)
				assert.GreaterOrEqual(t, fileResponse.Commit.Committer.Date.Unix(), before.Add(-time.Minute).Unix())
			}
		}
	})

	t.Run("TimeZoneHandling", func(t *testing.T) {
		// Test that different time zones are handled correctly
		
		// Specify date in different timezone
		location, _ := time.LoadLocation("America/New_York")
		dateNY := time.Date(2022, 7, 4, 15, 0, 0, 0, location)
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-timezone.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Test timezone",
				Dates: &api.CommitDateOptions{
					Author:    dateNY,
					Committer: dateNY,
				},
			},
			Content: "dGltZXpvbmU=",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// Date should be preserved (converted to UTC for storage)
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDate := fileResponse.Commit.Committer.Date
			assert.Equal(t, 2022, commitDate.Year())
			assert.Equal(t, time.July, commitDate.Month())
		}
	})
}

func TestAPIContentsDateRegression(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("NotRevertingTo2001Bug", func(t *testing.T) {
		// Specific test that dates don't revert to 2001-01-01
		// This was the bug in #35860
		
		fileURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-not-2001.txt", user.Name, repo.Name)
		
		req := NewRequestWithJSON(t, "POST", fileURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message: "Ensure not 2001",
			},
			Content: "bm90MjAwMQ==",
		}).AddTokenAuth(token)
		
		resp := MakeRequest(t, req, http.StatusCreated)
		
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		
		// CRITICAL: Should NOT be 2001-01-01
		if fileResponse.Commit != nil && fileResponse.Commit.Committer != nil {
			commitDate := fileResponse.Commit.Committer.Date
			
			assert.NotEqual(t, 2001, commitDate.Year(), "BUG: Date defaulted to 2001!")
			assert.NotEqual(t, 1, int(commitDate.Month()), "BUG: Date defaulted to January 2001!")
			assert.NotEqual(t, 1, commitDate.Day(), "BUG: Date defaulted to 2001-01-01!")
			
			// Should be current year
			currentYear := time.Now().Year()
			assert.GreaterOrEqual(t, commitDate.Year(), currentYear-1)
			assert.LessOrEqual(t, commitDate.Year(), currentYear+1)
		}
	})

	t.Run("MultipleOperationsConsistency", func(t *testing.T) {
		// Multiple operations in sequence should all use current dates
		
		baseURL := fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-sequence.txt", user.Name, repo.Name)
		
		// Create
		createReq := NewRequestWithJSON(t, "POST", baseURL, &api.CreateFileOptions{
			FileOptions: api.FileOptions{Message: "Create"},
			Content:     "Y3JlYXRl",
		}).AddTokenAuth(token)
		createResp := MakeRequest(t, createReq, http.StatusCreated)
		
		var createResponse api.FileResponse
		DecodeJSON(t, createResp, &createResponse)
		
		// Update
		updateReq := NewRequestWithJSON(t, "PUT", baseURL, &api.UpdateFileOptions{
			FileOptions: api.FileOptions{Message: "Update"},
			Content:     "dXBkYXRl",
			SHA:         createResponse.Content.SHA,
		}).AddTokenAuth(token)
		updateResp := MakeRequest(t, updateReq, http.StatusOK)
		
		var updateResponse api.FileResponse
		DecodeJSON(t, updateResp, &updateResponse)
		
		// Both should have current dates
		if createResponse.Commit != nil && createResponse.Commit.Committer != nil {
			assert.Greater(t, createResponse.Commit.Committer.Date.Year(), 2010)
		}
		if updateResponse.Commit != nil && updateResponse.Commit.Committer != nil {
			assert.Greater(t, updateResponse.Commit.Committer.Date.Year(), 2010)
		}
	})
}
