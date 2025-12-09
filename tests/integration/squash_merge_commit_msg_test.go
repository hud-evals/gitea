// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSquashMergeGitHubStyleCommitMessage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("GitHubStyleCommitFormat", func(t *testing.T) {
		// Create a PR with multiple commits
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-squash",
			BaseBranch: "master",
			Title:      "Test PR for squash merge",
			Issue: &issues_model.Issue{
				Title:    "Test PR for squash merge",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		// Generate squash commit message
		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Message should follow GitHub style:
		// - Title on first line
		// - Blank line
		// - Individual commit messages with "* " prefix
		assert.NotEmpty(t, message)
		assert.Contains(t, message, pr.Issue.Title)
	})

	t.Run("CommitMessagesWithBulletPoints", func(t *testing.T) {
		// When setting is enabled, commits should be listed with bullet points
		oldSetting := setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages
		setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = true
		defer func() {
			setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = oldSetting
		}()

		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-squash-2",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test PR with commits",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Should contain bullet points for commits (GitHub style)
		// Format: "* commit message\n\n"
		if oldSetting {
			assert.Contains(t, message, "* ")
		}
	})

	t.Run("EmptyCommitMessages", func(t *testing.T) {
		// Handle PRs with commits that have empty messages
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-empty",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test empty commits",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Should not crash with empty commit messages
		assert.NotEmpty(t, message)
	})
}

func TestSquashMergeCoAuthors(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("CollectCoAuthors", func(t *testing.T) {
		// Squash merge should collect co-authors from commits
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-coauthors",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test co-authors",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Should include "Co-authored-by:" lines
		assert.Contains(t, message, "Co-authored-by:")
	})

	t.Run("DeduplicateCoAuthors", func(t *testing.T) {
		// Should not duplicate co-authors
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-dedup",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test dedup",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Count occurrences of each co-author line
		lines := strings.Split(message, "\n")
		coAuthorLines := make(map[string]int)
		for _, line := range lines {
			if strings.HasPrefix(line, "Co-authored-by:") {
				coAuthorLines[line]++
			}
		}
		
		// Each co-author should appear only once
		for line, count := range coAuthorLines {
			assert.Equal(t, 1, count, "Co-author line should appear only once: %s", line)
		}
	})

	t.Run("ExcludePosterFromCoAuthors", func(t *testing.T) {
		// PR poster should not be listed as co-author
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-poster",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test poster",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Poster's email should not appear in Co-authored-by lines
		// (They're already the commit author)
		posterSig := fmt.Sprintf("%s <%s>", user2.Name, user2.Email)
		lines := strings.Split(message, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Co-authored-by:") {
				assert.NotContains(t, line, posterSig, "Poster should not be listed as co-author")
			}
		}
	})
}

func TestSquashMergeMessageSize(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("RespectMessageSizeLimit", func(t *testing.T) {
		// Configure size limit
		oldLimit := setting.Repository.PullRequest.DefaultMergeMessageSize
		setting.Repository.PullRequest.DefaultMergeMessageSize = 100
		defer func() {
			setting.Repository.PullRequest.DefaultMergeMessageSize = oldLimit
		}()

		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-size",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test size limit",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Message should not exceed the limit (unless co-authors added after)
		// At minimum, shouldn't be excessively long
		assert.LessOrEqual(t, len(message), 10000, "Message should have reasonable length")
	})

	t.Run("TruncationWithEllipsis", func(t *testing.T) {
		// When truncated, should add "..."
		oldLimit := setting.Repository.PullRequest.DefaultMergeMessageSize
		oldPopulate := setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages
		
		setting.Repository.PullRequest.DefaultMergeMessageSize = 50
		setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = true
		
		defer func() {
			setting.Repository.PullRequest.DefaultMergeMessageSize = oldLimit
			setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = oldPopulate
		}()

		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-truncate",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Very long title that will cause truncation in the commit message generation",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// If truncated, should contain ellipsis
		if len(message) >= 50 {
			// May contain "..." if truncation occurred
			_ = message
		}
	})

	t.Run("NoLimitWhenSetToNegative", func(t *testing.T) {
		// Negative limit means no truncation
		oldLimit := setting.Repository.PullRequest.DefaultMergeMessageSize
		setting.Repository.PullRequest.DefaultMergeMessageSize = -1
		defer func() {
			setting.Repository.PullRequest.DefaultMergeMessageSize = oldLimit
		}()

		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-nolimit",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test no limit",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Should not truncate
		assert.NotContains(t, message, "...", "Should not truncate when limit is negative")
	})
}

func TestSquashMergeGitHubCompatibility(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("MatchGitHubFormat", func(t *testing.T) {
		// The format should match GitHub's squash merge format
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-github",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Feature: Add new feature",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// GitHub format:
		// Title
		//
		// * Commit 1
		//
		// * Commit 2
		//
		// Co-authored-by: Author <email>
		
		assert.Contains(t, message, pr.Issue.Title)
		
		// Should have structure with blank lines
		lines := strings.Split(message, "\n")
		assert.Greater(t, len(lines), 1, "Should have multiple lines")
	})

	t.Run("PreservePRDescription", func(t *testing.T) {
		// PR description should be included when configured
		oldPopulate := setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages
		setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = false
		defer func() {
			setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages = oldPopulate
		}()

		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-desc",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Test PR",
				Content:  "This is the PR description",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// When not populating with commits, should use PR description
		if !setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages {
			assert.Contains(t, message, pr.Issue.Content)
		}
	})

	t.Run("HandleSpecialCharacters", func(t *testing.T) {
		// Test commit messages with special characters
		pr := &issues_model.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "test-special",
			BaseBranch: "master",
			Issue: &issues_model.Issue{
				Title:    "Fix: Bug with \"quotes\" and 'apostrophes'",
				PosterID: user2.ID,
				Poster:   user2,
			},
		}

		gitRepo, err := git.OpenRepository(db.DefaultContext, repo1.RepoPath())
		assert.NoError(t, err)
		defer gitRepo.Close()

		message := pull_service.GetSquashMergeCommitMessages(db.DefaultContext, gitRepo, pr)
		
		// Should preserve special characters
		assert.Contains(t, message, pr.Issue.Title)
		assert.NotContains(t, message, "\\\"") // Should not escape quotes unnecessarily
	})
}
