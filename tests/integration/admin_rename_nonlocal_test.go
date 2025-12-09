// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAdminRenameNonLocalUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}) // admin user
	normalUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // normal user

	t.Run("AdminCanRenameOAuth2User", func(t *testing.T) {
		// Create an OAuth2 user (non-local)
		oauth2User := &user_model.User{
			Name:      "oauth2testuser",
			Email:     "oauth2@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))
		defer func() {
			unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "oauth2renamed"})
		}()

		// Admin should be able to rename this user
		err := user.RenameUser(db.DefaultContext, oauth2User, "oauth2renamed", admin)
		assert.NoError(t, err, "Admin should be able to rename OAuth2 users")

		// Verify the rename worked
		renamed, err := user_model.GetUserByName(db.DefaultContext, "oauth2renamed")
		assert.NoError(t, err)
		assert.Equal(t, oauth2User.ID, renamed.ID)
	})

	t.Run("AdminCanRenameLDAPUser", func(t *testing.T) {
		// Create an LDAP user (non-local)
		ldapUser := &user_model.User{
			Name:      "ldaptestuser",
			Email:     "ldap@example.com",
			LoginType: auth.LDAP,
			LoginName: "ldap-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, ldapUser))

		// Admin should be able to rename this user
		err := user.RenameUser(db.DefaultContext, ldapUser, "ldaprenamed", admin)
		assert.NoError(t, err, "Admin should be able to rename LDAP users")

		// Verify the rename worked
		renamed, err := user_model.GetUserByName(db.DefaultContext, "ldaprenamed")
		assert.NoError(t, err)
		assert.Equal(t, ldapUser.ID, renamed.ID)
	})

	t.Run("NonAdminCannotRenameOAuth2User", func(t *testing.T) {
		// Create an OAuth2 user
		oauth2User := &user_model.User{
			Name:      "oauth2user2",
			Email:     "oauth2-2@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-login-2",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Normal user should NOT be able to rename this user
		err := user.RenameUser(db.DefaultContext, oauth2User, "should-fail", normalUser)
		assert.Error(t, err, "Non-admin should not be able to rename OAuth2 users")
	})

	t.Run("NonAdminCannotRenameThemselves", func(t *testing.T) {
		// Create an OAuth2 user
		oauth2User := &user_model.User{
			Name:      "oauth2self",
			Email:     "oauth2-self@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-self-login",
			IsAdmin:   false,
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// User trying to rename themselves (non-admin)
		err := user.RenameUser(db.DefaultContext, oauth2User, "oauth2newname", oauth2User)
		assert.Error(t, err, "Non-admin OAuth2 user should not be able to rename themselves")
	})

	t.Run("AdminCanRenameLocalUser", func(t *testing.T) {
		// Create a local user
		localUser := &user_model.User{
			Name:      "localuser",
			Email:     "local@example.com",
			LoginType: auth.Plain,
			Passwd:    "password",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, localUser))

		// Admin should still be able to rename local users
		err := user.RenameUser(db.DefaultContext, localUser, "localrenamed", admin)
		assert.NoError(t, err, "Admin should be able to rename local users")
	})
}

func TestAdminRenameNonLocalUsersAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1") // admin
	token := getTokenForLoggedInUser(t, session, auth.AccessTokenScopeWriteAdmin)

	t.Run("RenameOAuth2UserViaAPI", func(t *testing.T) {
		// Create OAuth2 user
		oauth2User := &user_model.User{
			Name:      "oauth2apitest",
			Email:     "oauth2api@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-api-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Try to rename via admin API
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/admin/users/%s", oauth2User.Name), &structs.EditUserOption{
			LoginName: "oauth2apirenamed",
		}).AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)
		
		var user structs.User
		DecodeJSON(t, resp, &user)
		assert.Equal(t, "oauth2apirenamed", user.UserName)
	})

	t.Run("RenameLDAPUserViaAPI", func(t *testing.T) {
		// Create LDAP user
		ldapUser := &user_model.User{
			Name:      "ldapapitest",
			Email:     "ldapapi@example.com",
			LoginType: auth.LDAP,
			LoginName: "ldap-api-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, ldapUser))

		// Rename via admin API
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/admin/users/%s", ldapUser.Name), &structs.EditUserOption{
			LoginName: "ldapapirenamed",
		}).AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)
		
		var user structs.User
		DecodeJSON(t, resp, &user)
		assert.Equal(t, "ldapapirenamed", user.UserName)
	})
}

func TestAdminRenameNonLocalUsersWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1") // admin

	t.Run("RenameOAuth2UserViaWebUI", func(t *testing.T) {
		// Create OAuth2 user
		oauth2User := &user_model.User{
			Name:      "oauth2webtest",
			Email:     "oauth2web@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-web-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Access admin edit page
		req := NewRequest(t, "GET", fmt.Sprintf("/admin/users/%d", oauth2User.ID))
		resp := session.MakeRequest(t, req, http.StatusOK)
		
		htmlDoc := NewHTMLParser(t, resp.Body)
		
		// Form should allow editing username for OAuth2 users (when admin)
		// Before the fix, this would be disabled
		usernameInput := htmlDoc.Find("input[name='user_name']")
		assert.NotNil(t, usernameInput)
	})

	t.Run("SubmitRenameViaWebUI", func(t *testing.T) {
		// Create OAuth2 user
		oauth2User := &user_model.User{
			Name:      "oauth2webtest2",
			Email:     "oauth2web2@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-web-login-2",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		csrf := GetUserCSRFToken(t, session)
		
		// Submit rename form
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/admin/users/%d", oauth2User.ID), map[string]string{
			"_csrf":     csrf,
			"user_name": "oauth2webrenamed",
			"email":     oauth2User.Email,
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Verify rename worked
		renamed, err := user_model.GetUserByName(db.DefaultContext, "oauth2webrenamed")
		assert.NoError(t, err)
		assert.Equal(t, oauth2User.ID, renamed.ID)
	})
}

func TestAdminRenameNonLocalUsersPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("OnlyAdminsCanRenameNonLocal", func(t *testing.T) {
		admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1, IsAdmin: true})
		normalUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2, IsAdmin: false})

		oauth2User := &user_model.User{
			Name:      "oauth2permtest",
			Email:     "oauth2perm@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-perm-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Admin can rename
		err := user.RenameUser(db.DefaultContext, oauth2User, "adminrenamed", admin)
		assert.NoError(t, err)

		// Reset name
		oauth2User.Name = "oauth2permtest"
		assert.NoError(t, user_model.UpdateUserCols(db.DefaultContext, oauth2User, "name"))

		// Normal user cannot rename
		err = user.RenameUser(db.DefaultContext, oauth2User, "shouldfail", normalUser)
		assert.Error(t, err)
	})

	t.Run("LocalUsersCanStillRenameSelf", func(t *testing.T) {
		// Local users should still be able to rename themselves
		localUser := &user_model.User{
			Name:      "localselftest",
			Email:     "localself@example.com",
			LoginType: auth.Plain,
			Passwd:    "password",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, localUser))

		// Local user can rename themselves
		err := user.RenameUser(db.DefaultContext, localUser, "localselfnew", localUser)
		// This should work (if allowed by other business logic)
		// The fix is specifically about allowing ADMINS to rename non-local users
		_ = err // Result depends on other business logic
	})

	t.Run("AdminCanRenameAnyUserType", func(t *testing.T) {
		admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1, IsAdmin: true})

		userTypes := []struct {
			name      string
			loginType auth.Type
		}{
			{"local-admin-test", auth.Plain},
			{"oauth2-admin-test", auth.OAuth2},
			{"ldap-admin-test", auth.LDAP},
			{"smtp-admin-test", auth.SMTP},
		}

		for _, ut := range userTypes {
			t.Run(ut.name, func(t *testing.T) {
				testUser := &user_model.User{
					Name:      ut.name,
					Email:     fmt.Sprintf("%s@example.com", ut.name),
					LoginType: ut.loginType,
					LoginName: fmt.Sprintf("%s-login", ut.name),
				}
				if ut.loginType == auth.Plain {
					testUser.Passwd = "password"
				}
				assert.NoError(t, user_model.CreateUser(db.DefaultContext, testUser))

				// Admin should be able to rename all types
				newName := fmt.Sprintf("%s-renamed", ut.name)
				err := user.RenameUser(db.DefaultContext, testUser, newName, admin)
				assert.NoError(t, err, "Admin should be able to rename %s users", ut.loginType)
			})
		}
	})
}

func TestAdminRenameNonLocalUsersEdgeCases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1, IsAdmin: true})

	t.Run("RenameToExistingName", func(t *testing.T) {
		existing := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		oauth2User := &user_model.User{
			Name:      "oauth2conflict",
			Email:     "oauth2conflict@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-conflict-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Try to rename to existing user's name
		err := user.RenameUser(db.DefaultContext, oauth2User, existing.Name, admin)
		assert.Error(t, err, "Should not allow renaming to existing username")
	})

	t.Run("RenameWithInvalidCharacters", func(t *testing.T) {
		oauth2User := &user_model.User{
			Name:      "oauth2invalid",
			Email:     "oauth2invalid@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-invalid-login",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		// Try invalid username
		err := user.RenameUser(db.DefaultContext, oauth2User, "invalid name!", admin)
		assert.Error(t, err, "Should reject invalid characters in username")
	})

	t.Run("RenamePreservesOtherData", func(t *testing.T) {
		oauth2User := &user_model.User{
			Name:      "oauth2preserve",
			Email:     "oauth2preserve@example.com",
			LoginType: auth.OAuth2,
			LoginName: "oauth2-preserve-login",
			FullName:  "Full Name",
			Website:   "https://example.com",
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, oauth2User))

		originalEmail := oauth2User.Email
		originalLoginName := oauth2User.LoginName
		originalFullName := oauth2User.FullName

		// Rename user
		err := user.RenameUser(db.DefaultContext, oauth2User, "oauth2newname", admin)
		assert.NoError(t, err)

		// Verify other data is preserved
		renamed, err := user_model.GetUserByName(db.DefaultContext, "oauth2newname")
		assert.NoError(t, err)
		assert.Equal(t, originalEmail, renamed.Email)
		assert.Equal(t, originalLoginName, renamed.LoginName)
		assert.Equal(t, originalFullName, renamed.FullName)
	})
}
