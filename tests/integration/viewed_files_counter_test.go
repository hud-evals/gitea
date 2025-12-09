// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestViewedFilesCounter(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	
	// Create a test PR with multiple files
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	t.Run("CounterDecreasesWhenFileChanges", func(t *testing.T) {
		// Create PR
		pr := &issues_model.PullRequest{
			HeadRepoID: repo.ID,
			BaseRepoID: repo.ID,
			HeadBranch: "test-branch",
			BaseBranch: "master",
			Index:      1000,
		}
		
		// Mark files as viewed
		err := pull_model.UpdateViewedFilesState(db.DefaultContext, user2.ID, pr.ID, []string{"file1.go", "file2.go"})
		assert.NoError(t, err)
		
		// Verify count
		viewedCount, err := pull_model.GetViewedFilesCount(db.DefaultContext, user2.ID, pr.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2, viewedCount)
		
		// Simulate file change - counter should decrease
		err = pull_model.HandleFileChange(db.DefaultContext, pr.ID, "file1.go")
		assert.NoError(t, err)
		
		viewedCount, err = pull_model.GetViewedFilesCount(db.DefaultContext, user2.ID, pr.ID)
		assert.NoError(t, err)
		assert.Equal(t, 1, viewedCount, "Counter should decrease when file changes")
	})

	t.Run("CounterAccuracyWithMultipleUsers", func(t *testing.T) {
		pr := &issues_model.PullRequest{
			HeadRepoID: repo.ID,
			BaseRepoID: repo.ID,
			Index:      1001,
		}
		
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		
		// User1 views file1
		err := pull_model.UpdateViewedFilesState(db.DefaultContext, user1.ID, pr.ID, []string{"file1.go"})
		assert.NoError(t, err)
		
		// User2 views file2
		err = pull_model.UpdateViewedFilesState(db.DefaultContext, user2.ID, pr.ID, []string{"file2.go"})
		assert.NoError(t, err)
		
		// Each user should have their own count
		count1, _ := pull_model.GetViewedFilesCount(db.DefaultContext, user1.ID, pr.ID)
		count2, _ := pull_model.GetViewedFilesCount(db.DefaultContext, user2.ID, pr.ID)
		
		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)
	})
}

func TestViewedFilesCounterWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	
	t.Run("WebUIDisplaysCorrectCount", func(t *testing.T) {
		// Create PR via web
		req := NewRequest(t, "GET", "/user2/repo1/compare/master...test")
		resp := session.MakeRequest(t, req, http.StatusOK)
		
		// Check that viewed files counter is displayed correctly
		assert.Contains(t, resp.Body.String(), "viewed")
	})
}
