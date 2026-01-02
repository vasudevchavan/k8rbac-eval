package access

import (
	"github.com/spf13/cobra"
)

var SaCmd = &cobra.Command{
	Use:     "sa [serviceaccount]",
	Short:   "Show Kubernetes access for serviceaccount",
	Args:    cobra.ExactArgs(1),
	PreRunE: ValidateCommonFlags,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunAccessCheck(cmd, args, true)
	},
}

func init() {
	addCommonFlags(SaCmd)
}
