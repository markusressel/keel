package helm3

import (
	"testing"

	"github.com/keel-hq/keel/types"
	"helm.sh/helm/v3/pkg/chartutil"
)

func TestApprovalsForFromValues(t *testing.T) {
	values, err := chartutil.ReadValues([]byte(`
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0
keel:
  policy: all
  approvals: 1
  approvalsFor: major
  images:
    - repository: image.repository
      tag: image.tag
`))
	if err != nil {
		t.Fatalf("failed to read values: %s", err)
	}
	cfg, err := getKeelConfig(values)
	if err != nil {
		t.Fatalf("failed to get keel config: %s", err)
	}
	if cfg.ApprovalsFor != "major" {
		t.Errorf("expected approvalsFor major, got %q", cfg.ApprovalsFor)
	}
}

func TestIsApprovedApprovalsFor(t *testing.T) {
	tests := []struct {
		name         string
		approvalsFor string
		current      string
		new          string
		wantApproved bool
	}{
		{name: "major, patch update", approvalsFor: "major", current: "1.1.0", new: "1.1.1", wantApproved: true},
		{name: "major, minor update", approvalsFor: "major", current: "1.1.0", new: "1.2.0", wantApproved: true},
		{name: "major, major update", approvalsFor: "major", current: "1.1.0", new: "2.0.0", wantApproved: false},
		{name: "minor, minor update", approvalsFor: "minor", current: "1.1.0", new: "1.2.0", wantApproved: false},
		{name: "major, four parts revision", approvalsFor: "major", current: "21.0.0.0", new: "21.0.0.1", wantApproved: true},
		{name: "major, not a version", approvalsFor: "major", current: "latest", new: "latest", wantApproved: false},
		{name: "unset, patch update", approvalsFor: "", current: "1.1.0", new: "1.1.1", wantApproved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am, teardown := approver()
			defer teardown()
			p := &Provider{approvalManager: am}

			plan := &UpdatePlan{
				Namespace:      "default",
				Name:           "release-1",
				Config:         &KeelChartConfig{Approvals: 1, ApprovalsFor: tt.approvalsFor},
				CurrentVersion: tt.current,
				NewVersion:     tt.new,
			}
			event := &types.Event{Repository: types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: tt.new}}

			approved, err := p.isApproved(event, plan)
			if err != nil {
				t.Fatalf("isApproved failed: %s", err)
			}
			if approved != tt.wantApproved {
				t.Errorf("isApproved = %v, want %v", approved, tt.wantApproved)
			}
			_, getErr := am.Get(getIdentifier(plan))
			if wantRequest := !tt.wantApproved; wantRequest != (getErr == nil) {
				t.Errorf("approval request created = %v, want %v", getErr == nil, wantRequest)
			}
		})
	}
}
