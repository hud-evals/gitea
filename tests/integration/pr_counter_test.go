// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TestPRCounterAccuracy tests that PR counters remain accurate when PRs are
// closed and reopened.
//
// Bug (before fix): UpdateRepoIssueNumbers() recalculated counts, leading to
// inaccurate counters when PRs were closed/reopened.
//
// Fix: Replace with IncrRepoIssueNumbers() and DecrRepoIssueNumbers() that
// increment/decrement specific counters based on the operation.
func TestPRCounterAccuracy(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create test repository
		repoName := "pr-counter-test"
		apiRepoOpts := api.CreateRepoOption{
			Name:          repoName,
			DefaultBranch: "main",
			AutoInit:      true,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &apiRepoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: repoName})

		// Record initial counters
		initialNumPulls := repo.NumPulls
		initialNumClosedPulls := repo.NumClosedPulls

		// Create 3 PRs
		for i := 1; i <= 3; i++ {
			// Create a branch
			branchName := fmt.Sprintf("test-branch-%d", i)
			fileName := fmt.Sprintf("test%d.txt", i)
			fileOpts := &api.CreateFileOptions{
				FileOptions: api.FileOptions{
					BranchName:    branchName,
					NewBranchName: branchName,
					Message:       fmt.Sprintf("Add %s", fileName),
				},
				ContentBase64: "dGVzdCBjb250ZW50", // "test content" base64
			}
			req = NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repoName, fileName),
				&fileOpts).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			// Create PR
			prOpts := &api.CreatePullRequestOption{
				Head:  branchName,
				Base:  repo.DefaultBranch,
				Title: fmt.Sprintf("Test PR %d", i),
			}
			req = NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls", user2.Name, repoName),
				&prOpts).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Reload repo to get updated counters
		err := repo.LoadAttributes(context.TODO())
		assert.NoError(t, err)
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// Verify total counter increased by 3
		assert.Equal(t, initialNumPulls+3, repo.NumPulls, "Total PR count should increase by 3")
		assert.Equal(t, initialNumClosedPulls, repo.NumClosedPulls, "Closed PR count should not change yet")

		// Close 2 PRs (PRs with index 1 and 2)
		for prIndex := int64(1); prIndex <= 2; prIndex++ {
			req = NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, prIndex),
				&api.EditPullRequestOption{
					State: "closed",
				}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Reload repo
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// Verify closed counter increased by 2
		assert.Equal(t, initialNumPulls+3, repo.NumPulls, "Total PR count should still be +3")
		assert.Equal(t, initialNumClosedPulls+2, repo.NumClosedPulls, "Closed PR count should increase by 2")

		// Reopen 1 PR (PR with index 1)
		req = NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls/1", user2.Name, repoName),
			&api.EditPullRequestOption{
				State: "open",
			}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// Reload repo
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// CRITICAL TEST: On baseline, counter may be wrong due to recalculation bug
		// On golden, closed counter should decrease by 1
		assert.Equal(t, initialNumPulls+3, repo.NumPulls, "Total PR count should still be +3")
		assert.Equal(t, initialNumClosedPulls+1, repo.NumClosedPulls,
			"Closed PR count should decrease to +1 after reopening one PR")

		// Close all remaining open PRs (PR 1 and 3)
		for _, prIndex := range []int64{1, 3} {
			req = NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, prIndex),
				&api.EditPullRequestOption{
					State: "closed",
				}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Reload repo
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// Final verification: all 3 PRs should be counted as closed
		assert.Equal(t, initialNumPulls+3, repo.NumPulls, "Final total PR count should be +3")
		assert.Equal(t, initialNumClosedPulls+3, repo.NumClosedPulls,
			"Final closed PR count should be +3 (all PRs closed)")
	})
}

// TestPRCounterMixedOperations tests counter accuracy with various operations.
func TestPRCounterMixedOperations(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create test repository
		repoName := "pr-counter-mixed-test"
		apiRepoOpts := api.CreateRepoOption{
			Name:          repoName,
			DefaultBranch: "main",
			AutoInit:      true,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &apiRepoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: repoName})
		initialNumPulls := repo.NumPulls
		initialNumClosedPulls := repo.NumClosedPulls

		// Create PR 1 and close immediately
		createAndClosePR := func(i int) {
			branchName := fmt.Sprintf("branch-%d", i)
			fileName := fmt.Sprintf("file%d.txt", i)
			fileOpts := &api.CreateFileOptions{
				FileOptions: api.FileOptions{
					BranchName:    branchName,
					NewBranchName: branchName,
					Message:       fmt.Sprintf("Add %s", fileName),
				},
				ContentBase64: "Y29udGVudA==", // "content" base64
			}
			req := NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repoName, fileName),
				&fileOpts).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			prOpts := &api.CreatePullRequestOption{
				Head:  branchName,
				Base:  repo.DefaultBranch,
				Title: fmt.Sprintf("PR %d", i),
			}
			req = NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls", user2.Name, repoName),
				&prOpts).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)

			// Close it
			req = NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, i),
				&api.EditPullRequestOption{State: "closed"}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Create and close 2 PRs
		createAndClosePR(1)
		createAndClosePR(2)

		// Verify counters
		repo, err := repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumPulls+2, repo.NumPulls)
		assert.Equal(t, initialNumClosedPulls+2, repo.NumClosedPulls)

		// Reopen both
		for i := 1; i <= 2; i++ {
			req := NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, i),
				&api.EditPullRequestOption{State: "open"}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Verify counters decreased
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumPulls+2, repo.NumPulls, "Total should remain +2")
		assert.Equal(t, initialNumClosedPulls, repo.NumClosedPulls,
			"Closed count should return to initial after reopening both")
	})
}

// TestPRCounterWithMerge tests counter accuracy when PRs are merged.
func TestPRCounterWithMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create test repository
		repoName := "pr-counter-merge-test"
		apiRepoOpts := api.CreateRepoOption{
			Name:          repoName,
			DefaultBranch: "main",
			AutoInit:      true,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &apiRepoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: repoName})
		initialNumPulls := repo.NumPulls
		initialNumClosedPulls := repo.NumClosedPulls

		// Create PR
		branchName := "test-merge-branch"
		fileOpts := &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName:    branchName,
				NewBranchName: branchName,
				Message:       "Add merge test file",
			},
			ContentBase64: "bWVyZ2UgdGVzdCBjb250ZW50", // "merge test content" base64
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/contents/merge-test.txt", user2.Name, repoName),
			&fileOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		prOpts := &api.CreatePullRequestOption{
			Head:  branchName,
			Base:  repo.DefaultBranch,
			Title: "Merge Test PR",
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls", user2.Name, repoName),
			&prOpts).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var pr api.PullRequest
		DecodeJSON(t, resp, &pr)

		// Merge the PR
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", user2.Name, repoName, pr.Index),
			&api.MergePullRequestOption{
				Do:            "merge",
				MergeTitleField: "Merge PR",
			}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Verify counters: merged PR counts as closed
		repo, err := repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumPulls+1, repo.NumPulls, "Total PR count should increase by 1")
		assert.Equal(t, initialNumClosedPulls+1, repo.NumClosedPulls,
			"Closed PR count should increase by 1 (merged counts as closed)")

		// Verify PR state
		prModel, err := issues_model.GetPullRequestByIndex(context.TODO(), repo.ID, pr.Index)
		assert.NoError(t, err)
		assert.True(t, prModel.HasMerged, "PR should be marked as merged")
	})
}
