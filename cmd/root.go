package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information (set during build)
	Version   = "0.1.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "agentenv",
	Short: "Multi-agent development environment tool",
	Long: `agentenv is a CLI tool for running multiple isolated development environments from git worktrees.

It enables running multiple LLM coding agents (Claude, Codex, etc.) simultaneously, each with their own:
- Git worktree and branch
- Docker services with unique ports
- Database instance
- Environment configuration`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Add version command
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("agentenv version %s\n", Version)
			fmt.Printf("Build date: %s\n", BuildDate)
			fmt.Printf("Git commit: %s\n", GitCommit)
		},
	}
	rootCmd.AddCommand(versionCmd)
}
