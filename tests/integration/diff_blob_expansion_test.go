// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	"code.gitea.io/gitea/services/gitdiff"
	
	"github.com/stretchr/testify/assert"
)

func TestDiffBlobExpansion(t *testing.T) {
	t.Run("ExpandDirectionCalculation", func(t *testing.T) {
		testCases := []struct {
			name          string
			lastRightIdx  int
			lastLeftIdx   int
			rightIdx      int  
			leftIdx       int
			rightHunkSize int
			leftHunkSize  int
			expected      string
		}{
			{"NoHiddenLines", 100, 100, 101, 101, 0, 0, ""},
			{"FileHead", 0, 0, 101, 101, 0, 0, "up"},
			{"SingleHunk", 100, 100, 102, 102, 10, 10, "single"},
			{"UpDownHunks", 100, 100, 100 + gitdiff.BlobExcerptChunkSize + 2, 100 + gitdiff.BlobExcerptChunkSize + 2, 10, 10, "updown"},
			{"FileTail", 100, 100, 102, 102, 0, 0, "down"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				line := &gitdiff.DiffLine{
					Type: gitdiff.DiffLineSection,
					SectionInfo: &gitdiff.DiffLineSectionInfo{
						LastRightIdx:  tc.lastRightIdx,
						LastLeftIdx:   tc.lastLeftIdx,
						RightIdx:      tc.rightIdx,
						LeftIdx:       tc.leftIdx,
						RightHunkSize: tc.rightHunkSize,
						LeftHunkSize:  tc.leftHunkSize,
					},
				}
				result := line.GetExpandDirection()
				assert.Equal(t, tc.expected, result, "Test case: %s", tc.name)
			})
		}
	})
}
