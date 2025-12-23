package access

import (
	"github.com/spf13/cobra"
)

var AccessCmd = &cobra.Command{
	Use:   "show",
	Short: "Show access level of user and service account",
}

func init() {
	AccessCmd.AddCommand(UserCmd)
	AccessCmd.AddCommand(SaCmd)
}
