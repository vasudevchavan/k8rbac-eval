package access

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vasudevchavan/k8s-get-access-level/util"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var verbs []string

var GenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate access manifests for user and service account",
}

var GenerateUserCmd = &cobra.Command{
	Use:   "user [username]",
	Short: "Generate Role/Binding for a user",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Use ValidateCommonFlags but we might need lenient validation if some flags like --namespace are optional for cluster roles
		// But ValidateCommonFlags enforces namespace logic which is good.
		return ValidateCommonFlags(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(args[0], false)
	},
}

var GenerateSaCmd = &cobra.Command{
	Use:   "sa [serviceaccount]",
	Short: "Generate Role/Binding for a service account",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return ValidateCommonFlags(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenerate(args[0], true)
	},
}

func init() {
	GenerateCmd.AddCommand(GenerateUserCmd)
	GenerateCmd.AddCommand(GenerateSaCmd)

	addCommonFlags(GenerateUserCmd)
	addCommonFlags(GenerateSaCmd)

	GenerateUserCmd.Flags().StringSliceVar(&verbs, "verb", []string{"get", "list", "watch"}, "Verbs for the role")
	GenerateSaCmd.Flags().StringSliceVar(&verbs, "verb", []string{"get", "list", "watch"}, "Verbs for the role")
}

func runGenerate(name string, isServiceAccount bool) error {
	clientset, err := util.GetClientset()
	if err != nil {
		return err
	}

	// Resolve resource aliases if provided
	if resource != "" {
		resolved, err := util.ResolveResourceName(clientset.Discovery(), resource)
		if err != nil {
			return err
		}
		resource = resolved
	} else {
		return fmt.Errorf("resource must be specified via --resource")
	}

	resolver, err := util.NewResourceScopeResolver(clientset.Discovery())
	if err != nil {
		return err
	}

	namespaced, err := resolver.IsNamespaced(resource)
	if err != nil {
		return err
	}

	if namespaced && clusterScope {
		return fmt.Errorf("cannot use --clusterscope with namespaced resource %s", resource)
	}

	// Try to resolve group
	var group string
	gvr, err := resolver.ResourceFor(resource)
	if err == nil {
		group = gvr.Group
	}

	roleBytes, bindingBytes, err := generateManifests(name, isServiceAccount, resource, group, verbs, userNamespace, namespaced)
	if err != nil {
		return err
	}

	fmt.Println(string(roleBytes))
	fmt.Println("---")
	fmt.Println(string(bindingBytes))

	return nil
}

func generateManifests(
	name string,
	isServiceAccount bool,
	resource string,
	group string,
	verbs []string,
	namespace string,
	namespaced bool,
) ([]byte, []byte, error) {

	var roleBytes, bindingBytes []byte

	if namespaced {
		// Role + RoleBinding
		roleName := fmt.Sprintf("%s-role", name)
		bindingName := fmt.Sprintf("%s-binding", name)

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

		roleBytes, _ = yaml.Marshal(role)
		bindingBytes, _ = yaml.Marshal(binding)

	} else {
		// ClusterRole + ClusterRoleBinding
		roleName := fmt.Sprintf("%s-clusterrole", name)
		bindingName := fmt.Sprintf("%s-clusterbinding", name)

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

		subject := rbacv1.Subject{
			Kind: "User",
			Name: name,
		}
		if isServiceAccount {
			subject.Kind = "ServiceAccount"
			subject.Name = name
			if namespace == "" {
				subject.Namespace = "default"
			} else {
				subject.Namespace = namespace
			}
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

		roleBytes, _ = yaml.Marshal(role)
		bindingBytes, _ = yaml.Marshal(binding)
	}

	return roleBytes, bindingBytes, nil
}
