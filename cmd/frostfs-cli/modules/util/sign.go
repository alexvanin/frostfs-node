package util

import (
	"github.com/spf13/cobra"
)

const (
	signFromFlag = "from"
	signToFlag   = "to"
)

var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign FrostFS structure",
}

func initSignCmd() {
	signCmd.AddCommand(signBearerCmd, signSessionCmd)

	initSignBearerCmd()
	initSignSessionCmd()
}
