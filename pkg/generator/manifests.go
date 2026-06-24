package generator

import (
	"fmt"
	"regexp"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// sanitizeName converts an arbitrary subject name into a DNS-label-safe string
// suitable for use in Kubernetes resource names.
// e.g. "system:serviceaccount:default:my-app" → "system-serviceaccount-default-my-app"
var nonDNSRe = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeName(name string) string {
	lower := strings.ToLower(name)
	safe := nonDNSRe.ReplaceAllString(lower, "-")
	// Trim leading/trailing hyphens
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "rbac"
	}
	return safe
}

// GenerateManifests generates RBAC manifests (Role/Binding or ClusterRole/Binding).
func GenerateManifests(
	name string,
	isServiceAccount bool,
	resource string,
	group string,
	verbs []string,
	namespace string,
	namespaced bool,
) ([]byte, []byte, error) {

	safeName := sanitizeName(name)

	if namespaced {
		// Role + RoleBinding
		roleName := fmt.Sprintf("%s-role", safeName)
		bindingName := fmt.Sprintf("%s-binding", safeName)

		role := &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "Role",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{group},
					Resources: []string{resource},
					Verbs:     verbs,
				},
			},
		}
		if group == "" {
			role.Rules[0].APIGroups = []string{""}
		}

		subject := rbacv1.Subject{
			Kind: "User",
			Name: name,
		}
		if isServiceAccount {
			subject.Kind = "ServiceAccount"
			subject.Name = name
			subject.Namespace = namespace
		}

		binding := &rbacv1.RoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "RoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{subject},
			RoleRef: rbacv1.RoleRef{
				Kind:     "Role",
				Name:     roleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}

		roleBytes, err := yaml.Marshal(role)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling Role: %w", err)
		}
		bindingBytes, err := yaml.Marshal(binding)
		if err != nil {
			return nil, nil, fmt.Errorf("marshaling RoleBinding: %w", err)
		}
		return roleBytes, bindingBytes, nil
	}

	// ClusterRole + ClusterRoleBinding
	roleName := fmt.Sprintf("%s-clusterrole", safeName)
	bindingName := fmt.Sprintf("%s-clusterbinding", safeName)

	role := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{group},
				Resources: []string{resource},
				Verbs:     verbs,
			},
		},
	}
	if group == "" {
		role.Rules[0].APIGroups = []string{""}
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}
	subject := rbacv1.Subject{
		Kind: "User",
		Name: name,
	}
	if isServiceAccount {
		subject.Kind = "ServiceAccount"
		subject.Name = name
		subject.Namespace = ns
	}

	binding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []rbacv1.Subject{subject},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	roleBytes, err := yaml.Marshal(role)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling ClusterRole: %w", err)
	}
	bindingBytes, err := yaml.Marshal(binding)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling ClusterRoleBinding: %w", err)
	}
	return roleBytes, bindingBytes, nil
}
