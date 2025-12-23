// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	webhook_model "code.gitea.io/gitea/models/webhook"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

// TestTagPushDoesNotBypassBranchFilter verifies that tag push events do NOT
// trigger webhooks that have branch filters configured.
//
// Bug (before fix): Tag creation/deletion was triggering push webhooks even
// when branch filters were configured, causing unintended pipeline executions.
// The filter logic only checked branch names, so tags returned empty string
// and bypassed the filter entirely (empty string always matched the filter).
//
// Fix: The getPayloadBranch() function was renamed to getPayloadRef() and now
// returns full ref names for both branches AND tags. The filter now properly
// checks if a tag ref matches the branch filter pattern, correctly rejecting
// tags when only branch patterns are specified.
//
// This is a regression test for issue #35449 / PR #35567.
func TestTagPushDoesNotBypassBranchFilter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Use repo 2 which has webhook 4 with:
	// - push_only: true
	// - branch_filter: "{master,feature*}"
	// (see models/fixtures/webhook.yml)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	// Create a push payload for a TAG - this should NOT match the branch filter
	// "{master,feature*}" because it's a tag ref, not a branch ref
	tagPushPayload := &api.PushPayload{
		Ref:     "refs/tags/v1.0.0",
		Commits: []*api.PayloadCommit{{}},
	}

	hookTask := &webhook_model.HookTask{HookID: 4, EventType: webhook_module.HookEventPush}
	unittest.AssertNotExistsBean(t, hookTask)

	// PrepareWebhooks with a tag push event
	assert.NoError(t, PrepareWebhooks(t.Context(), EventSource{Repository: repo}, webhook_module.HookEventPush, tagPushPayload))

	// On BASELINE (bug): Hook task WOULD be created because tag push returned
	// empty branch name, which bypassed the filter check entirely.
	// On GOLDEN (fix): Hook task should NOT be created because tag refs don't
	// match branch filter patterns like "master" or "feature*".
	// Tag push to 'refs/tags/v1.0.0' should NOT trigger webhook with branch filter
	// '{master,feature*}' - tags must respect branch filters and not bypass them.
	unittest.AssertNotExistsBean(t, hookTask)
}

