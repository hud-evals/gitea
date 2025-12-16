// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

// Test that StartRepositoryTransfer respects creation limits
// Bug: Users can bypass repo limits by receiving repository transfers
// Fix: StartRepositoryTransfer should check if recipient can create repos
func TestStartRepositoryTransferLimitBypass(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Set repository creation limit to 0 - no new repos allowed
	originalLimit := setting.Repository.MaxCreationLimit
	setting.Repository.MaxCreationLimit = 0
	defer func() { setting.Repository.MaxCreationLimit = originalLimit }()

	// Use user2 as doer (repo owner), user10 as recipient
	// user10 is a non-admin who should be subject to limits
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	recipient := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	assert.NoError(t, repo.LoadOwner(db.DefaultContext))

	// Verify recipient is NOT admin and CANNOT create repos (due to limit)
	assert.False(t, recipient.IsAdmin)
	assert.False(t, recipient.CanCreateRepo(), "User should not be able to create repos when limit is 0")

	// Try to start a transfer to the recipient
	// On BASELINE (bug): This succeeds, bypassing the repo limit
	// On GOLDEN (fix): This should fail because recipient can't create repos
	err := StartRepositoryTransfer(db.DefaultContext, doer, recipient, repo, nil)

	// The fix should return an error when recipient can't create repos
	assert.Error(t, err, "Transfer should be blocked when recipient is at repo limit")
}
