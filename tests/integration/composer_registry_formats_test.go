// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/dsnet/compress/bzip2"
	"github.com/stretchr/testify/assert"
)

// createComposerTarGz creates a valid composer package in tar.gz format
func createComposerTarGz(name, version string) []byte {
	composerJSON := fmt.Sprintf(`{"name":"%s","version":"%s","type":"library"}`, name, version)

	// Create tar archive
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	// Add composer.json to tar
	hdr := &tar.Header{
		Name: "composer.json",
		Mode: 0o644,
		Size: int64(len(composerJSON)),
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte(composerJSON))
	tw.Close()

	// Compress with gzip
	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	gzWriter.Write(tarBuf.Bytes())
	gzWriter.Close()

	return gzBuf.Bytes()
}

// createComposerTarBz2 creates a valid composer package in tar.bz2 format
func createComposerTarBz2(name, version string) []byte {
	composerJSON := fmt.Sprintf(`{"name":"%s","version":"%s","type":"library"}`, name, version)

	// Create tar archive
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	// Add composer.json to tar
	hdr := &tar.Header{
		Name: "composer.json",
		Mode: 0o644,
		Size: int64(len(composerJSON)),
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte(composerJSON))
	tw.Close()

	// Compress with bzip2
	var bz2Buf bytes.Buffer
	bz2Writer, _ := bzip2.NewWriter(&bz2Buf, nil)
	bz2Writer.Write(tarBuf.Bytes())
	bz2Writer.Close()

	return bz2Buf.Bytes()
}

// TestComposerRegistryTarGzSupport tests that tar.gz packages can be uploaded
// to the Composer registry. On baseline this fails because only zip was supported.
// After the fix (PR #35958), tar.gz packages are accepted with 201 Created.
func TestComposerRegistryTarGzSupport(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	root := fmt.Sprintf("/api/packages/%s/composer", user.Name)

	t.Run("UploadTarGzPackage", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Create a valid tar.gz composer package with proper composer.json
		packageData := createComposerTarGz("test/targz-package", "1.0.0")

		// Upload via API
		// On baseline: returns error (not a valid zip file)
		// On golden: returns 201 Created (tar.gz now supported)
		req := NewRequestWithBody(t, "PUT", root, bytes.NewReader(packageData)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusCreated)

		assert.NotNil(t, resp)
	})
}

// TestComposerRegistryTarBz2Support tests that tar.bz2 packages can be uploaded
// to the Composer registry. On baseline this fails because only zip was supported.
// After the fix (PR #35958), tar.bz2 packages are accepted with 201 Created.
func TestComposerRegistryTarBz2Support(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	root := fmt.Sprintf("/api/packages/%s/composer", user.Name)

	t.Run("UploadTarBz2Package", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Create a valid tar.bz2 composer package with proper composer.json
		packageData := createComposerTarBz2("test/tarbz2-package", "2.0.0")

		// Upload via API
		// On baseline: returns error (not a valid zip file)
		// On golden: returns 201 Created (tar.bz2 now supported)
		req := NewRequestWithBody(t, "PUT", root, bytes.NewReader(packageData)).
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusCreated)

		assert.NotNil(t, resp)
	})
}
