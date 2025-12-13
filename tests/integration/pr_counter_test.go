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
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// ptrString returns a pointer to the given string
func ptrString(s string) *string {
	return &s
}

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
					BranchName:    repo.DefaultBranch,
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
					State: ptrString("closed"),
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
				State: ptrString("open"),
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
					State: ptrString("closed"),
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

		// Helper to create a PR
		createPR := func(i int) {
			branchName := fmt.Sprintf("branch-%d", i)
			fileName := fmt.Sprintf("file%d.txt", i)
			fileOpts := &api.CreateFileOptions{
				FileOptions: api.FileOptions{
					BranchName:    repo.DefaultBranch,
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
		}

		// Helper to close a PR
		closePR := func(i int) {
			req := NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, i),
				&api.EditPullRequestOption{State: ptrString("closed")}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Helper to reopen a PR
		reopenPR := func(i int) {
			req := NewRequestWithJSON(t, "PATCH",
				fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", user2.Name, repoName, i),
				&api.EditPullRequestOption{State: ptrString("open")}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Create 4 PRs
		for i := 1; i <= 4; i++ {
			createPR(i)
		}

		// Verify all 4 PRs created and open
		repo, err := repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumPulls+4, repo.NumPulls, "Should have 4 PRs")
		assert.Equal(t, initialNumClosedPulls, repo.NumClosedPulls, "No PRs closed yet")

		// Close PRs 1, 2, 3 (but not 4)
		closePR(1)
		closePR(2)
		closePR(3)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumClosedPulls+3, repo.NumClosedPulls, "3 PRs should be closed")

		// Reopen PR 1
		reopenPR(1)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumClosedPulls+2, repo.NumClosedPulls, "2 PRs should be closed after reopening 1")

		// Close PR 1 again
		closePR(1)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumClosedPulls+3, repo.NumClosedPulls, "3 PRs closed again")

		// Reopen all closed PRs (1, 2, 3)
		reopenPR(1)
		reopenPR(2)
		reopenPR(3)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumClosedPulls, repo.NumClosedPulls, "All PRs should be open now")

		// Close PR 4 (first time for this one)
		closePR(4)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumClosedPulls+1, repo.NumClosedPulls, "Only PR 4 should be closed")

		// Rapid close/reopen cycles to stress the counter logic
		for cycle := 0; cycle < 3; cycle++ {
			closePR(1)
			closePR(2)
			repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
			assert.NoError(t, err)
			assert.Equal(t, initialNumClosedPulls+3, repo.NumClosedPulls,
				"After closing 1,2 in cycle %d: expected 3 closed (1,2,4)", cycle)

			reopenPR(1)
			reopenPR(2)
			repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
			assert.NoError(t, err)
			assert.Equal(t, initialNumClosedPulls+1, repo.NumClosedPulls,
				"After reopening 1,2 in cycle %d: expected 1 closed (4)", cycle)
		}

		// Final state verification
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)
		assert.Equal(t, initialNumPulls+4, repo.NumPulls, "Final: should have 4 total PRs")
		assert.Equal(t, initialNumClosedPulls+1, repo.NumClosedPulls, "Final: only PR 4 should be closed")
	})
}

// TestPRCounterUsesIncrementNotRecalculation tests that PR counters use
// atomic increment/decrement operations rather than recalculating from scratch.
//
// This test intentionally corrupts counter values to detect if the code is
// recalculating (baseline bug) vs incrementing (golden fix).
//
// Expected behavior:
// - Baseline: Recalculates to actual count (test FAILS) ❌
// - Golden: Increments from corrupted value (test PASSES) ✅
func TestPRCounterUsesIncrementNotRecalculation(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// Create test repository
		repoName := "pr-counter-increment-test"
		apiRepoOpts := api.CreateRepoOption{
			Name:          repoName,
			DefaultBranch: "main",
			AutoInit:      true,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &apiRepoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: repoName})

		// CRITICAL: Intentionally corrupt counters to detect recalculation
		corruptedCount := int64(9999)
		repo.NumPulls = corruptedCount
		repo.NumClosedPulls = 0

		// Directly update the database with corrupted values
		err := unittest.GetEngine(context.TODO()).ID(repo.ID).
			Cols("num_pulls", "num_closed_pulls").
			Update(repo)
		assert.NoError(t, err, "Should successfully corrupt counter values for testing")

		// Create a branch and PR (should INCREMENT, not recalculate)
		branchName := "test-branch"
		fileName := "test.txt"
		fileOpts := &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName:    repo.DefaultBranch,
				NewBranchName: branchName,
				Message:       "Add test file",
			},
			ContentBase64: "dGVzdA==", // "test" base64
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repoName, fileName),
			&fileOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		prOpts := &api.CreatePullRequestOption{
			Head:  branchName,
			Base:  repo.DefaultBranch,
			Title: "Test PR",
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls", user2.Name, repoName),
			&prOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// Reload repo to get updated counters
		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// CRITICAL ASSERTION: This detects recalculation vs increment
		// Baseline (recalculates): counter will be 1 (actual count) → TEST FAILS
		// Golden (increments): counter will be 10000 (9999 + 1) → TEST PASSES
		assert.Equal(t, corruptedCount+1, repo.NumPulls,
			"Counter should be INCREMENTED from corrupted value (9999+1=10000), not recalculated. "+
				"If actual=%d (close to 1), code is RECALCULATING (baseline bug). "+
				"If actual=%d (10000), code is INCREMENTING (golden fix).",
			repo.NumPulls, corruptedCount+1)

		// Test that closing also increments (not recalculates)
		req = NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls/1", user2.Name, repoName),
			&api.EditPullRequestOption{State: ptrString("closed")}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// NumClosedPulls should increment from 0 to 1 (not recalculate)
		assert.Equal(t, int64(1), repo.NumClosedPulls,
			"Closed counter should INCREMENT from 0 to 1, not recalculate")

		// Test that reopening decrements (not recalculates)
		req = NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/repos/%s/%s/pulls/1", user2.Name, repoName),
			&api.EditPullRequestOption{State: ptrString("open")}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo, err = repo_model.GetRepositoryByID(context.TODO(), repo.ID)
		assert.NoError(t, err)

		// NumClosedPulls should decrement from 1 to 0 (not recalculate)
		assert.Equal(t, int64(0), repo.NumClosedPulls,
			"Closed counter should DECREMENT from 1 to 0, not recalculate")

		// Total count should still be at the corrupted + 1 value
		assert.Equal(t, corruptedCount+1, repo.NumPulls,
			"Total counter should remain at incremented value (10000)")
	})
}

