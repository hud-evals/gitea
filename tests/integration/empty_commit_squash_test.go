// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

// TestPullSquashMergeEmpty tests that squash merging a PR where all changes
// are already in the target branch (e.g., via cherry-pick) succeeds instead
// of returning a 500 error.
//
// Bug: Before the fix, git commit would fail because there's nothing to commit
// after the squash when all changes were already cherry-picked to master.
// Fix: Add --allow-empty flag to the git commit command in merge_squash.go
//
// See: https://github.com/go-gitea/gitea/pull/35989
func TestPullSquashMergeEmpty(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")

		// Step 1: Create a PR by editing a file on a new branch
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "pr-squash-empty", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user2", "repo1", false, "master", "pr-squash-empty", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])

		// Step 2: Clone the repo and cherry-pick the PR commit to master
		// This simulates the scenario where someone already applied the changes
		httpContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
		dstPath := t.TempDir()

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		// Checkout the PR branch to get its commit
		doGitCheckoutBranch(dstPath, "-b", "pr-squash-empty", "remotes/origin/pr-squash-empty")(t)

		// Switch to master and cherry-pick the commit
		doGitCheckoutBranch(dstPath, "master")(t)
		_, _, err := gitcmd.NewCommand("cherry-pick").AddArguments("pr-squash-empty").
			WithDir(dstPath).
			RunStdString(t.Context())
		assert.NoError(t, err)

		// Push the cherry-picked commit to master
		doGitPushTestRepository(dstPath)(t)

		// Step 3: Try to squash merge the PR
		// On baseline: This will fail with 500 because git commit fails (nothing to commit)
		// On golden: This will succeed because --allow-empty is added
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleSquash,
			DeleteBranch: false,
		})
	})
}

// TestPullSquashMergeEmptyMultipleCommits tests squash merging when multiple
// PR commits are all already in the target branch.
func TestPullSquashMergeEmptyMultipleCommits(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")

		// Create first commit on feature branch
		testEditFileToNewBranch(t, session, "user2", "repo1", "master", "pr-squash-multi", "README.md", "First edit\n")

		// Create PR
		resp := testPullCreate(t, session, "user2", "repo1", false, "master", "pr-squash-multi", "Multi-commit PR")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.Equal(t, "pulls", elem[3])

		// Clone and cherry-pick to master
		httpContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository)
		dstPath := t.TempDir()

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword("user2", userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		doGitCheckoutBranch(dstPath, "-b", "pr-squash-multi", "remotes/origin/pr-squash-multi")(t)
		doGitCheckoutBranch(dstPath, "master")(t)

		_, _, err := gitcmd.NewCommand("cherry-pick").AddArguments("pr-squash-multi").
			WithDir(dstPath).
			RunStdString(t.Context())
		assert.NoError(t, err)

		doGitPushTestRepository(dstPath)(t)

		// Squash merge should succeed even though content is already in master
		testPullMerge(t, session, elem[1], elem[2], elem[4], MergeOptions{
			Style:        repo_model.MergeStyleSquash,
			DeleteBranch: false,
		})
	})
}
