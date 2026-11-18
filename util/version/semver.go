package version

import (
	"regexp"
	"strconv"

	"github.com/Masterminds/semver"
)

// fourPart matches X.Y.Z.R with an optional pre-release and build metadata,
// for example 1.2.5.6 or v1.3.1.1-rc1. Projects that ship hotfixes as a fourth
// number (Bambuddy: 1.2.5.5 Hotfix) use it, and plain semver rejects it.
var fourPart = regexp.MustCompile(`^(v?[0-9]+\.[0-9]+\.[0-9]+)\.([0-9]+)((?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?)$`)

// Semver is a semantic version with an optional fourth number (the revision,
// X.Y.Z.R). A tag without it has revision 0, so 1.2.5 < 1.2.5.1 < 1.2.6. The
// revision does not count for the major, minor and patch policies, a hotfix of
// the same X.Y.Z is allowed by all of them.
type Semver struct {
	v        *semver.Version
	revision int64
	original string
}

// NewSemver parses X.Y.Z and X.Y.Z.R versions like semver.NewVersion does, and
// rejects everything else the same way.
func NewSemver(s string) (*Semver, error) {
	if m := fourPart.FindStringSubmatch(s); m != nil {
		revision, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil, ErrInvalidSemVer
		}
		v, err := semver.NewVersion(m[1] + m[3])
		if err != nil {
			return nil, err
		}
		return &Semver{v: v, revision: revision, original: s}, nil
	}

	v, err := semver.NewVersion(s)
	if err != nil {
		return nil, err
	}
	return &Semver{v: v, original: v.Original()}, nil
}

func (s *Semver) Major() int64       { return s.v.Major() }
func (s *Semver) Minor() int64       { return s.v.Minor() }
func (s *Semver) Patch() int64       { return s.v.Patch() }
func (s *Semver) Revision() int64    { return s.revision }
func (s *Semver) Prerelease() string { return s.v.Prerelease() }
func (s *Semver) Metadata() string   { return s.v.Metadata() }
func (s *Semver) Original() string   { return s.original }
func (s *Semver) String() string     { return s.original }

// Compare returns -1, 0 or 1 when s is lower than, equal to or higher than o.
// The revision comes after the patch and before the pre-release, so
// 1.2.5-rc1 < 1.2.5 < 1.2.5.1-rc1 < 1.2.5.1.
func (s *Semver) Compare(o *Semver) int {
	if c := cmpInt(s.Major(), o.Major()); c != 0 {
		return c
	}
	if c := cmpInt(s.Minor(), o.Minor()); c != 0 {
		return c
	}
	if c := cmpInt(s.Patch(), o.Patch()); c != 0 {
		return c
	}
	if c := cmpInt(s.revision, o.revision); c != 0 {
		return c
	}
	return s.v.Compare(o.v)
}

func (s *Semver) LessThan(o *Semver) bool    { return s.Compare(o) < 0 }
func (s *Semver) GreaterThan(o *Semver) bool { return s.Compare(o) > 0 }

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Collection sorts versions in ascending order (sort.Interface).
type Collection []*Semver

func (c Collection) Len() int           { return len(c) }
func (c Collection) Less(i, j int) bool { return c[i].LessThan(c[j]) }
func (c Collection) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
