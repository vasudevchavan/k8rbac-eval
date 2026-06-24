package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kubeaccess",
	Short: "Inspect and generate Kubernetes RBAC access",
	Long: `kubeaccess inspects RBAC access levels for users and service accounts
and generates ready-to-apply Role/ClusterRole manifests.

Examples:
  kubeaccess show user alice -n default --resource pods
  kubeaccess show sa my-app -n default --resource secrets
  kubeaccess generate user bob --resource deployments --verb create --verb delete
  kubeaccess generate sa monitor-sa --resource nodes --verb get --verb list -c`,
}

// Execute is the entrypoint called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(AccessCmd)
	rootCmd.AddCommand(GenerateCmd)
	rootCmd.AddCommand(VersionCmd)
}
