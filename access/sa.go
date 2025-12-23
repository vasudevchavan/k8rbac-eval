package access

import (
	"fmt"

	"github.com/spf13/cobra"
)

var saNamespace string
var saCluster bool

var SaCmd = &cobra.Command{
	Use:   "sa [namespace/name]",
	Short: "show access for serviceaccount",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		saName := args[0]
		fmt.Printf("Inspecting serviceaccount:%s \n", saName)
		fmt.Printf("namespace:%s \n", saNamespace)
		fmt.Printf("cluster:%v \n", saCluster)
	},
}

func init() {
	SaCmd.Flags().StringVarP(&saNamespace, "namespace", "n", "default", "namespace Scope")
	SaCmd.Flags().BoolVarP(&saCluster, "cluster", "c", false, "cluster scope")
}
