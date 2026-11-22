package policy

import (
	"fmt"
	"github.com/keel-hq/keel/types"
	"strings"

	"github.com/ryanuber/go-glob"
)

type GlobPolicy struct {
	policy  string // original string
	pattern string // without prefix
}

func NewGlobPolicy(policy string) (*GlobPolicy, error) {
	if strings.Contains(policy, ":") {
		parts := strings.Split(policy, ":")
		if len(parts) == 2 {
			return &GlobPolicy{
				policy:  policy,
				pattern: parts[1],
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid glob policy: %s", policy)
}

func (p *GlobPolicy) ShouldUpdate(current string, new string) (bool, error) {
	return (glob.Glob(p.pattern, new) && isNewerTag(new, current)), nil
}

func (p *GlobPolicy) Filter(tags []string) []string {
	filtered := []string{}

	for _, tag := range tags {
		if glob.Glob(p.pattern, tag) {
			filtered = append(filtered, tag)
		}
	}

	sortTagsDesc(filtered, func(tag string) string { return tag })

	return filtered
}

func (p *GlobPolicy) Name() string           { return p.policy }
func (p *GlobPolicy) Type() types.PolicyType { return types.PolicyTypeGlob }
func (p *GlobPolicy) KeepTag() bool          { return false }
