package kubernetes

import (
	"testing"
	"time"

	"github.com/keel-hq/keel/internal/k8s"
	"github.com/keel-hq/keel/types"

	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckRequestedApproval(t *testing.T) {
	fp := &fakeImplementer{}
	fp.namespaces = &v1.NamespaceList{
		Items: []v1.Namespace{
			{
				meta_v1.TypeMeta{},
				meta_v1.ObjectMeta{Name: "xxxx"},
				v1.NamespaceSpec{},
				v1.NamespaceStatus{},
			},
		},
	}
	deployments := []*apps_v1.Deployment{
		{
			meta_v1.TypeMeta{},
			meta_v1.ObjectMeta{
				Name:        "dep-1",
				Namespace:   "xxxx",
				Labels:      map[string]string{types.KeelPolicyLabel: "all", types.KeelMinimumApprovalsLabel: "1"},
				Annotations: map[string]string{},
			},
			apps_v1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Image: "gcr.io/v2-namespace/hello-world:1.1.1",
							},
						},
					},
				},
			},
			apps_v1.DeploymentStatus{},
		},
	}

	grs := MustParseGRS(deployments)
	grc := &k8s.GenericResourceCache{}
	grc.Add(grs...)

	approver, teardown := approver()
	defer teardown()
	provider, err := NewProvider(fp, &fakeSender{}, approver, grc)
	if err != nil {
		t.Fatalf("failed to get provider: %s", err)
	}
	// creating "new version" event
	repo := types.Repository{
		Name: "gcr.io/v2-namespace/hello-world",
		Tag:  "1.1.2",
	}

	deps, err := provider.processEvent(&types.Event{Repository: repo})
	if err != nil {
		t.Errorf("failed to get deployments: %s", err)
	}

	if len(deps) != 0 {
		t.Errorf("expected to find 0 updated deployment but found %d", len(deps))
	}

	// checking approvals
	approval, err := provider.approvalManager.Get("deployment/xxxx/dep-1:1.1.2")
	if err != nil {
		t.Fatalf("failed to find approval, err: %s", err)
	}

	if approval.Provider != types.ProviderTypeKubernetes {
		t.Errorf("wrong provider: %s", approval.Provider)
	}
}

func TestCheckRequestedApprovalAnnotation(t *testing.T) {
	fp := &fakeImplementer{}
	fp.namespaces = &v1.NamespaceList{
		Items: []v1.Namespace{
			{
				meta_v1.TypeMeta{},
				meta_v1.ObjectMeta{Name: "xxxx"},
				v1.NamespaceSpec{},
				v1.NamespaceStatus{},
			},
		},
	}
	deployments := []*apps_v1.Deployment{
		{
			meta_v1.TypeMeta{},
			meta_v1.ObjectMeta{
				Name:      "dep-1",
				Namespace: "xxxx",
				Labels:    map[string]string{},
				Annotations: map[string]string{
					types.KeelPolicyLabel:           "all",
					types.KeelMinimumApprovalsLabel: "3",
					types.KeelApprovalDeadlineLabel: "20",
				},
			},
			apps_v1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Image: "gcr.io/v2-namespace/hello-world:1.1.1",
							},
						},
					},
				},
			},
			apps_v1.DeploymentStatus{},
		},
	}

	grs := MustParseGRS(deployments)
	grc := &k8s.GenericResourceCache{}
	grc.Add(grs...)

	approver, teardown := approver()
	defer teardown()
	provider, err := NewProvider(fp, &fakeSender{}, approver, grc)
	if err != nil {
		t.Fatalf("failed to get provider: %s", err)
	}
	// creating "new version" event
	repo := types.Repository{
		Name: "gcr.io/v2-namespace/hello-world",
		Tag:  "1.1.2",
	}

	deps, err := provider.processEvent(&types.Event{Repository: repo})
	if err != nil {
		t.Errorf("failed to get deployments: %s", err)
	}

	if len(deps) != 0 {
		t.Errorf("expected to find 0 updated deployment but found %d", len(deps))
	}

	// checking approvals
	approval, err := provider.approvalManager.Get("deployment/xxxx/dep-1:1.1.2")
	if err != nil {
		t.Fatalf("failed to find approval, err: %s", err)
	}

	if approval.Provider != types.ProviderTypeKubernetes {
		t.Errorf("wrong provider: %s", approval.Provider)
	}

	if approval.VotesRequired != 3 {
		t.Errorf("expected 3 required votes, got: %d", approval.VotesRequired)
	}
	if approval.Deadline.Before(time.Now().Add(19 * time.Hour)) {
		t.Errorf("unexpected deadline: %s", approval.Deadline)
	}
}

