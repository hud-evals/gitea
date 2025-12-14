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
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

// TestPRPathsRebaseNoFalseTrigger tests that when a PR modifying only dir2
// is rebased after main has changes to dir1, the workflow filtering on dir1/**
// does NOT trigger incorrectly.
//
// Bug: Before the fix, rebasing would cause the workflow to trigger because
// the merge_base wasn't updated, so the diff included changes from main.
func TestPRPathsRebaseNoFalseTrigger(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "pr-paths-rebase-test"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		// Setup: create dir1 and dir2
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir1/file.txt", "1")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir2/file.txt", "2")

		// Workflow that only triggers on dir1/** changes
		wfContent := `name: dir1-only
on: 
  pull_request:
    paths:
      - 'dir1/**'
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'triggered on dir1 change'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/dir1.yml", wfContent)

		// Create PR that only modifies dir2/ - should NOT trigger workflow
		testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "update-dir2-only", "dir2/file.txt", "modified dir2")
		apiPull, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "update-dir2-only")(t)
		assert.NoError(t, err)

		// Verify: workflow should NOT trigger for dir2-only change
		runner.fetchNoTask(t)

		// Now modify dir1 on main branch (this is the key setup for the bug)
		testEditFile(t, session, "user2", repoName, repo.DefaultBranch, "dir1/file.txt", "main branch change to dir1")

		// Rebase the PR (which only modifies dir2)
		req := NewRequestWithValues(t, "POST",
			fmt.Sprintf("/%s/%s/pulls/%d/update?style=rebase", "user2", repoName, apiPull.Index),
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
			})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// KEY ASSERTION: After rebase, workflow should still NOT trigger
		// because the PR's diff (against updated merge_base) only includes dir2 changes.
		// 
		// BUG BEHAVIOR (before fix): Workflow would trigger because merge_base
		// wasn't updated, so diff incorrectly included dir1 changes from main.
		runner.fetchNoTask(t)
	})
}

// TestPRPathsTriggerBasic verifies basic path filtering works
func TestPRPathsTriggerBasic(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "pr-paths-basic-test"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		// Setup
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "src/code.go", "package main")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "docs/readme.md", "# Docs")

		// Workflow triggers only on src/** changes
		wfContent := `name: build
on: 
  pull_request:
    paths:
      - 'src/**'
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'build'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/build.yml", wfContent)

		// PR modifying src/ - should trigger
		testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "update-src", "src/code.go", "package main\n// updated")
		_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "update-src")(t)
		assert.NoError(t, err)

		task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)
		assert.Equal(t, webhook_module.HookEventPullRequest, run.Event)
	})
}
