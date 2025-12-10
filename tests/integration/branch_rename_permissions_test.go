// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// Test that non-admin users cannot rename the default branch
// Bug: Non-admin users with write access can rename default/protected branches
// Fix: Only repo admins or site admins should be able to rename these branches
func TestDefaultBranchRenamePermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Test case: Non-admin user with WRITE access tries to rename the default branch
	// user4 has write access (mode=2) to org3/repo3 but is NOT a repo admin
	// On baseline (bug): This succeeds (HTTP 204) - transfer completes!
	// On golden (fix): This should return 403 Forbidden

	token := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/org3/repo3/branches/master", &api.UpdateBranchRepoOption{
		Name: "new-default-name",
	}).AddTokenAuth(token)

	// The fix should return 403 Forbidden when non-admin tries to rename default branch
	resp := MakeRequest(t, req, http.StatusForbidden)

	// Verify the response indicates permission denied
	body := resp.Body.String()
	assert.NotEmpty(t, body, "Response body should contain error message")
}
