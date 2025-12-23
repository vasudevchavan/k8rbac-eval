package access

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userNamespace string
var userCluster bool

var UserCmd = &cobra.Command{
	Use:   "user [username]",
	Short: "show kubernetes access",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		username := args[0]
		fmt.Printf("Inspecting user:%s \n", username)
		fmt.Printf("namespace:%s \n", userNamespace)
		fmt.Printf("Cluster:%v \n", userCluster)
	},
}

func init() {
	UserCmd.Flags().StringVarP(&userNamespace, "namespace", "n", "default", "Namespace Scope")
	UserCmd.Flags().BoolVarP(&userCluster, "cluster", "c", false, "Cluster scope")
	// AccessCmd.AddCommand(UserCmd)
}