func TestApprovedCheck(t *testing.T) {
	fp := &fakeImplementer{}
	fp.namespaces = &v1.NamespaceList{
		Items: []v1.Namespace{
			{
				meta_v1.TypeMeta{},
				meta_v1.ObjectMeta{Name: "xxxx"},
				v1.NamespaceSpec{},
				v1.NamespaceStatus{},
			},
		},
	}
	deployments := []*apps_v1.Deployment{
		{
			meta_v1.TypeMeta{},
			meta_v1.ObjectMeta{
				Name:        "dep-1",
				Namespace:   "xxxx",
				Labels:      map[string]string{types.KeelPolicyLabel: "all", types.KeelMinimumApprovalsLabel: "1"},
				Annotations: map[string]string{},
			},
			apps_v1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Image: "gcr.io/v2-namespace/hello-world:1.1.1",
							},
						},
					},
				},
			},
			apps_v1.DeploymentStatus{},
		},
	}
	grs := MustParseGRS(deployments)
	grc := &k8s.GenericResourceCache{}
	grc.Add(grs...)

	approver, teardown := approver()
	defer teardown()
	provider, err := NewProvider(fp, &fakeSender{}, approver, grc)
	if err != nil {
		t.Fatalf("failed to get provider: %s", err)
	}

	// approving event
	err = provider.approvalManager.Create(&types.Approval{
		Identifier:    "deployment/xxxx/dep-1:1.1.2",
		VotesReceived: 2,
		VotesRequired: 2,
		Deadline:      time.Now().Add(10 * time.Second),
	})
	if err != nil {
		t.Fatalf("failed to create approval: %s", err)
	}

	appr, err := provider.approvalManager.Get("deployment/xxxx/dep-1:1.1.2")
	if err != nil {
		t.Fatalf("failed to get approval: %s", err)
	}
	if appr.Status() != types.ApprovalStatusApproved {
		t.Fatalf("approval not approved")
	}

	// creating "new version" event
	repo := types.Repository{
		Name: "gcr.io/v2-namespace/hello-world",
		Tag:  "1.1.2",
	}

	deps, err := provider.processEvent(&types.Event{Repository: repo})
	if err != nil {
		t.Errorf("failed to get deployments: %s", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected to find 1 updated deployment but found %d", len(deps))
	}
}

func TestApprovalsCleanup(t *testing.T) {
	fp := &fakeImplementer{}
	fp.namespaces = &v1.NamespaceList{
		Items: []v1.Namespace{
			{
				meta_v1.TypeMeta{},
				meta_v1.ObjectMeta{Name: "xxxx"},
				v1.NamespaceSpec{},
				v1.NamespaceStatus{},
			},
		},
	}
	deployments := []*apps_v1.Deployment{
		{
			meta_v1.TypeMeta{},
			meta_v1.ObjectMeta{
				Name:        "dep-1",
				Namespace:   "xxxx",
				Labels:      map[string]string{types.KeelPolicyLabel: "all", types.KeelMinimumApprovalsLabel: "1"},
				Annotations: map[string]string{},
			},
			apps_v1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Image: "gcr.io/v2-namespace/hello-world:1.1.1",
							},
						},
					},
				},
			},
			apps_v1.DeploymentStatus{},
		},
	}
	grs := MustParseGRS(deployments)
	grc := &k8s.GenericResourceCache{}
	grc.Add(grs...)

	approver, teardown := approver()
	defer teardown()
	provider, err := NewProvider(fp, &fakeSender{}, approver, grc)
	if err != nil {
		t.Fatalf("failed to get provider: %s", err)
	}

	// approving event
	err = provider.approvalManager.Create(&types.Approval{
		Identifier:    "deployment/xxxx/dep-1:1.1.2",
		VotesReceived: 2,
		VotesRequired: 2,
		Deadline:      time.Now().Add(10 * time.Second),
	})
	if err != nil {
		t.Fatalf("failed to create approval: %s", err)
	}

	appr, err := provider.approvalManager.Get("deployment/xxxx/dep-1:1.1.2")
	if err != nil {
		t.Fatalf("failed to get approval: %s", err)
	}
	if appr.Status() != types.ApprovalStatusApproved {
		t.Fatalf("approval not approved")
	}

	// creating "new version" event
	repo := types.Repository{
		Name: "gcr.io/v2-namespace/hello-world",
		Tag:  "1.1.2",
	}

	deps, err := provider.processEvent(&types.Event{Repository: repo})
	if err != nil {
		t.Errorf("failed to get deployments: %s", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected to find 1 updated deployment but found %d", len(deps))
	}

	// no approvals expected

	approvals, err := provider.approvalManager.List()
	if err != nil {
		t.Fatalf("failed to get a list of approvals: %s", err)
	}

	if len(approvals) != 1 && !approvals[0].Archived {
		t.Errorf("expected to find 1 archived approval but found %d", len(approvals))
		t.Logf("approval status: %v, identifier: %s", approvals[0].Archived, approvals[0].Identifier)
	}
}

