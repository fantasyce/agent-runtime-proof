package versioninfo

import "testing"

func TestFormatReturnsBoundedReleaseIdentity(t *testing.T) {
	got := Format("agent-runtime-proof", "1.0.0", "0123456789abcdef")
	want := "agent-runtime-proof 1.0.0 (0123456789abcdef)"
	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

func TestFormatRejectsUnsafeBuildValues(t *testing.T) {
	if got := Format("agent-runtime-proof\nsecret", "1.0.0", "abcdef0"); got != "agent-runtime-proof unknown (unknown)" {
		t.Fatalf("unsafe Format() = %q", got)
	}
}
