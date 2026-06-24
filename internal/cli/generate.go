package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/client"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/discovery"
	"github.com/vasudevchavan/k8s-get-access-level/pkg/generator"
)

var GenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate access manifests for user and service account",
}

var GenerateUserCmd = &cobra.Command{
	Use:   "user [username]",
	Short: "Generate Role/Binding for a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := ValidateCommonFlags(cmd, args)
		if err != nil {
			return err
		}
		verbs, err := cmd.Flags().GetStringSlice("verb")
		if err != nil {
			return err
		}
		return runGenerate(args[0], false, opts, verbs)
	},
}

var GenerateSaCmd = &cobra.Command{
	Use:   "sa [serviceaccount]",
	Short: "Generate Role/Binding for a service account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := ValidateCommonFlags(cmd, args)
		if err != nil {
			return err
		}
		verbs, err := cmd.Flags().GetStringSlice("verb")
		if err != nil {
			return err
		}
		return runGenerate(args[0], true, opts, verbs)
	},
}

func init() {
	GenerateCmd.AddCommand(GenerateUserCmd)
	GenerateCmd.AddCommand(GenerateSaCmd)

	addCommonFlags(GenerateUserCmd)
	addCommonFlags(GenerateSaCmd)

	GenerateUserCmd.Flags().StringSlice("verb", []string{"get", "list", "watch"}, "Verbs for the role")
	GenerateSaCmd.Flags().StringSlice("verb", []string{"get", "list", "watch"}, "Verbs for the role")
}

func runGenerate(name string, isServiceAccount bool, opts AccessOptions, verbs []string) error {
	if opts.Resource == "" {
		return fmt.Errorf("resource must be specified via --resource")
	}

	// Resource and scope are already resolved by ValidateCommonFlags; build
	// the client with the explicit kubeconfig to resolve the API group.
	clientset, err := client.GetClientsetWithKubeconfig(opts.Kubeconfig)
	if err != nil {
		return err
	}

	resolver, err := discovery.NewResourceScopeResolver(clientset.Discovery())
	if err != nil {
		return err
	}

	namespaced, err := resolver.IsNamespaced(opts.Resource)
	if err != nil {
		return err
	}

	if namespaced && opts.ClusterScope {
		return fmt.Errorf("cannot use --clusterscope with namespaced resource %s", opts.Resource)
	}

	// Try to resolve group
	var group string
	gvr, err := resolver.ResourceFor(opts.Resource)
	if err == nil {
		group = gvr.Group
	}

	roleBytes, bindingBytes, err := generator.GenerateManifests(name, isServiceAccount, opts.Resource, group, verbs, opts.UserNamespace, namespaced)
	if err != nil {
		return err
	}

	fmt.Println(string(roleBytes))
	fmt.Println("---")
	fmt.Println(string(bindingBytes))

	return nil
}
