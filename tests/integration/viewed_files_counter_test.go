// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"testing"

	pull_model "code.gitea.io/gitea/models/pull"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// TestViewedFilesCounter tests the viewed files counter functionality.
//
// Bug (before fix): Functions UpdateViewedFilesState, GetViewedFilesCount, and
// HandleFileChange did not exist, making it impossible to track viewed file
// state changes properly.
//
// Fix: Added these helper functions to enable proper viewed file tracking,
// including invalidating viewed status when files are modified.
func TestViewedFilesCounter(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Use unique PR IDs that don't conflict with existing data
	prID1 := int64(99901)
	prID2 := int64(99902)

	t.Run("CounterDecreasesWhenFileChanges", func(t *testing.T) {
		// Mark files as viewed
		err := pull_model.UpdateViewedFilesState(context.TODO(), user2.ID, prID1, []string{"file1.go", "file2.go"})
		assert.NoError(t, err)

		// Verify count
		viewedCount, err := pull_model.GetViewedFilesCount(context.TODO(), user2.ID, prID1)
		assert.NoError(t, err)
		assert.Equal(t, 2, viewedCount)

		// Simulate file change - counter should decrease
		err = pull_model.HandleFileChange(context.TODO(), prID1, "file1.go")
		assert.NoError(t, err)

		viewedCount, err = pull_model.GetViewedFilesCount(context.TODO(), user2.ID, prID1)
		assert.NoError(t, err)
		assert.Equal(t, 1, viewedCount, "Counter should decrease when file changes")
	})

	t.Run("CounterAccuracyWithMultipleUsers", func(t *testing.T) {
		// User1 views file1
		err := pull_model.UpdateViewedFilesState(context.TODO(), user1.ID, prID2, []string{"file1.go"})
		assert.NoError(t, err)

		// User2 views file2
		err = pull_model.UpdateViewedFilesState(context.TODO(), user2.ID, prID2, []string{"file2.go"})
		assert.NoError(t, err)

		// Each user should have their own count
		count1, err := pull_model.GetViewedFilesCount(context.TODO(), user1.ID, prID2)
		assert.NoError(t, err)
		count2, err := pull_model.GetViewedFilesCount(context.TODO(), user2.ID, prID2)
		assert.NoError(t, err)

		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)
	})
}
