// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/tests"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainerIndexManifestTagOverwrite verifies that pushing an image index
// manifest with the same tag properly replaces the existing version.
//
// Bug (before fix): When pushing a container image index manifest with the same
// tag, the system would sometimes fail to properly overwrite the existing version.
// The check for "24 hour old" images prevented proper overwrites for recently
// created tags, and the media type check was too restrictive.
//
// Fix: Remove the 24-hour age check and always properly delete and recreate
// the package version when overwriting a tag with an image index manifest.
//
// This is a regression test for issue #35853 / PR #35936.
func TestContainerIndexManifestTagOverwrite(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWritePackage)

	// Unique image name for this test
	image := "hud-tag-overwrite-test"
	tag := "latest"

	// Build the container API URL
	url := fmt.Sprintf("/v2/%s/%s", user.Name, image)

	// First, we need to upload a blob (required for manifest)
	blobContent := "test blob content for layer"
	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"

	// Upload blob
	req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), strings.NewReader(blobContent)).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	assert.NotEmpty(t, resp.Header().Get("Docker-Content-Digest"))

	// Create a simple manifest referencing the blob
	manifestDigest := "sha256:4f10484d1c1bb13e3956b4de1cd42db8e0f14a75be1617b60f2de3cd59c803c6"
	manifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageManifest + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}]}`

	// Upload manifest first (untagged, needed for index)
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, manifestDigest), strings.NewReader(manifestContent)).
		AddTokenAuth(token).
		SetHeader("Content-Type", oci.MediaTypeImageManifest)
	MakeRequest(t, req, http.StatusCreated)

	// Create an image INDEX manifest (multi-arch manifest)
	indexManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageIndex + `","manifests":[{"mediaType":"` + oci.MediaTypeImageManifest + `","digest":"` + manifestDigest + `","platform":{"os":"linux","architecture":"amd64"}}]}`

	// Push the index manifest with a tag
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(indexManifestContent)).
		AddTokenAuth(token).
		SetHeader("Content-Type", oci.MediaTypeImageIndex)
	MakeRequest(t, req, http.StatusCreated)

	// Get the initial package version
	pv1, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)
	require.NotNil(t, pv1)
	initialVersionID := pv1.ID

	// Push the SAME index manifest with the same tag again
	// This simulates "docker buildx imagetools create" operations
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(indexManifestContent)).
		AddTokenAuth(token).
		SetHeader("Content-Type", oci.MediaTypeImageIndex)
	MakeRequest(t, req, http.StatusCreated)

	// Get the package version after overwrite
	pv2, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)
	require.NotNil(t, pv2)

	// The version ID MUST be different after overwriting
	// On BASELINE (bug): Version ID might be the same (not properly overwritten)
	// On GOLDEN (fix): Version ID is different (old version deleted, new one created)
	assert.NotEqual(t, initialVersionID, pv2.ID,
		"Package version ID should change when overwriting a tag with an image index manifest. "+
			"If they're equal, the old version was not properly deleted and recreated.")
}

// TestContainerManifestOverwriteKeepsDownloadCount is a REGRESSION test to ensure
// that when overwriting a manifest tag, the download count is preserved.
func TestContainerManifestOverwriteKeepsDownloadCount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWritePackage)

	image := "hud-download-count-test"
	tag := "v1"
	url := fmt.Sprintf("/v2/%s/%s", user.Name, image)

	// Upload a blob
	blobContent := "test blob"
	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), strings.NewReader(blobContent)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Upload manifest with tag
	manifestContent := `{"schemaVersion":2,"mediaType":"` + container_module.ContentTypeDockerDistributionManifestV2 + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}]}`

	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
		AddTokenAuth(token).
		SetHeader("Content-Type", container_module.ContentTypeDockerDistributionManifestV2)
	MakeRequest(t, req, http.StatusCreated)

	// Simulate a download by getting the manifest
	req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", url, tag)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Get download count
	pv1, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)
	initialDownloadCount := pv1.DownloadCount

	// REGRESSION: Overwrite should preserve download count
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
		AddTokenAuth(token).
		SetHeader("Content-Type", container_module.ContentTypeDockerDistributionManifestV2)
	MakeRequest(t, req, http.StatusCreated)

	pv2, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)

	assert.Equal(t, initialDownloadCount, pv2.DownloadCount,
		"Download count should be preserved when overwriting a manifest tag")
}

