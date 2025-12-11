// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	testModule "code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

// TestSpecialTagsEscaping verifies that dangerous HTML tags like <script> and <style>
// are properly escaped during markdown post-processing to match GitHub's behavior.
// See: https://github.com/go-gitea/gitea/pull/34129
func TestSpecialTagsEscaping(t *testing.T) {
	setting.StaticURLPrefix = markup.TestAppURL
	defer testModule.MockVariableValue(&markup.RenderBehaviorForTesting.DisableAdditionalAttributes, true)()

	test := func(input, expected string) {
		var res strings.Builder
		err := markup.PostProcessDefault(markup.NewTestRenderContext(markup.TestAppURL, map[string]string{"user": "test-user", "repo": "test-repo"}), strings.NewReader(input), &res)
		assert.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(res.String()))
	}

	// Script tag tests - these should be escaped to prevent XSS
	t.Run("ScriptTagUnclosed", func(t *testing.T) {
		test("<script>alert('xss')", `&lt;script&gt;alert('xss')`)
	})

	t.Run("ScriptTagClosed", func(t *testing.T) {
		test("<script>a</script>", `&lt;script&gt;a&lt;/script&gt;`)
	})

	t.Run("ScriptTagUppercase", func(t *testing.T) {
		test("<SCRIPT>a</SCRIPT>", `&lt;SCRIPT&gt;a&lt;/SCRIPT&gt;`)
	})

	t.Run("ScriptTagMixedCase", func(t *testing.T) {
		test("<ScRiPt>a</sCrIpT>", `&lt;ScRiPt&gt;a&lt;/sCrIpT&gt;`)
	})

	// Style tag tests - these should be escaped to prevent CSS injection
	t.Run("StyleTagUnclosed", func(t *testing.T) {
		test("<style>.evil{}", `&lt;style&gt;.evil{}`)
	})

	t.Run("StyleTagClosed", func(t *testing.T) {
		test("<style>a</style>", `&lt;style&gt;a&lt;/style&gt;`)
	})

	t.Run("StyleTagUppercase", func(t *testing.T) {
		test("<STYLE>a</STYLE>", `&lt;STYLE&gt;a&lt;/STYLE&gt;`)
	})

	t.Run("StyleTagMixedCase", func(t *testing.T) {
		test("<Style>a</STYLE>", `&lt;Style&gt;a&lt;/STYLE&gt;`)
	})

	// HTML and HEAD tags - these were already handled, verify they still work
	t.Run("HtmlTag", func(t *testing.T) {
		test("<html>content", `&lt;html&gt;content`)
	})

	t.Run("HeadTag", func(t *testing.T) {
		test("<head>content", `&lt;head&gt;content`)
	})

	// Multiple dangerous tags in one input
	t.Run("MultipleDangerousTags", func(t *testing.T) {
		test("<script>a</script><style>b</style>", `&lt;script&gt;a&lt;/script&gt;&lt;style&gt;b&lt;/style&gt;`)
	})

	// Verify safe tags are NOT escaped
	t.Run("SafeTagsNotEscaped", func(t *testing.T) {
		// Body, div, span, etc. should pass through
		var res strings.Builder
		input := "<div>content</div>"
		err := markup.PostProcessDefault(markup.NewTestRenderContext(markup.TestAppURL, map[string]string{"user": "test-user", "repo": "test-repo"}), strings.NewReader(input), &res)
		assert.NoError(t, err)
		// div should remain as HTML, not escaped
		assert.Contains(t, res.String(), "content")
		assert.NotContains(t, res.String(), "&lt;div&gt;")
	})
}

