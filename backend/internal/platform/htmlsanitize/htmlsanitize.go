// Package htmlsanitize strips dangerous markup (script tags, event handlers,
// javascript: URLs, etc.) from rich-text bodies before they are persisted, so
// content that later renders as raw HTML on the public portal (announcements,
// knowledgebase articles) can't carry a stored XSS payload.
package htmlsanitize

import "github.com/microcosm-cc/bluemonday"

// policy is a UGC allow-list: common formatting tags/attributes survive,
// scripts/event handlers/forms/iframes do not. Built once and reused — the
// bluemonday policy is safe for concurrent use.
var policy = bluemonday.UGCPolicy()

// HTML sanitizes s for safe use as trusted HTML (e.g. behind Svelte's
// `{@html}`), removing any tag/attribute not on the allow-list.
func HTML(s string) string {
	return policy.Sanitize(s)
}
