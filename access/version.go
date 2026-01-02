package access

import (
	"fmt"

	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "cli version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("version 1.0")
	},
}
