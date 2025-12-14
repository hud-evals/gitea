// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// TestWebhookURLValidation tests that webhook API endpoints properly validate URLs.
// The bug was that the API did not validate webhook URLs, allowing empty or invalid URLs
// to be saved, while the UI validates correctly.
func TestWebhookURLValidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateWebhookWithValidURL", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
			Type: "gitea",
			Config: api.CreateHookOptionConfig{
				"content_type": "json",
				"url":          "https://example.com/webhook",
			},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		var apiHook *api.Hook
		DecodeJSON(t, resp, &apiHook)
		assert.Equal(t, "https://example.com/webhook", apiHook.Config["url"])
	})

	t.Run("CreateWebhookWithInvalidURL", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Try to create webhook with invalid URL (missing scheme)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
			Type: "gitea",
			Config: api.CreateHookOptionConfig{
				"content_type": "json",
				"url":          "example.com/webhook", // Missing http:// or https://
			},
		}).AddTokenAuth(token)

		// Should fail with 422 Unprocessable Entity
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("CreateWebhookWithEmptyURL", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Try to create webhook with empty URL
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
			Type: "gitea",
			Config: api.CreateHookOptionConfig{
				"content_type": "json",
				"url":          "", // Empty URL
			},
		}).AddTokenAuth(token)

		// Should fail with 422 Unprocessable Entity
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("EditWebhookToInvalidURL", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// First create a valid webhook
		createReq := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
			Type: "gitea",
			Config: api.CreateHookOptionConfig{
				"content_type": "json",
				"url":          "https://valid-url.com/webhook",
			},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, createReq, http.StatusCreated)

		var apiHook *api.Hook
		DecodeJSON(t, resp, &apiHook)
		hookID := apiHook.ID

		// Try to edit it to an invalid URL
		editReq := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d", owner.Name, repo.Name, hookID), api.EditHookOption{
			Config: map[string]string{
				"url": "not-a-valid-url", // Invalid URL
			},
		}).AddTokenAuth(token)

		// Should fail with 422 Unprocessable Entity
		MakeRequest(t, editReq, http.StatusUnprocessableEntity)
	})
}
