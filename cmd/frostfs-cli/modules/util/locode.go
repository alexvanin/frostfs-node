package util

import (
	"github.com/spf13/cobra"
)

// locode section.
var locodeCmd = &cobra.Command{
	Use:   "locode",
	Short: "Working with FrostFS UN/LOCODE database",
}

func initLocodeCmd() {
	locodeCmd.AddCommand(locodeGenerateCmd, locodeInfoCmd)

	initUtilLocodeInfoCmd()
	initUtilLocodeGenerateCmd()
}
