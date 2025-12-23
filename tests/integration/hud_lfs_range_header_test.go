// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLFSRangeContentRangeHeader tests that the Content-Range header
// returns the correct total file size according to RFC 7233.
//
// Bug (before fix): The Content-Range header was using (total - fromByte)
// as the complete-length instead of the actual total file size.
//
// Example: 56-byte file with Range: bytes=10-
//   - Wrong:   Content-Range: bytes 10-55/46  (46 = 56 - 10)
//   - Correct: Content-Range: bytes 10-55/56  (56 = actual size)
//
// RFC 7233 §4.2: Content-Range = "bytes" first-byte-pos "-" last-byte-pos "/" complete-length
//
// This is a regression test for issue #35276 / PR #35277.
func TestLFSRangeContentRangeHeader(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Create test content of known size
	contentSize := 56
	content := make([]byte, contentSize)
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}

	// Store the content in LFS
	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	require.NoError(t, err)

	pointer, err := lfs.GeneratePointer(bytes.NewReader(content))
	require.NoError(t, err)

	_, err = git_model.NewLFSMetaObject(t.Context(), repo.ID, pointer)
	require.NoError(t, err)
	defer git_model.RemoveLFSMetaObjectByOid(t.Context(), repo.ID, pointer.Oid)

	contentStore := lfs.NewContentStore()
	exist, err := contentStore.Exists(pointer)
	require.NoError(t, err)
	if !exist {
		err = contentStore.Put(pointer, bytes.NewReader(content))
		require.NoError(t, err)
	}

	session := loginUser(t, "user2")

	testCases := []struct {
		name           string
		rangeHeader    string
		expectedFrom   int
		expectedTo     int
		expectedTotal  int
		expectedStatus int
	}{
		{
			name:           "RangeFromMiddle",
			rangeHeader:    "bytes=10-",
			expectedFrom:   10,
			expectedTo:     contentSize - 1,
			expectedTotal:  contentSize, // Must be total file size, not remaining
			expectedStatus: http.StatusPartialContent,
		},
		{
			name:           "RangeFromStart",
			rangeHeader:    "bytes=0-9",
			expectedFrom:   0,
			expectedTo:     9,
			expectedTotal:  contentSize,
			expectedStatus: http.StatusPartialContent,
		},
		{
			name:           "RangeFromEnd",
			rangeHeader:    "bytes=50-",
			expectedFrom:   50,
			expectedTo:     contentSize - 1,
			expectedTotal:  contentSize,
			expectedStatus: http.StatusPartialContent,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1.git/info/lfs/objects/"+pointer.Oid+"/test")
			req.Header.Set("Range", tc.rangeHeader)

			resp := session.MakeRequest(t, req, tc.expectedStatus)

			// Parse the Content-Range header
			contentRange := resp.Header().Get("Content-Range")
			require.NotEmpty(t, contentRange, "Content-Range header should be present")

			// Expected format: "bytes <from>-<to>/<total>"
			expectedContentRange := fmt.Sprintf("bytes %d-%d/%d",
				tc.expectedFrom, tc.expectedTo, tc.expectedTotal)

			assert.Equal(t, expectedContentRange, contentRange,
				"Content-Range header should use the complete file size as total, not the remaining bytes. "+
					"RFC 7233 §4.2 specifies: Content-Range = bytes first-byte-pos \"-\" last-byte-pos \"/\" complete-length")

			// Also verify by parsing the total from the header
			parts := strings.Split(contentRange, "/")
			if len(parts) == 2 {
				total, err := strconv.Atoi(parts[1])
				if err == nil {
					assert.Equal(t, contentSize, total,
						"The total in Content-Range should equal the actual file size (%d), got %d",
						contentSize, total)
				}
			}
		})
	}
}
