package tests

import (
	"slices"
	"strings"
	"testing"
)

func TestAdmissionPolicyManifestAdaptsToNamespace(t *testing.T) {
	labels := map[string]string{"app": "keel-e2e"}
	policy, binding, err := loadAdmissionPolicy(admissionPolicyManifest, "keel-image-updates-only-run", "system:serviceaccount:keel-e2e-system-run:keel", labels)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Name != "keel-image-updates-only-run" || binding.Name != policy.Name || binding.Spec.PolicyName != policy.Name {
		t.Fatalf("names not adapted: policy %q, binding %q -> %q", policy.Name, binding.Name, binding.Spec.PolicyName)
	}
	if len(policy.Spec.MatchConditions) != 1 || !strings.Contains(policy.Spec.MatchConditions[0].Expression, "'system:serviceaccount:keel-e2e-system-run:keel'") {
		t.Fatalf("ServiceAccount username not adapted: %+v", policy.Spec.MatchConditions)
	}
	if policy.Spec.FailurePolicy == nil || *policy.Spec.FailurePolicy != "Fail" {
		t.Fatalf("policy must fail closed, got %v", policy.Spec.FailurePolicy)
	}
	if !slices.Contains(binding.Spec.ValidationActions, "Deny") {
		t.Fatalf("binding must deny, got %v", binding.Spec.ValidationActions)
	}

	// every kind the ClusterRole lets Keel update must be covered
	covered := map[string]bool{}
	for _, rule := range policy.Spec.MatchConstraints.ResourceRules {
		if !slices.Contains(rule.Operations, "UPDATE") {
			continue
		}
		for _, resource := range rule.Resources {
			covered[resource] = true
		}
	}
	for _, resource := range []string{"deployments", "statefulsets", "daemonsets", "cronjobs"} {
		if !covered[resource] {
			t.Errorf("policy does not cover updates of %s", resource)
		}
	}
	if len(policy.Spec.Validations) == 0 {
		t.Fatal("policy has no validations")
	}
}
