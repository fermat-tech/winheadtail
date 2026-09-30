package shared

import "testing"

func TestIsReleaseVersion(t *testing.T) {
	// Only a version a tag could have given is worth reporting; everything
	// the toolchain makes up for an untagged or modified tree must be
	// rejected so that --version falls back to the released semver.
	release := []string{
		"v1.0.0",
		"v1.1.0",
		"v2.13.4",
		"v1.0.0-rc1",
		"v1.0.0-beta.2",
	}
	for _, v := range release {
		if !isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = false; want true", v)
		}
	}

	synthesized := []string{
		"",
		"(devel)",
		"dev",
		"1.0.0", // no leading v
		"v0.0.0-20260905220925-4bad63905266",
		"v0.0.0-20260905220925-4bad63905266+dirty",
		"v1.0.0+dirty",
		"v1.2.3-0.20260905220925-abcdef123456",
		"v1.2.3-pre.0.20260905220925-abcdef123456",
	}
	for _, v := range synthesized {
		if isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = true; want false", v)
		}
	}
}

func TestIsPseudoVersion(t *testing.T) {
	plain := []string{
		"v1.0.0",
		"v1.0.0-rc1",
		"v1.0.0-20260905220925",              // a timestamp but no commit
		"v0.0.0-2026090522092-4bad63905266",  // 13-digit timestamp
		"v0.0.0-20260905220925-4bad6390526g", // not hexadecimal
		"short",
	}
	for _, v := range plain {
		if isPseudoVersion(v) {
			t.Errorf("isPseudoVersion(%q) = true; want false", v)
		}
	}
}

func TestBuildVersionNeverReportsNothing(t *testing.T) {
	if got := BuildVersion(); got == "" {
		t.Error("BuildVersion() returned an empty version")
	}
}
