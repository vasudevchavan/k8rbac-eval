package access

import (
	"github.com/spf13/cobra"
)

var UserCmd = &cobra.Command{
	Use:     "user [username]",
	Short:   "Show Kubernetes access for user",
	Args:    cobra.ExactArgs(1),
	PreRunE: ValidateCommonFlags,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunAccessCheck(cmd, args, false)
	},
}

func init() {
	addCommonFlags(UserCmd)
}
