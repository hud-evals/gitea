// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/dsnet/compress/bzip2"
	"github.com/stretchr/testify/assert"
)

// condaInfo represents the info section of repodata.json
type condaInfo struct {
	Subdir string `json:"subdir"`
}

// condaPackageInfo represents package info in repodata.json
type condaPackageInfo struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	NoArch        string   `json:"noarch"`
	Subdir        string   `json:"subdir"`
	Timestamp     int64    `json:"timestamp"`
	Build         string   `json:"build"`
	BuildNumber   int64    `json:"build_number"`
	Dependencies  []string `json:"depends"`
	License       string   `json:"license"`
	LicenseFamily string   `json:"license_family"`
	HashMD5       string   `json:"md5"`
	HashSHA256    string   `json:"sha256"`
	Size          int64    `json:"size"`
}

// condaRepoData represents the structure of repodata.json
type condaRepoData struct {
	Info          condaInfo                    `json:"info"`
	Packages      map[string]*condaPackageInfo `json:"packages"`
	PackagesConda map[string]*condaPackageInfo `json:"packages.conda"`
	Removed       map[string]*condaPackageInfo `json:"removed"`
}

// TestCondaNullDependencies tests that packages with no dependencies
// return an empty array [] instead of null in the repodata.json response.
// This is important for conda client compatibility - when Dependencies is nil,
// the JSON serialization produces "null" instead of "[]", which causes
// the conda client to fail when searching packages.
func TestCondaNullDependencies(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Use unique package name to avoid conflicts with other tests
	packageName := "nodeps_package"
	packageVersion := "2.0.0"

	root := fmt.Sprintf("/api/packages/%s/conda", user.Name)

	// Create a package with NO dependencies in the index.json
	// Note: the absence of "depends" field means empty dependencies
	tarContent := func() []byte {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		// index.json without any dependencies field
		content := []byte(`{"name":"` + packageName + `","version":"` + packageVersion + `","subdir":"noarch","build":"nodeps"}`)

		hdr := &tar.Header{
			Name: "info/index.json",
			Mode: 0o600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write(content)
		tw.Close()
		return buf.Bytes()
	}()

	// Upload as .tar.bz2
	t.Run("UploadPackageWithNoDeps", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		var buf bytes.Buffer
		bw, _ := bzip2.NewWriter(&buf, nil)
		io.Copy(bw, bytes.NewReader(tarContent))
		bw.Close()

		filename := fmt.Sprintf("%s-%s-nodeps.tar.bz2", packageName, packageVersion)

		req := NewRequestWithBody(t, "PUT", root+"/"+filename, bytes.NewReader(buf.Bytes())).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	// Verify repodata.json returns Dependencies as empty array, not null
	t.Run("VerifyDependenciesNotNull", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", root+"/noarch/repodata.json")
		resp := MakeRequest(t, req, http.StatusOK)

		var result condaRepoData
		DecodeJSON(t, resp, &result)

		filename := fmt.Sprintf("%s-%s-nodeps.tar.bz2", packageName, packageVersion)
		assert.Contains(t, result.Packages, filename, "Package should exist in repodata")

		packageInfo := result.Packages[filename]
		assert.Equal(t, packageName, packageInfo.Name)
		assert.Equal(t, packageVersion, packageInfo.Version)

		// Critical assertions: Dependencies must be non-nil (empty array, not null)
		// On baseline this will fail because Dependencies is nil
		// On golden this will pass because util.SliceNilAsEmpty ensures []
		assert.NotNil(t, packageInfo.Dependencies, "Dependencies should not be nil - it should be an empty array")
		assert.Empty(t, packageInfo.Dependencies, "Dependencies should be empty for a package with no deps")
	})
}
