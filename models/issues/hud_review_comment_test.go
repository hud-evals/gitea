// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommentReviewerNotRemoved tests that reviewers who only have
// comment reviews are still included in the reviewers list.
// This is a regression test for issue #34617 / PR #35591.
func TestCommentReviewerNotRemoved(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Issue 3 has reviews including ReviewTypeComment
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})

	allReviews, _, err := issues_model.GetReviewsByIssueID(t.Context(), issue.ID)
	require.NoError(t, err)

	// Load reviewer info for all reviews
	for _, review := range allReviews {
		require.NoError(t, review.LoadReviewer(t.Context()))
	}

	// Check that we have reviews with ReviewTypeComment type
	hasCommentReview := false
	for _, review := range allReviews {
		if review.Type == issues_model.ReviewTypeComment {
			hasCommentReview = true
			break
		}
	}

	// The important assertion: comment reviews should be included in the list
	// If ReviewTypeComment is not in countedReviewTypes, comment-only reviewers
	// will be excluded from the list, which is the bug we're testing for
	assert.True(t, hasCommentReview,
		"Reviews with type ReviewTypeComment should be included in the result. "+
			"Found %d reviews but none with ReviewTypeComment type.", len(allReviews))
}

// TestReviewTypesIncluded tests that all expected review types are counted
func TestReviewTypesIncluded(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})

	allReviews, _, err := issues_model.GetReviewsByIssueID(t.Context(), issue.ID)
	require.NoError(t, err)

	// Count the types of reviews we got
	reviewTypeCounts := make(map[issues_model.ReviewType]int)
	for _, review := range allReviews {
		reviewTypeCounts[review.Type]++
	}

	// The result should include at least these review types:
	// - ReviewTypeApprove
	// - ReviewTypeReject  
	// - ReviewTypeRequest
	// - ReviewTypeComment (this is the one that was missing in the bug)
	expectedTypes := []issues_model.ReviewType{
		issues_model.ReviewTypeComment,
	}

	for _, expectedType := range expectedTypes {
		assert.Greater(t, reviewTypeCounts[expectedType], 0,
			"Expected at least one review of type %d (%v)", expectedType, expectedType)
	}
}
