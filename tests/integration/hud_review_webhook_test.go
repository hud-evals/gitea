// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPullCreateWithReviewers creates a PR with the specified reviewers.
// reviewerIDs is a comma-separated list of user IDs.
func testPullCreateWithReviewers(t *testing.T, session *TestSession, baseRepoOwner, baseRepoName, baseBranch, headRepoOwner, headRepoName, headBranch, title, reviewerIDs string) *httptest.ResponseRecorder {
	headCompare := headBranch
	if headRepoOwner != "" {
		if headRepoName != "" {
			headCompare = fmt.Sprintf("%s/%s:%s", headRepoOwner, headRepoName, headBranch)
		} else {
			headCompare = fmt.Sprintf("%s:%s", headRepoOwner, headBranch)
		}
	}
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/compare/%s...%s", baseRepoOwner, baseRepoName, baseBranch, headCompare))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Submit the form for creating the pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	params := map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"title": title,
	}
	if reviewerIDs != "" {
		params["reviewer_ids"] = reviewerIDs
	}
	req = NewRequestWithValues(t, "POST", link, params)
	resp = session.MakeRequest(t, req, http.StatusOK)
	return resp
}

// TestWebhookPullRequestWithReviewers tests that the pull_request webhook contains
// the correct requested_reviewers when a PR is created with reviewers.
//
// Bug: When creating a PR with reviewers via the web UI, the webhook was sent
// with an empty requested_reviewers array because the review request was processed
// AFTER the webhook notification was sent.
//
// Fix: The review request processing should happen BEFORE the webhook notification.
func TestWebhookPullRequestWithReviewers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		var payloads []api.PullRequestPayload
		var triggeredEvent string
		provider := newMockWebhookProvider(func(r *http.Request) {
			content, _ := io.ReadAll(r.Body)
			var payload api.PullRequestPayload
			err := json.Unmarshal(content, &payload)
			assert.NoError(t, err)
			payloads = append(payloads, payload)
			triggeredEvent = "pull_request"
		}, http.StatusOK)
		defer provider.Close()

		testCtx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeAll)
		// Add user4 as collaborator so that it can be a reviewer
		doAPIAddCollaborator(testCtx, "user4", perm.AccessModeWrite)(t)

		// Create webhook that only triggers on pull_request (not review_requested)
		// to keep the test deterministic
		sessionUser2 := loginUser(t, "user2")
		sessionUser4 := loginUser(t, "user4")

		testAPICreateWebhookForRepo(t, sessionUser2, "user2", "repo1", provider.URL(), "pull_request_only")

		testAPICreateBranch(t, sessionUser2, "user2", "repo1", "master", "reviewer-test-branch", http.StatusCreated)

		// Create PR with user4 as the creator and user2 as reviewer
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 1})
		testPullCreateWithReviewers(t, sessionUser4,
			repo1.OwnerName, repo1.Name, repo1.DefaultBranch,
			"", "", "reviewer-test-branch",
			"PR with reviewer",
			"2", // user2 as reviewer
		)

		// Validate the webhook is triggered and contains the reviewer
		assert.Equal(t, "pull_request", triggeredEvent)
		require.Len(t, payloads, 1)

		// This is the critical assertion:
		// The requested_reviewers should contain user2 (ID=2) at the time of the webhook
		// On baseline: this will be empty [] because review request happens after webhook
		// On golden: this will contain user2 because review request happens before webhook
		assert.Len(t, payloads[0].PullRequest.RequestedReviewers, 1,
			"Webhook payload should contain requested reviewers when PR is created with reviewers")
		if len(payloads[0].PullRequest.RequestedReviewers) > 0 {
			assert.Equal(t, int64(2), payloads[0].PullRequest.RequestedReviewers[0].ID,
				"Reviewer should be user2 (ID=2)")
		}
	})
}
