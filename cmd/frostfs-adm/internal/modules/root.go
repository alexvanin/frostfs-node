package modules

import (
	"os"

	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-adm/internal/modules/config"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-adm/internal/modules/morph"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-adm/internal/modules/storagecfg"
	"github.com/TrueCloudLab/frostfs-node/misc"
	"github.com/TrueCloudLab/frostfs-node/pkg/util/autocomplete"
	"github.com/TrueCloudLab/frostfs-node/pkg/util/gendoc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:   "frostfs-adm",
		Short: "NeoFS Administrative Tool",
		Long: `NeoFS Administrative Tool provides functions to setup and
manage NeoFS network deployment.`,
		RunE:         entryPoint,
		SilenceUsage: true,
	}

	configFlag = "config"
)

func init() {
	cobra.OnInitialize(func() { initConfig(rootCmd) })
	// we need to init viper config to bind viper and cobra configurations for
	// rpc endpoint, alphabet wallet dir, key credentials, etc.

	// use stdout as default output for cmd.Print()
	rootCmd.SetOut(os.Stdout)

	rootCmd.PersistentFlags().StringP(configFlag, "c", "", "Config file")
	rootCmd.Flags().Bool("version", false, "Application version")

	rootCmd.AddCommand(config.RootCmd)
	rootCmd.AddCommand(morph.RootCmd)
	rootCmd.AddCommand(storagecfg.RootCmd)

	rootCmd.AddCommand(autocomplete.Command("frostfs-adm"))
	rootCmd.AddCommand(gendoc.Command(rootCmd))
}

func Execute() error {
	return rootCmd.Execute()
}

func entryPoint(cmd *cobra.Command, args []string) error {
	printVersion, _ := cmd.Flags().GetBool("version")
	if printVersion {
		cmd.Print(misc.BuildInfo("NeoFS Adm"))
		return nil
	}

	return cmd.Usage()
}

func initConfig(cmd *cobra.Command) {
	configFile, err := cmd.Flags().GetString(configFlag)
	if err != nil || configFile == "" {
		return
	}

	viper.SetConfigType("yml")
	viper.SetConfigFile(configFile)
	_ = viper.ReadInConfig() // if config file is set but unavailable, ignore it
}