func TestApprovalsFor(t *testing.T) {
	tests := []struct {
		name         string
		approvalsFor string
		useLabel     bool
		current      string
		new          string
		wantUpdate   bool
	}{
		{name: "major, patch update", approvalsFor: "major", current: "1.1.1", new: "1.1.2", wantUpdate: true},
		{name: "major, minor update", approvalsFor: "major", current: "1.1.1", new: "1.2.0", wantUpdate: true},
		{name: "major, major update", approvalsFor: "major", current: "1.1.1", new: "2.0.0", wantUpdate: false},
		{name: "major as label, patch update", approvalsFor: "major", useLabel: true, current: "1.1.1", new: "1.1.2", wantUpdate: true},
		{name: "minor, patch update", approvalsFor: "minor", current: "1.1.1", new: "1.1.2", wantUpdate: true},
		{name: "minor, minor update", approvalsFor: "minor", current: "1.1.1", new: "1.2.0", wantUpdate: false},
		{name: "major, four parts revision", approvalsFor: "major", current: "21.0.0.0", new: "21.0.0.1", wantUpdate: true},
		{name: "major, not a version", approvalsFor: "major", current: "build-1", new: "build-2", wantUpdate: false},
		{name: "unset, patch update", approvalsFor: "", current: "1.1.1", new: "1.1.2", wantUpdate: false},
		{name: "unknown value, patch update", approvalsFor: "sometimes", current: "1.1.1", new: "1.1.2", wantUpdate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := &fakeImplementer{}
			fp.namespaces = &v1.NamespaceList{
				Items: []v1.Namespace{{ObjectMeta: meta_v1.ObjectMeta{Name: "xxxx"}}},
			}
			labels := map[string]string{}
			annotations := map[string]string{
				types.KeelPolicyLabel:           "force",
				types.KeelMinimumApprovalsLabel: "1",
			}
			if tt.approvalsFor != "" {
				if tt.useLabel {
					labels[types.KeelApprovalsForLabel] = tt.approvalsFor
				} else {
					annotations[types.KeelApprovalsForLabel] = tt.approvalsFor
				}
			}
			deployments := []*apps_v1.Deployment{
				{
					ObjectMeta: meta_v1.ObjectMeta{
						Name:        "dep-1",
						Namespace:   "xxxx",
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: apps_v1.DeploymentSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								Containers: []v1.Container{
									{Image: "gcr.io/v2-namespace/hello-world:" + tt.current},
								},
							},
						},
					},
				},
			}

			grc := &k8s.GenericResourceCache{}
			grc.Add(MustParseGRS(deployments)...)

			approver, teardown := approver()
			defer teardown()
			sender := &fakeSender{}
			provider, err := NewProvider(fp, sender, approver, grc)
			if err != nil {
				t.Fatalf("failed to get provider: %s", err)
			}

			repo := types.Repository{Name: "gcr.io/v2-namespace/hello-world", Tag: tt.new}
			deps, err := provider.processEvent(&types.Event{Repository: repo})
			if err != nil {
				t.Fatalf("failed to process event: %s", err)
			}

			_, approvalErr := provider.approvalManager.Get("deployment/xxxx/dep-1:" + tt.new)
			if tt.wantUpdate {
				if len(deps) != 1 {
					t.Fatalf("expected 1 updated deployment, got %d", len(deps))
				}
				if approvalErr == nil {
					t.Errorf("expected no approval request for a direct update")
				}
				sender.mu.Lock()
				sent := sender.sentEvent
				sender.mu.Unlock()
				if sent.Type != types.NotificationDeploymentUpdate || sent.Level != types.LevelSuccess {
					t.Errorf("expected a successful deployment update notification, got type %s, level %s", sent.Type, sent.Level)
				}
				return
			}
			if len(deps) != 0 {
				t.Fatalf("expected 0 updated deployments, got %d", len(deps))
			}
			if approvalErr != nil {
				t.Errorf("expected an approval request, got error: %s", approvalErr)
			}
		})
	}
}
