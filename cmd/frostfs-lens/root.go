package main

import (
	"os"

	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-lens/internal/blobovnicza"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-lens/internal/meta"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-lens/internal/writecache"
	"github.com/TrueCloudLab/frostfs-node/misc"
	"github.com/TrueCloudLab/frostfs-node/pkg/util/gendoc"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:          "frostfs-lens",
	Short:        "FrostFS Storage Engine Lens",
	Long:         `FrostFS Storage Engine Lens provides tools to browse the contents of the FrostFS storage engine.`,
	RunE:         entryPoint,
	SilenceUsage: true,
}

func entryPoint(cmd *cobra.Command, _ []string) error {
	printVersion, _ := cmd.Flags().GetBool("version")
	if printVersion {
		cmd.Print(misc.BuildInfo("FrostFS Lens"))

		return nil
	}

	return cmd.Usage()
}

func init() {
	// use stdout as default output for cmd.Print()
	command.SetOut(os.Stdout)
	command.Flags().Bool("version", false, "Application version")
	command.AddCommand(
		blobovnicza.Root,
		meta.Root,
		writecache.Root,
		gendoc.Command(command),
	)
}

func main() {
	err := command.Execute()
	if err != nil {
		os.Exit(1)
	}
}
