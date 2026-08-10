package main

import "testing"

func TestValidateChangelogPost(t *testing.T) {
	title, body, issue := validateChangelogPost("  Economy update  ", "  **Details**  ")
	if issue != "" || title != "Economy update" || body != "**Details**" {
		t.Fatalf("unexpected validation result: %q %q %q", title, body, issue)
	}
	if _, _, issue = validateChangelogPost("x", "body"); issue == "" {
		t.Fatal("expected a short-title error")
	}
	if _, _, issue = validateChangelogPost("Valid title", ""); issue == "" {
		t.Fatal("expected an empty-body error")
	}
}
