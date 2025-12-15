// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fixUnfinishedRunStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Find the check by its registered name (behavioral test - doesn't require specific function name)
	var targetCheck *Check
	for _, check := range Checks {
		if check.Name == "fix-actions-unfinished-run-status" {
			targetCheck = check
			break
		}
	}
	require.NotNil(t, targetCheck, "Doctor check 'fix-actions-unfinished-run-status' should be registered")

	// Run the check with autofix enabled
	err := targetCheck.Run(t.Context(), log.GetLogger(log.DEFAULT), true)
	assert.NoError(t, err)

	// Check if the run status was fixed
	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 805})
	assert.Equal(t, actions_model.StatusCancelled, run.Status)
}
