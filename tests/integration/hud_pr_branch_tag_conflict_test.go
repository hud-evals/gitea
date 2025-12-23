// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPRCreationWithBranchTagConflict tests that creating a pull request
// works correctly when the target branch has the same name as an existing tag.
// This is a regression test for issue #35470 / PR #35552.
func TestPRCreationWithBranchTagConflict(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrintCurrentTest(t)()

		// Load user and session
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// Create a test repository
		repoName := "pr-branch-tag-conflict-test"
		apiRepo := createRepoForBranchTagTest(t, session, user.Name, repoName)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

		defaultBranch := repo.DefaultBranch

		// Create a branch named "release-v1.0" using API
		conflictingName := "release-v1.0"
		testAPICreateBranch(t, session, user.Name, repoName, defaultBranch, conflictingName, http.StatusCreated)

		// Create an ANNOTATED tag with the same name "release-v1.0"
		// The bug is only triggered by annotated tags (git tag -a), not lightweight tags
		// See issue #35470: lightweight tags work fine, but annotated tags cause 500 error
		gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
		require.NoError(t, err)
		err = gitRepo.CreateAnnotatedTag(conflictingName, "Release v1.0", defaultBranch)
		gitRepo.Close()
		require.NoError(t, err, "Failed to create annotated tag with conflicting name")

		// Create a new branch for the head of the PR
		headBranch := "feature-branch"
		testAPICreateBranch(t, session, user.Name, repoName, defaultBranch, headBranch, http.StatusCreated)

		// Add a file to the head branch via API to create a new commit
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+user.Name+"/"+repoName+"/contents/test-file.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				Message:    "Add test file",
				BranchName: headBranch,
			},
			ContentBase64: "dGVzdCBjb250ZW50IGZvciBQUg==", // base64 of "test content for PR"
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		assert.NotNil(t, resp)

		// Now try to create a PR from headBranch to conflictingName (which has same name as a tag)
		// This is the critical test: when branch and tag have the same name,
		// the git fetch command needs to use full ref names to avoid ambiguity
		t.Run("CreatePRWithBranchTagConflict", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			prReq := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+user.Name+"/"+repoName+"/pulls", &api.CreatePullRequestOption{
				Head:  headBranch,
				Base:  conflictingName, // This branch name conflicts with a tag name
				Title: "Test PR with branch-tag name conflict",
				Body:  "This PR tests that PRs can be created when the target branch has the same name as a tag",
			}).AddTokenAuth(token)

			// The PR creation should succeed, not fail with an ambiguous ref error
			prResp := MakeRequest(t, prReq, http.StatusCreated)

			var pr api.PullRequest
			DecodeJSON(t, prResp, &pr)

			assert.Equal(t, "Test PR with branch-tag name conflict", pr.Title)
			assert.Equal(t, conflictingName, pr.Base.Name)
			assert.Equal(t, headBranch, pr.Head.Name)
		})

		// Cleanup: delete the repository
		doAPIDeleteRepository(NewAPITestContext(t, user.Name, repoName, auth_model.AccessTokenScopeWriteRepository))(t)
	})
}

// createRepoForBranchTagTest creates a repository for testing
func createRepoForBranchTagTest(t *testing.T, session *TestSession, owner, repoName string) *api.Repository {
	// Need both write:repository and write:user scopes for creating repos via /api/v1/user/repos
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		Name:        repoName,
		Description: "Test repository for PR branch-tag conflict",
		AutoInit:    true,
	}).AddTokenAuth(token)

	resp := MakeRequest(t, req, http.StatusCreated)

	var repo api.Repository
	DecodeJSON(t, resp, &repo)
	return &repo
}
