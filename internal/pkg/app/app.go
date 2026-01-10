package app

import "github.com/stakater/Reloader/internal/pkg/cmd"

// Run runs the command
func Run() error {
	rootCmd := cmd.NewReloaderCommand()
	return rootCmd.Execute()
}
