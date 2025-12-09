// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIBranchProtectionForcePushAllowlistUsernames(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Test that ForcePushAllowlistUsernames field is properly read when updating branch protection
	// This tests the fix for issue #35893 where the wrong field was being checked

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Create a test branch
	testBranch := "test-force-push-allowlist"

	t.Run("CreateBranchProtection", func(t *testing.T) {
		// Create branch protection with force push enabled
		enableForcePush := true
		enableForcePushAllowlist := true
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections", &api.CreateBranchProtectionOption{
			BranchName:               testBranch,
			EnableForcePush:          &enableForcePush,
			EnableForcePushAllowlist: &enableForcePushAllowlist,
			ForcePushAllowlistUsernames: []string{user2.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.Equal(t, testBranch, branchProtection.RuleName)
		assert.True(t, branchProtection.EnableForcePush)
		assert.True(t, branchProtection.EnableForcePushAllowlist)
		assert.Equal(t, []string{user2.Name}, branchProtection.ForcePushAllowlistUsernames)
	})

	t.Run("UpdateForcePushAllowlistUsernames", func(t *testing.T) {
		// Update the force push allowlist - this is where the bug was
		// Before the fix, this would fail because it checked ForcePushAllowlistDeployKeys instead of ForcePushAllowlistUsernames
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{user4.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.Equal(t, []string{user4.Name}, branchProtection.ForcePushAllowlistUsernames)

		// Verify in database
		unittest.AssertCount(t, &git_model.ProtectedBranch{RepoID: repo.ID}, 1)
		pb := unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.Equal(t, []int64{user4.ID}, pb.ForcePushAllowlistUserIDs)
	})

	t.Run("UpdateWithMultipleUsers", func(t *testing.T) {
		// Test updating with multiple users in the allowlist
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{user2.Name, user4.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.ElementsMatch(t, []string{user2.Name, user4.Name}, branchProtection.ForcePushAllowlistUsernames)

		// Verify in database
		pb := unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.ElementsMatch(t, []int64{user2.ID, user4.ID}, pb.ForcePushAllowlistUserIDs)
	})

	t.Run("UpdateWithEmptyList", func(t *testing.T) {
		// Test clearing the allowlist by setting it to empty
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.Empty(t, branchProtection.ForcePushAllowlistUsernames)

		// Verify in database
		pb := unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.Empty(t, pb.ForcePushAllowlistUserIDs)
	})

	t.Run("UpdateWithInvalidUsername", func(t *testing.T) {
		// Test that invalid usernames are rejected
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{"nonexistentuser123456"},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("UpdatePreservesOtherSettings", func(t *testing.T) {
		// Ensure that updating force push allowlist doesn't affect other settings
		// First, set some other settings
		enablePush := true
		enablePushWhitelist := true
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			EnablePush:          &enablePush,
			EnablePushWhitelist: &enablePushWhitelist,
			PushWhitelistUsernames: []string{user2.Name},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Now update force push allowlist
		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{user4.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		// Verify both settings are preserved
		assert.Equal(t, []string{user4.Name}, branchProtection.ForcePushAllowlistUsernames)
		assert.Equal(t, []string{user2.Name}, branchProtection.PushWhitelistUsernames)
	})

	t.Run("UpdateNullVsEmptyArray", func(t *testing.T) {
		// Set initial value
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{user2.Name},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Verify it's set
		pb := unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.Equal(t, []int64{user2.ID}, pb.ForcePushAllowlistUserIDs)

		// Patch with null (field not provided) should not change the value
		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			// ForcePushAllowlistUsernames not provided (null)
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Should still have user2
		pb = unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.Equal(t, []int64{user2.ID}, pb.ForcePushAllowlistUserIDs)

		// Patch with empty array should clear it
		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Should now be empty
		pb = unittest.AssertExistsAndLoadBean(t, &git_model.ProtectedBranch{RepoID: repo.ID, RuleName: testBranch})
		assert.Empty(t, pb.ForcePushAllowlistUserIDs)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		req := NewRequestf(t, "DELETE", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPIBranchProtectionForcePushAllowlistDeployKeysIndependent(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Test that ForcePushAllowlistDeployKeys works independently and isn't affected by the username bug

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	testBranch := "test-force-push-deploykeys"

	t.Run("CreateWithDeployKeys", func(t *testing.T) {
		enableForcePush := true
		enableForcePushAllowlist := true
		deployKeys := true
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections", &api.CreateBranchProtectionOption{
			BranchName:                   testBranch,
			EnableForcePush:              &enableForcePush,
			EnableForcePushAllowlist:     &enableForcePushAllowlist,
			ForcePushAllowlistDeployKeys: &deployKeys,
			ForcePushAllowlistUsernames:  []string{user2.Name},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.True(t, branchProtection.ForcePushAllowlistDeployKeys)
		assert.Equal(t, []string{user2.Name}, branchProtection.ForcePushAllowlistUsernames)
	})

	t.Run("UpdateDeployKeysWithoutAffectingUsernames", func(t *testing.T) {
		deployKeys := false
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistDeployKeys: &deployKeys,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.False(t, branchProtection.ForcePushAllowlistDeployKeys)
		// Usernames should still be preserved
		assert.Equal(t, []string{user2.Name}, branchProtection.ForcePushAllowlistUsernames)
	})

	t.Run("UpdateUsernamesWithoutAffectingDeployKeys", func(t *testing.T) {
		// First, re-enable deploy keys
		deployKeys := true
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistDeployKeys: &deployKeys,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		// Now update usernames - this should not affect deploy keys setting
		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch, &api.EditBranchProtectionOption{
			ForcePushAllowlistUsernames: []string{},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var branchProtection api.BranchProtection
		DecodeJSON(t, resp, &branchProtection)
		assert.True(t, branchProtection.ForcePushAllowlistDeployKeys)
		assert.Empty(t, branchProtection.ForcePushAllowlistUsernames)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		req := NewRequestf(t, "DELETE", "/api/v1/repos/"+owner.Name+"/"+repo.Name+"/branch_protections/"+testBranch).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}
