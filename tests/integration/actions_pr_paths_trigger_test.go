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

func TestActionsPullRequestPathsTrigger(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "actions-pr-paths-basic"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		// Setup directory structure
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir1/file1.txt", "content1")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir2/file2.txt", "content2")

		// Create workflow with path filter
		wfContent := `name: pr-paths-test
on: 
  pull_request:
    paths:
      - 'dir1/**'
jobs:
  test-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'triggered'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/pr-paths.yml", wfContent)

		t.Run("PRModifyingMatchingPath", func(t *testing.T) {
			// Create PR modifying dir1/ - should trigger
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "modify-dir1", "dir1/file1.txt", "modified content")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "modify-dir1")(t)
			assert.NoError(t, err)

			task := runner.fetchTask(t)
			_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)
			assert.Equal(t, webhook_module.HookEventPullRequest, run.Event)
		})

		t.Run("PRModifyingNonMatchingPath", func(t *testing.T) {
			// Create PR modifying dir2/ - should NOT trigger
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "modify-dir2", "dir2/file2.txt", "modified content")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "modify-dir2")(t)
			assert.NoError(t, err)

			// Should not trigger workflow
			runner.fetchNoTask(t)
		})
	})
}

func TestActionsPullRequestPathsRebase(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "actions-pr-paths-rebase"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		// Setup files
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir1/dir1.txt", "1")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "dir2/dir2.txt", "2")

		wfFileContent := `name: ci
on: 
  pull_request:
    paths:
      - 'dir1/**'
jobs:
  ci-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'ci'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/ci.yml", wfFileContent)

		t.Run("RebaseWithoutTrigger", func(t *testing.T) {
			// Create PR modifying dir1/ - workflow triggers
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "update-dir1", "dir1/dir1.txt", "11")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "update-dir1")(t)
			assert.NoError(t, err)
			task1 := runner.fetchTask(t)
			_, _, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
			assert.Equal(t, webhook_module.HookEventPullRequest, run1.Event)

			// Create PR modifying dir2/ - workflow does NOT trigger
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "update-dir2", "dir2/dir2.txt", "22")
			apiPull, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "update-dir2")(t)
			runner.fetchNoTask(t)
			assert.NoError(t, err)

			// Update main branch with change to dir1
			testEditFile(t, session, "user2", repoName, repo.DefaultBranch, "dir1/dir1.txt", "11")

			// Rebase PR (which only modifies dir2/) - should NOT trigger workflow
			req := NewRequestWithValues(t, "POST",
				fmt.Sprintf("/%s/%s/pulls/%d/update?style=rebase", "user2", repoName, apiPull.Index),
				map[string]string{
					"_csrf": GetUserCSRFToken(t, session),
				})
			session.MakeRequest(t, req, http.StatusSeeOther)

			// Workflow should not trigger because PR only modifies dir2/
			runner.fetchNoTask(t)
		})
	})
}

func TestActionsPullRequestPathsMultiplePatterns(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "actions-pr-paths-multi"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		// Setup complex directory structure
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "src/main.go", "package main")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "docs/README.md", "# Docs")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "tests/test.go", "package tests")

		// Workflow with multiple path patterns
		wfContent := `name: multi-path
on: 
  pull_request:
    paths:
      - 'src/**'
      - '**.go'
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'build'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/multi.yml", wfContent)

		t.Run("MatchFirstPattern", func(t *testing.T) {
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "modify-src", "src/main.go", "package main\n// modified")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "modify-src")(t)
			assert.NoError(t, err)
			task := runner.fetchTask(t)
			assert.NotNil(t, task)
		})

		t.Run("MatchSecondPattern", func(t *testing.T) {
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "modify-tests", "tests/test.go", "package tests\n// modified")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "modify-tests")(t)
			assert.NoError(t, err)
			task := runner.fetchTask(t)
			assert.NotNil(t, task)
		})

		t.Run("NoMatch", func(t *testing.T) {
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "modify-docs", "docs/README.md", "# Updated Docs")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "modify-docs")(t)
			assert.NoError(t, err)
			runner.fetchNoTask(t)
		})
	})
}

func TestActionsPullRequestPathsForceUpdate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "actions-pr-paths-force"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "backend/api.go", "package backend")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "frontend/ui.js", "console.log('ui')")

		wfContent := `name: backend-ci
on: 
  pull_request:
    paths:
      - 'backend/**'
jobs:
  backend-build:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'backend build'
`
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/backend.yml", wfContent)

		t.Run("InitialPRTriggers", func(t *testing.T) {
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "backend-change", "backend/api.go", "package backend\n// change")
			pr, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "backend-change")(t)
			assert.NoError(t, err)
			task := runner.fetchTask(t)
			assert.NotNil(t, task)

			// Force update the PR branch with a frontend change
			// This tests that merge base is updated correctly
			testEditFile(t, session, "user2", repoName, "backend-change", "frontend/ui.js", "console.log('updated')")

			// Update main to have backend change
			testEditFile(t, session, "user2", repoName, repo.DefaultBranch, "backend/api.go", "package backend\n// main change")

			// Sync PR with rebase
			req := NewRequestWithValues(t, "POST",
				fmt.Sprintf("/%s/%s/pulls/%d/update?style=rebase", "user2", repoName, pr.Index),
				map[string]string{
					"_csrf": GetUserCSRFToken(t, session),
				})
			session.MakeRequest(t, req, http.StatusSeeOther)

			// After rebase, the PR's diff should only include frontend change
			// So workflow should NOT trigger
			runner.fetchNoTask(t)
		})
	})
}

func TestActionsPullRequestPathsEdgeCases(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repoName := "actions-pr-paths-edge"
		apiRepo := createActionsTestRepo(t, token, repoName, false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		apiCtx := NewAPITestContext(t, "user2", repoName, auth_model.AccessTokenScopeWriteRepository)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, "user2", repoName, "mock-runner", []string{"ubuntu-latest"}, false)

		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "config.yml", "setting: value")
		testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", "app/config.yml", "app: config")

		t.Run("ExactFileMatch", func(t *testing.T) {
			wfContent := `name: exact-file
on: 
  pull_request:
    paths:
      - 'config.yml'
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'check'
`
			testCreateFile(t, session, "user2", repoName, repo.DefaultBranch, "", ".gitea/workflows/exact.yml", wfContent)

			// Modify exact file - should trigger
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "exact-match", "config.yml", "setting: new value")
			_, err := doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "exact-match")(t)
			assert.NoError(t, err)
			task := runner.fetchTask(t)
			assert.NotNil(t, task)

			// Modify different file - should NOT trigger
			testEditFileToNewBranch(t, session, "user2", repoName, repo.DefaultBranch, "not-exact", "app/config.yml", "app: new config")
			_, err = doAPICreatePullRequest(apiCtx, "user2", repoName, repo.DefaultBranch, "not-exact")(t)
			assert.NoError(t, err)
			runner.fetchNoTask(t)
		})
	})
}
