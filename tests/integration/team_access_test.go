// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TestTeamMemberCanAccessRepo tests that users in a team with repo access
// can actually access the repository through API operations.
//
// Bug (before fix): Team members could not access organization repositories
// they had team-based access to, because the access check only looked at
// direct user collaborations, not team memberships.
//
// Fix: The access checking logic was updated to include team members when
// checking repository permissions for organization-owned repositories.
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
			IsActive: true,
		}
		assert.NoError(t, teamMember.SetPassword("password"))
		assert.NoError(t, user_model.CreateUser(context.Background(), teamMember, nil,
			&user_model.CreateUserOverwriteOptions{
				IsActive: optional.Some(true),
			}))

		// Create organization
		orgName := "testorg-team-access"
		orgOpts := api.CreateOrgOption{
			UserName: orgName,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &orgOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: orgName})

		// Create team with write permissions
		teamName := "developers"
		teamOpts := api.CreateTeamOption{
			Name:       teamName,
			Permission: "write",
			Units:      []string{"repo.code", "repo.issues", "repo.pulls"},
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgName), &teamOpts).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var team api.Team
		DecodeJSON(t, resp, &team)

		// Add team member to team
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/members/%s", team.ID, teamMemberUsername)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Create private repository in organization
		repoName := "team-access-test-repo"
		repoOpts := api.CreateRepoOption{
			Name:     repoName,
			Private:  true,
			AutoInit: true,
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &repoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: org.ID, Name: repoName})

		// Add repository to team
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/repos/%s/%s", team.ID, orgName, repoName)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Login as team member and try to access the repo
		teamMemberSession := loginUser(t, teamMemberUsername)
		teamMemberToken := getTokenForLoggedInUser(t, teamMemberSession, auth_model.AccessTokenScopeWriteRepository)

		// KEY TEST: Team member tries to get repo info via API
		// On baseline (bug): 404 Not Found (team member denied access)
		// On golden (fix): 200 OK (team member has access via team)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", orgName, repoName)).AddTokenAuth(teamMemberToken)
		resp = teamMemberSession.MakeRequest(t, req, http.StatusOK)

		var repoInfo api.Repository
		DecodeJSON(t, resp, &repoInfo)
		assert.Equal(t, repo.Name, repoInfo.Name, "Team member should be able to read repo info")

		// Additional test: team member can write to the repo
		fileOpts := api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName: repo.DefaultBranch,
				Message:    "Test commit by team member",
			},
			ContentBase64: "dGVzdCBjb250ZW50", // "test content" base64
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/contents/test-team-access.txt", orgName, repoName),
			&fileOpts).AddTokenAuth(teamMemberToken)

		// On baseline (bug): 403 or 404 (access denied)
		// On golden (fix): 201 Created (access granted via team)
		teamMemberSession.MakeRequest(t, req, http.StatusCreated)
	})
}

// TestDirectCollaboratorStillWorks ensures that direct collaborator access
// still works after the team access fix (regression test).
func TestDirectCollaboratorStillWorks(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, orgOwner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		// Create user with direct access
		directUsername := "directuser"
		directUser := &user_model.User{
			Name:     directUsername,
			Email:    "direct@example.com",
			IsActive: true,
		}
		assert.NoError(t, directUser.SetPassword("password"))
		assert.NoError(t, user_model.CreateUser(context.Background(), directUser, nil,
			&user_model.CreateUserOverwriteOptions{
				IsActive: optional.Some(true),
			}))

		// Create organization and private repo
		orgName := "testorg-direct"
		orgOpts := api.CreateOrgOption{UserName: orgName}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &orgOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: orgName})

		repoName := "direct-access-repo"
		repoOpts := api.CreateRepoOption{
			Name:     repoName,
			Private:  true,
			AutoInit: true,
		}
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/repos", orgName), &repoOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: org.ID, Name: repoName})

		// Add direct collaborator with write access
		collabOpts := api.AddCollaboratorOption{Permission: ptrString("write")}
		req = NewRequestWithJSON(t, "PUT",
			fmt.Sprintf("/api/v1/repos/%s/%s/collaborators/%s", orgName, repoName, directUsername),
			&collabOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Login as direct collaborator
		directSession := loginUser(t, directUsername)
		directToken := getTokenForLoggedInUser(t, directSession, auth_model.AccessTokenScopeWriteRepository)

		// Direct collaborator should be able to access the repo
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s", orgName, repoName)).AddTokenAuth(directToken)
		resp := directSession.MakeRequest(t, req, http.StatusOK)

		var repoInfo api.Repository
		DecodeJSON(t, resp, &repoInfo)
		assert.Equal(t, repo.Name, repoInfo.Name, "Direct collaborator should be able to read repo info")
	})
}

// ptrString returns a pointer to the given string.
func ptrString(s string) *string {
	return &s
}
