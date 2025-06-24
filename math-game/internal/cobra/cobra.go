package cobra

import (
	"github.com/Anthya1104/math-game-cli/internal/config"
	"github.com/Anthya1104/math-game-cli/internal/service"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var maxRounds int

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
		logrus.Infof("Version: %s", config.Version)
	},
}

var playCmd = &cobra.Command{
	Use:   "play",
	Short: "Run math game play with input rounds",
	Run: func(cmd *cobra.Command, args []string) {
		service.StartGamePlay(maxRounds)
	},
}

func InitCLI() *cobra.Command {

	playCmd.PersistentFlags().IntVarP(&maxRounds, "rounds", "r", 1, "Max game play round") // Maximum number of rounds for the game

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(playCmd)

	return rootCmd
}

func ExecuteCmd() error {

	return InitCLI().Execute()

}
