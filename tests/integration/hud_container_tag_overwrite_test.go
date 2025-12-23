// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

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

	// Unique image name for this test
	image := "hud-tag-overwrite-test"
	tag := "latest"

	// Build the container API URL
	url := fmt.Sprintf("/v2/%s/%s", user.Name, image)

	// First, we need to upload blobs (required for manifest)
	// This is a valid gzip-compressed layer that matches the digest
	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)

	// Upload layer blob
	req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), bytes.NewReader(blobContent)).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)
	assert.NotEmpty(t, resp.Header().Get("Docker-Content-Digest"))

	// Upload config blob (required for manifest)
	configDigest := "sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d"
	configContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`
	req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, configDigest), strings.NewReader(configContent)).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusCreated)

	// Create a simple manifest referencing the blobs
	// This digest matches the manifest content with oci.MediaTypeImageManifest
	manifestDigest := "sha256:4305f5f5572b9a426b88909b036e52ee3cf3d7b9c1b01fac840e90747f56623d"
	manifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageManifest + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4","size":32}]}`

	// Upload manifest first (untagged, needed for index)
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, manifestDigest), strings.NewReader(manifestContent)).
		AddBasicAuth(user.Name).
		SetHeader("Content-Type", oci.MediaTypeImageManifest)
	MakeRequest(t, req, http.StatusCreated)

	// Create an image INDEX manifest (multi-arch manifest)
	indexManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageIndex + `","manifests":[{"mediaType":"` + oci.MediaTypeImageManifest + `","digest":"` + manifestDigest + `","platform":{"os":"linux","architecture":"amd64"}}]}`

	// Push the index manifest with a tag
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(indexManifestContent)).
		AddBasicAuth(user.Name).
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
		AddBasicAuth(user.Name).
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

	image := "hud-download-count-test"
	tag := "v1"
	url := fmt.Sprintf("/v2/%s/%s", user.Name, image)

	// Upload blobs (required for manifest)
	// This is a valid gzip-compressed layer that matches the digest
	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)
	req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), bytes.NewReader(blobContent)).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusCreated)

	// Upload config blob
	configDigest := "sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d"
	configContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`
	req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, configDigest), strings.NewReader(configContent)).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusCreated)

	// Upload manifest with tag
	manifestContent := `{"schemaVersion":2,"mediaType":"` + container_module.ContentTypeDockerDistributionManifestV2 + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"` + configDigest + `","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"` + blobDigest + `","size":32}]}`

	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
		AddBasicAuth(user.Name).
		SetHeader("Content-Type", container_module.ContentTypeDockerDistributionManifestV2)
	MakeRequest(t, req, http.StatusCreated)

	// Simulate a download by getting the manifest
	req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", url, tag)).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusOK)

	// Get download count
	pv1, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)
	initialDownloadCount := pv1.DownloadCount

	// REGRESSION: Overwrite should preserve download count
	req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
		AddBasicAuth(user.Name).
		SetHeader("Content-Type", container_module.ContentTypeDockerDistributionManifestV2)
	MakeRequest(t, req, http.StatusCreated)

	pv2, err := packages_model.GetVersionByNameAndVersion(t.Context(), user.ID, packages_model.TypeContainer, image, tag)
	require.NoError(t, err)

	assert.Equal(t, initialDownloadCount, pv2.DownloadCount,
		"Download count should be preserved when overwriting a manifest tag")
}

