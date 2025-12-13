// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTeamMemberInBranchProtectionWhitelist tests that team members appear in
// branch protection user whitelists when they have access via team membership.
//
// Bug (before fix): Users with team-based access to a repo didn't show up in
// the branch protection user dropdown/search. When retrieving branch protection
// via API, team members' usernames were missing from PushWhitelistUsernames
// even if their IDs were in the whitelist.
//
// This was because GetRepoReaders() only looked at the 'access' table for direct
// collaborators and didn't include team members for organization-owned repos.
//
// Fix: GetUsersWithUnitAccess() now includes team members by calling
// GetTeamUserIDsWithAccessToAnyRepoUnit() for organization-owned repositories.
//
// Related issue: https://github.com/go-gitea/gitea/issues/35499
func TestTeamMemberInBranchProtectionWhitelist(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Create organization owner (user 2 - has valid password hash)
		orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, orgOwner.Name)
		token := getTokenForLoggedInUser(t, session,
			auth_model.AccessTokenScopeWriteOrganization,
			auth_model.AccessTokenScopeWriteRepository)

		// Create a test user who will be added to team
		// Copy from user2 (which has a valid password hash for "password")
		teamMemberUsername := "teammember_bp"
		teamMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		teamMember.Name = teamMemberUsername
		teamMember.LowerName = strings.ToLower(teamMemberUsername)
		teamMember.Email = "teammember_bp@example.com"
		teamMember.ID = 0
		require.NoError(t, db.Insert(t.Context(), teamMember))

		// Reload to get the assigned ID
		teamMember = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: strings.ToLower(teamMemberUsername)})

		// Create organization
		orgName := "testorg-bp-whitelist"
		orgOpts := api.CreateOrgOption{
			UserName: orgName,
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &orgOpts).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: orgName})

		// Create team with write permissions (needed to be in push whitelist)
		teamName := "writers"
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
		repoName := "bp-whitelist-repo"
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

		// Create branch protection rule with the team member in push whitelist
		// We use the team member's username in push_whitelist_usernames
		bpOpts := api.BranchProtection{
			RuleName:              repo.DefaultBranch,
			EnablePush:            true,
			EnablePushWhitelist:   true,
			PushWhitelistUsernames: []string{teamMemberUsername},
		}
		req = NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/branch_protections", orgName, repoName),
			&bpOpts).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusCreated)

		var createdBP api.BranchProtection
		DecodeJSON(t, resp, &createdBP)

		// Now GET the branch protection and check if the team member appears
		// in PushWhitelistUsernames
		req = NewRequest(t, "GET",
			fmt.Sprintf("/api/v1/repos/%s/%s/branch_protections/%s", orgName, repoName, repo.DefaultBranch)).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)

		var fetchedBP api.BranchProtection
		DecodeJSON(t, resp, &fetchedBP)

		// KEY ASSERTION:
		// On baseline (bug): PushWhitelistUsernames will be empty or not contain
		// the team member, because GetRepoReaders() didn't include team members
		// On golden (fix): PushWhitelistUsernames will contain the team member
		assert.Contains(t, fetchedBP.PushWhitelistUsernames, teamMemberUsername,
			"Team member should appear in PushWhitelistUsernames. "+
				"Bug: GetRepoReaders() didn't include team members for org repos. "+
				"If this fails, the team member was added to whitelist but their "+
				"username doesn't appear in the API response because they're not "+
				"found in the 'readers' list used by ToBranchProtection().")
	})
}
