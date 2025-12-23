package access

import (
	"fmt"

	"github.com/spf13/cobra"
)

var GenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate access to user and service account",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Generate called")
	},
}
