// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscordPayloadEmptyUsername tests that when Username is empty,
// it should be omitted from the JSON payload entirely.
// Discord API returns 400 error when username is an empty string.
// This is a regression test for issue #35411 / PR #35412.
func TestDiscordPayloadEmptyUsername(t *testing.T) {
	t.Run("EmptyUsernameShouldBeOmitted", func(t *testing.T) {
		payload := DiscordPayload{
			Content:  "Test message",
			Username: "", // Empty username
			Embeds:   []DiscordEmbed{},
		}

		data, err := json.Marshal(payload)
		require.NoError(t, err)

		jsonStr := string(data)

		// The username field should NOT appear in the JSON when it's empty
		// If it appears as "username":"", Discord will reject the webhook
		assert.False(t, strings.Contains(jsonStr, `"username":"`),
			"Empty username should be omitted from JSON, but got: %s", jsonStr)
	})

	t.Run("NonEmptyUsernameShouldBeIncluded", func(t *testing.T) {
		payload := DiscordPayload{
			Content:  "Test message",
			Username: "TestBot",
			Embeds:   []DiscordEmbed{},
		}

		data, err := json.Marshal(payload)
		require.NoError(t, err)

		jsonStr := string(data)

		// Non-empty username should be included
		assert.True(t, strings.Contains(jsonStr, `"username":"TestBot"`),
			"Non-empty username should be included in JSON, but got: %s", jsonStr)
	})

	t.Run("EmptyAvatarURLShouldBeOmitted", func(t *testing.T) {
		// Also verify that avatar_url behaves correctly (should also be omitempty)
		payload := DiscordPayload{
			Content:   "Test message",
			Username:  "TestBot",
			AvatarURL: "", // Empty avatar URL
			Embeds:    []DiscordEmbed{},
		}

		data, err := json.Marshal(payload)
		require.NoError(t, err)

		jsonStr := string(data)

		// The avatar_url field should NOT appear in the JSON when it's empty
		assert.False(t, strings.Contains(jsonStr, `"avatar_url":"`),
			"Empty avatar_url should be omitted from JSON, but got: %s", jsonStr)
	})
}
