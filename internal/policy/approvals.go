package policy

import (
	"fmt"
	"strings"

	"github.com/keel-hq/keel/util/version"
)

// Values of the keel.sh/approvalsFor annotation (keel.approvalsFor in Helm values). They limit the
// approvals that keel.sh/approvals asks for to the larger version changes.
const (
	// ApprovalsForAll asks for approvals on every update. It is the default, an empty value means the same.
	ApprovalsForAll = "all"
	// ApprovalsForMinor asks for approvals on major and minor changes, patch and smaller updates go through.
	ApprovalsForMinor = "minor"
	// ApprovalsForMajor asks for approvals on major changes only, everything smaller goes through.
	ApprovalsForMajor = "major"
)

// ApprovalRequired reports whether an update from currentTag to newTag needs approvals, given the value of
// keel.sh/approvalsFor. It compares the tags as versions (X.Y, X.Y.Z or X.Y.Z.R, with an optional v prefix):
//
//   - all or empty: always true, as without the annotation.
//   - major: true only when the major number differs.
//   - minor: true when the major or the minor number differs.
//
// A change of the patch number, the fourth number, the pre-release or only the digest of the same tag is
// never a major or minor change. When either tag is not a version (latest, a build tag of a glob, regexp or
// force policy), the size of the change is unknown and approvals are required. An unknown value of
// approvalsFor also requires approvals and returns an error, so a typo never skips an approval.
func ApprovalRequired(approvalsFor, currentTag, newTag string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(approvalsFor)) {
	case "", ApprovalsForAll:
		return true, nil
	case ApprovalsForMajor:
		current, newer, ok := tagVersions(currentTag, newTag)
		if !ok {
			return true, nil
		}
		return current.Major() != newer.Major(), nil
	case ApprovalsForMinor:
		current, newer, ok := tagVersions(currentTag, newTag)
		if !ok {
			return true, nil
		}
		return current.Major() != newer.Major() || current.Minor() != newer.Minor(), nil
	default:
		return true, fmt.Errorf("unknown approvalsFor value %q, expected %s, %s or %s", approvalsFor, ApprovalsForAll, ApprovalsForMinor, ApprovalsForMajor)
	}
}

func tagVersions(currentTag, newTag string) (current, newer *version.Semver, ok bool) {
	c, cOK := parseTagVersion(currentTag)
	n, nOK := parseTagVersion(newTag)
	if !cOK || !nOK {
		return nil, nil, false
	}
	return c, n, true
}
