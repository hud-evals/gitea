// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// TestAdminCanRenameNonLocalUser tests that admins can rename non-local users
// via the admin rename API endpoint.
//
// Bug (before fix): The rename API rejected all renames of non-local users.
// Fix: The rename API now allows admins to rename non-local users.
func TestAdminCanRenameNonLocalUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Create an OAuth2 user (non-local)
	oauth2User := &user_model.User{
		Name:      "oauth2renametest",
		Email:     "oauth2rename@example.com",
		LoginType: auth.OAuth2,
		LoginName: "oauth2-rename-login",
		Type:      user_model.UserTypeIndividual,
	}
	assert.NoError(t, user_model.CreateUser(context.TODO(), oauth2User, nil))

	// Get admin session and token
	adminSession := loginUser(t, "user1") // user1 is admin
	adminToken := getTokenForLoggedInUser(t, adminSession, auth.AccessTokenScopeWriteAdmin)

	newName := "oauth2renamed123"

	// Admin tries to rename the OAuth2 user via the rename API endpoint
	// Endpoint: POST /api/v1/admin/users/{username}/rename
	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/api/v1/admin/users/%s/rename", oauth2User.Name),
		&api.RenameUserOption{
			NewName: newName,
		}).AddTokenAuth(adminToken)

	// On baseline: This returns 500/403 because non-local users can't be renamed
	// On golden: This returns 204 No Content (success)
	adminSession.MakeRequest(t, req, http.StatusNoContent)

	// Verify the user was actually renamed
	renamedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: oauth2User.ID})
	assert.Equal(t, newName, renamedUser.Name, "OAuth2 user should be renamed by admin")
}

// TestAdminCanRenameLDAPUser tests that admins can rename LDAP users.
func TestAdminCanRenameLDAPUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Create an LDAP user (non-local)
	ldapUser := &user_model.User{
		Name:      "ldaprenametest",
		Email:     "ldaprename@example.com",
		LoginType: auth.LDAP,
		LoginName: "ldap-rename-login",
		Type:      user_model.UserTypeIndividual,
	}
	assert.NoError(t, user_model.CreateUser(context.TODO(), ldapUser, nil))

	// Get admin session and token
	adminSession := loginUser(t, "user1")
	adminToken := getTokenForLoggedInUser(t, adminSession, auth.AccessTokenScopeWriteAdmin)

	newName := "ldaprenamed123"

	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/api/v1/admin/users/%s/rename", ldapUser.Name),
		&api.RenameUserOption{
			NewName: newName,
		}).AddTokenAuth(adminToken)

	// Should succeed on golden, fail on baseline
	adminSession.MakeRequest(t, req, http.StatusNoContent)

	// Verify rename
	renamedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: ldapUser.ID})
	assert.Equal(t, newName, renamedUser.Name, "LDAP user should be renamed by admin")
}
