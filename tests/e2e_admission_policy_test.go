package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// admissionPolicyManifest is the optional hardening policy from the static manifests. The e2e suite applies
// it to its own Keel, so every scenario also proves that the policy admits what Keel changes.
var admissionPolicyManifest = filepath.Join("..", "docs", "manifests", "keel", "optional", "admission-policy.yaml")

const admissionPolicyUsername = "system:serviceaccount:keel:keel"

func (cfg e2eConfig) admissionPolicyName() string {
	return "keel-image-updates-only-" + cfg.runID
}

func (cfg e2eConfig) keelUsername() string {
	return "system:serviceaccount:" + cfg.systemNamespace() + ":keel"
}

// loadAdmissionPolicy reads the policy and its binding from the manifest and adapts them to a namespace:
// the ServiceAccount username in matchConditions, the object names and the labels.
func loadAdmissionPolicy(path, name, username string, labels map[string]string) (*admissionv1.ValidatingAdmissionPolicy, *admissionv1.ValidatingAdmissionPolicyBinding, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var policy *admissionv1.ValidatingAdmissionPolicy
	var binding *admissionv1.ValidatingAdmissionPolicyBinding
	for _, document := range strings.Split(string(contents), "\n---\n") {
		var meta metav1.TypeMeta
		if err := yaml.Unmarshal([]byte(document), &meta); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}
		switch meta.Kind {
		case "ValidatingAdmissionPolicy":
			policy = &admissionv1.ValidatingAdmissionPolicy{}
			if err := yaml.UnmarshalStrict([]byte(document), policy); err != nil {
				return nil, nil, fmt.Errorf("parse policy: %w", err)
			}
		case "ValidatingAdmissionPolicyBinding":
			binding = &admissionv1.ValidatingAdmissionPolicyBinding{}
			if err := yaml.UnmarshalStrict([]byte(document), binding); err != nil {
				return nil, nil, fmt.Errorf("parse binding: %w", err)
			}
		}
	}
	if policy == nil || binding == nil {
		return nil, nil, fmt.Errorf("%s must contain a ValidatingAdmissionPolicy and its binding", path)
	}
	if binding.Spec.PolicyName != policy.Name {
		return nil, nil, fmt.Errorf("binding refers to %q, policy is %q", binding.Spec.PolicyName, policy.Name)
	}

	replaced := 0
	for i, condition := range policy.Spec.MatchConditions {
		if strings.Contains(condition.Expression, "'"+admissionPolicyUsername+"'") {
			policy.Spec.MatchConditions[i].Expression = strings.ReplaceAll(condition.Expression, admissionPolicyUsername, username)
			replaced++
		}
	}
	if replaced != 1 {
		return nil, nil, fmt.Errorf("expected one matchCondition with %q, found %d", admissionPolicyUsername, replaced)
	}

	policy.Name = name
	policy.Labels = labels
	binding.Name = name
	binding.Labels = labels
	binding.Spec.PolicyName = name
	return policy, binding, nil
}

func createAdmissionPolicy(ctx context.Context, client kubernetes.Interface, cfg e2eConfig) error {
	policy, binding, err := loadAdmissionPolicy(admissionPolicyManifest, cfg.admissionPolicyName(), cfg.keelUsername(), cfg.labels())
	if err != nil {
		return err
	}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		return err
	}
	_, err = client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{})
	return err
}

func deleteAdmissionPolicy(ctx context.Context, client kubernetes.Interface, cfg e2eConfig) {
	_ = client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, cfg.admissionPolicyName(), metav1.DeleteOptions{})
	_ = client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, cfg.admissionPolicyName(), metav1.DeleteOptions{})
}

// keelImpersonatingClient acts as Keel's ServiceAccount, the only user the policy applies to.
func (s *E2ESuite) keelImpersonatingClient() *kubernetes.Clientset {
	restConfig, err := clientcmd.BuildConfigFromFlags("", s.cfg.kubeconfig)
	s.Require().NoError(err)
	restConfig.Impersonate.UserName = s.cfg.keelUsername()
	client, err := kubernetes.NewForConfig(restConfig)
	s.Require().NoError(err)
	return client
}

// The scenarios above show that the policy admits Keel's own updates. This one shows that it stops other
// changes made with Keel's credentials. The requests are dry runs, so nothing is stored.
func (s *E2ESuite) TestAdmissionPolicyLimitsKeelServiceAccount() {
	ctx := context.Background()
	repository := s.cfg.repositoryPrefix + "/admission"
	managed := testPolicyDeployment(s.testNamespace, "admission-managed", repository+":1.0.0", map[string]string{"keel.sh/policy": "never"})
	unmanaged := testPolicyDeployment(s.testNamespace, "admission-unmanaged", repository+":1.0.0", nil)
	for _, deployment := range []*appsv1.Deployment{managed, unmanaged} {
		_, err := s.client.AppsV1().Deployments(s.testNamespace).Create(ctx, deployment, metav1.CreateOptions{})
		s.Require().NoError(err)
	}

	keel := s.keelImpersonatingClient()
	privileged := true
	cases := []struct {
		name   string
		target string
		change func(*appsv1.Deployment)
		denied string
	}{
		{"image tag", "admission-managed", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Image = repository + ":1.0.1"
			d.Spec.Template.Annotations = map[string]string{"keel.sh/update-time": "now"}
			d.Annotations["kubernetes.io/change-cause"] = "keel automated update"
		}, ""},
		{"other repository", "admission-managed", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Image = s.cfg.repositoryPrefix + "/other:1.0.1"
		}, "image tag or digest of a container"},
		{"privileged", "admission-managed", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{Privileged: &privileged}
		}, "image tag or digest of a container"},
		{"host network", "admission-managed", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.HostNetwork = true
		}, "pod spec outside containers and volumes"},
		{"replicas", "admission-managed", func(d *appsv1.Deployment) {
			replicas := int32(3)
			d.Spec.Replicas = &replicas
		}, "workload spec outside the pod template"},
		{"other annotation", "admission-managed", func(d *appsv1.Deployment) {
			d.Annotations["example.com/other"] = "x"
		}, "annotations kubernetes.io/change-cause"},
		{"workload without keel.sh/policy", "admission-unmanaged", func(d *appsv1.Deployment) {
			d.Spec.Template.Spec.Containers[0].Image = repository + ":1.0.1"
		}, "keel.sh/policy label or annotation"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			current, err := s.client.AppsV1().Deployments(s.testNamespace).Get(ctx, tc.target, metav1.GetOptions{})
			s.Require().NoError(err)
			if current.Annotations == nil {
				current.Annotations = map[string]string{}
			}
			tc.change(current)
			_, err = keel.AppsV1().Deployments(s.testNamespace).Update(ctx, current, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
			if tc.denied == "" {
				s.Require().NoError(err)
				return
			}
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.denied)
		})
	}
}

func testPolicyDeployment(namespace, name, image string, annotations map[string]string) *appsv1.Deployment {
	replicas := int32(0)
	labels := map[string]string{"app": name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: annotations},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: image}},
				},
			},
		},
	}
}
