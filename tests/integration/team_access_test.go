// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TestTeamMemberCanAccessRepo tests that users in a team with repo access
// can actually access the repository.
//
// Bug (before fix): GetUserIDsWithUnitAccess() only checked direct user access,
// not team memberships, causing team members to be denied access.
//
// Fix: Added organization.GetTeamUserIDsWithAccessToAnyRepoUnit() call to include
// team members when the repo owner is an organization.
func TestTeamMemberCanAccessRepo(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Create organization owner (user 2)
		orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, orgOwner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		// Create a test user who will be added to team
		teamMemberUsername := "teammember"
		teamMemberEmail := "teammember@example.com"
		teamMember := &user_model.User{
			Name:     teamMemberUsername,
			Email:    teamMemberEmail,
			Passwd:   "password123",
			IsActive: true,
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, teamMember, nil))

		// Create organization
		orgName := "testorg"
		orgOpts := api.CreateOrgOption{
			UserName: orgName,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &orgOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: orgName})

		// Create team
		teamName := "developers"
		teamOpts := api.CreateTeamOption{
			Name:       teamName,
			Permission: "write",
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgName), &teamOpts).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var team api.Team
		DecodeJSON(t, resp, &team)

		// Add team member to team
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/members/%s", team.ID, teamMemberUsername)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Create repository in organization
		repoName := "team-access-test-repo"
		repoOpts := api.CreateRepoOption{
			Name: repoName,
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &repoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: org.ID, Name: repoName})

		// Add repository to team
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", team.ID, orgName, repoName)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// KEY TEST: Check if team member can access the repo via GetUsersWithUnitAccess
		// On baseline: team member is NOT included (bug)
		// On golden: team member IS included (fix)
		users, err := access_model.GetUsersWithUnitAccess(db.DefaultContext, repo, perm.AccessModeWrite, 0)
		assert.NoError(t, err)

		// Check if team member is in the list
		foundTeamMember := false
		for _, u := range users {
			if u.ID == teamMember.ID {
				foundTeamMember = true
				break
			}
		}

		// On golden: should find team member
		// On baseline: will NOT find team member (bug)
		assert.True(t, foundTeamMember,
			"Team member should have access to repo through team membership")

		// Additional verification: try to access repo via API as team member
		teamMemberSession := loginUser(t, teamMemberUsername)
		teamMemberToken := getTokenForLoggedInUser(t, teamMemberSession, auth_model.AccessTokenScopeWriteRepository)

		// Try to create a file in the repo - should succeed on golden, fail on baseline
		fileOpts := api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName: repo.DefaultBranch,
				Message:    "Test commit by team member",
			},
			Content: "test content",
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/contents/test.txt", orgName, repoName),
			&fileOpts).AddTokenAuth(teamMemberToken)
		
		// On baseline: may return 403 Forbidden or 404 (access denied)
		// On golden: should return 201 Created (access granted via team)
		resp = MakeRequest(t, req, http.StatusCreated)
		assert.Equal(t, http.StatusCreated, resp.Code,
			"Team member should be able to write to repo through team membership")
	})
}

// TestDirectAccessStillWorks ensures that direct user access still works
// after the team access fix (regression test).
func TestDirectAccessStillWorks(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, orgOwner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		// Create user with direct access
		directUsername := "directuser"
		directUser := &user_model.User{
			Name:     directUsername,
			Email:    "direct@example.com",
			Passwd:   "password123",
			IsActive: true,
		}
		assert.NoError(t, user_model.CreateUser(db.DefaultContext, directUser, nil))

		// Create organization and repo
		orgName := "testorg2"
		orgOpts := api.CreateOrgOption{UserName: orgName}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &orgOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: orgName})

		repoName := "direct-access-repo"
		repoOpts := api.CreateRepoOption{Name: repoName}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &repoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: org.ID, Name: repoName})

		// Add direct collaborator with write access
		req = NewRequest(t, "PUT",
			fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", orgName, repoName, directUsername)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify direct user has access
		users, err := access_model.GetUsersWithUnitAccess(db.DefaultContext, repo, perm.AccessModeWrite, 0)
		assert.NoError(t, err)

		foundDirectUser := false
		for _, u := range users {
			if u.ID == directUser.ID {
				foundDirectUser = true
				break
			}
		}

		assert.True(t, foundDirectUser, "Direct collaborator should have access")
	})
}

// TestNonOrgRepoOwnerAccess ensures individual repository owners still have access.
func TestNonOrgRepoOwnerAccess(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// Create personal (non-org) repository
		repoName := "personal-repo"
		repoOpts := api.CreateRepoOption{
			Name:     repoName,
			AutoInit: true,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &repoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName})

		// Verify owner has access
		users, err := access_model.GetUsersWithUnitAccess(db.DefaultContext, repo, perm.AccessModeWrite, 0)
		assert.NoError(t, err)

		foundOwner := false
		for _, u := range users {
			if u.ID == user.ID {
				foundOwner = true
				break
			}
		}

		assert.True(t, foundOwner, "Repository owner should have access to their own repo")
	})
}
