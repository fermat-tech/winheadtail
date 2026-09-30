package shared

import (
	"runtime/debug"
	"strings"
)

// DefaultVersion is the version of this source. It is what --version reports
// for any build the Go toolchain cannot name a released version for: a source
// archive with no repository in it, a checkout that is not sitting on a tag,
// and a working tree with uncommitted changes in it.
const DefaultVersion = "v1.1.0"

// BuildVersion reports the version the Go toolchain stamped into the binary,
// but only when that names a real release: `go install module@vX.Y.Z` records
// the version asked for, and a plain `go build` in a clean checkout sitting on
// a tag records the tag, so both of those are worth preferring over
// DefaultVersion — a later tag then reports itself without an edit here.
//
// Everything else the toolchain synthesizes is rejected. A commit that no tag
// names gets a pseudo-version, and a modified working tree gets a "+dirty"
// suffix; neither is a version anyone released.
func BuildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return DefaultVersion
	}
	if v := bi.Main.Version; isReleaseVersion(v) {
		return v
	}
	return DefaultVersion
}

// isReleaseVersion reports whether v is a version a tag could have given,
// rather than one the toolchain made up for an untagged or dirty build.
func isReleaseVersion(v string) bool {
	if len(v) < 2 || v[0] != 'v' || v == "(devel)" {
		return false
	}
	// Build metadata only ever arrives here as the "+dirty" the toolchain
	// appends to the version of a modified working tree.
	if strings.ContainsRune(v, '+') {
		return false
	}
	return !isPseudoVersion(v)
}

// isPseudoVersion reports whether v is one of the versions the Go toolchain
// derives from a bare commit. Every form of them ends in a 14-digit UTC
// timestamp and a 12-digit commit prefix — v0.0.0-20260905220925-4bad63905266,
// v1.2.3-0.20260905220925-4bad63905266 and
// v1.2.3-pre.0.20260905220925-4bad63905266 — so the separator before the
// timestamp is a dot in the two that carry a base version.
func isPseudoVersion(v string) bool {
	const stampLen, hashLen = 14, 12

	if len(v) < stampLen+hashLen+2 {
		return false
	}
	hash := v[len(v)-hashLen:]
	stamp := v[len(v)-hashLen-1-stampLen : len(v)-hashLen-1]

	if v[len(v)-hashLen-1] != '-' {
		return false
	}
	if sep := v[len(v)-hashLen-1-stampLen-1]; sep != '-' && sep != '.' {
		return false
	}
	for i := 0; i < len(stamp); i++ {
		if stamp[i] < '0' || stamp[i] > '9' {
			return false
		}
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
