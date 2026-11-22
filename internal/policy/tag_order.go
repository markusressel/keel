package policy

import (
	"sort"
	"strings"

	"github.com/keel-hq/keel/util/version"
)

// parseTagVersion returns the version of a tag with at least two parts (1.2, 1.2.3, 1.2.3.4), so that a
// bare number or a word does not count as a version.
func parseTagVersion(tag string) (*version.Semver, bool) {
	if len(strings.SplitN(tag, ".", 3)) < 2 {
		return nil, false
	}
	v, err := version.NewSemver(tag)
	if err != nil {
		return nil, false
	}
	return v, true
}

// compareTags orders two tags: by version when both are versions, otherwise as strings. A version ranks
// above a tag that is not one, which keeps the order consistent in a mixed list.
func compareTags(a, b string) int {
	va, aIsVersion := parseTagVersion(a)
	vb, bIsVersion := parseTagVersion(b)
	switch {
	case aIsVersion && bIsVersion:
		return va.Compare(vb)
	case aIsVersion:
		return 1
	case bIsVersion:
		return -1
	default:
		return strings.Compare(a, b)
	}
}

// isNewerTag reports whether candidate is newer than current: by version when both are versions,
// otherwise as strings, so that a pattern can move a workload from a version tag to a build tag.
func isNewerTag(candidate, current string) bool {
	vc, candidateIsVersion := parseTagVersion(candidate)
	vu, currentIsVersion := parseTagVersion(current)
	if candidateIsVersion && currentIsVersion {
		return vc.GreaterThan(vu)
	}
	return strings.Compare(candidate, current) > 0
}

// sortTagsDesc sorts tags from the highest to the lowest, comparing the part that key returns for each tag.
// Pattern policies return the first tag of this order that differs from the current one, so a plain string
// order would pick 1.0.9 over 1.0.12.
func sortTagsDesc(tags []string, key func(string) string) {
	sort.SliceStable(tags, func(i, j int) bool {
		return compareTags(key(tags[i]), key(tags[j])) > 0
	})
}
