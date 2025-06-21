package cobra

import (
	"github.com/Anthya1104/math-game-cli/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var maxRoundsFlag int

var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "A math game CLI application",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Debugf("Hello from the base CLI app!")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info",
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Infof("Version: %s\n", config.Version)
	},
}

func InitCLI() *cobra.Command {

	rootCmd.PersistentFlags().IntVarP(&maxRoundsFlag, "rounds", "r", 1, "Max game play round") // Maximum number of rounds for the game
	rootCmd.AddCommand(versionCmd)

	return rootCmd
}

func ExecuteCmd() error {

	return InitCLI().Execute()

}

// GetMaxRoundsFlag returns the value of the --rounds CLI flag.
func GetMaxRoundsFlag() int {
	return maxRoundsFlag
}
