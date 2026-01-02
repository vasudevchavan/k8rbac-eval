package access

import "github.com/spf13/cobra"

func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&userNamespace,
		"namespace",
		"n",
		"default",
		"Namespace Scope",
	)

	cmd.Flags().StringVar(
		&resource,
		"resource",
		"",
		"Kubernetes resource",
	)

	cmd.Flags().BoolVarP(
		&clusterScope,
		"clusterscope",
		"c",
		false,
		"Cluster Scope",
	)
}
