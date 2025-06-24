package cobra

import (
	"github.com/Anthya1104/raid-simulator/internal/config"
	"github.com/Anthya1104/raid-simulator/internal/raid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var raidType string
var inputData string

var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "A base CLI app with Cobra and logrus",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Info("Hello from the base CLI app!")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("Version: %s", config.Version)
	},
}

var raidCmd = &cobra.Command{
	Use:   "raid",
	Short: "Run RAID simulation (raid0, raid1, ...)",
	Run: func(cmd *cobra.Command, args []string) {
		if raidType == "" || inputData == "" {
			logrus.Error("Please provide --type and --data flags")
			return
		}
		raid.RunRAIDSimulation(raid.RaidType(raidType), inputData)
	},
}

func InitCLI() *cobra.Command {
	raidCmd.Flags().StringVar(&raidType, "type", "", "RAID type (e.g. raid0)")
	raidCmd.Flags().StringVar(&inputData, "data", "", "Input data to write into RAID")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(raidCmd)

	return rootCmd
}

func ExecuteCmd() error {

	return InitCLI().Execute()

}
