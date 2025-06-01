package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	
	"github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands"
)

var (
	// Global options
	globalOpts = struct {
		OutputFormat   string
		Quiet          bool
		Verbose        bool
		KubeconfigPath string
	}{}

	// Run command options
	runOpts = &commands.RunOptions{}

	// Check command options
	checkOpts = &commands.CheckOptions{}
)

// rootCmd represents the application's root command
var rootCmd = &cobra.Command{
	Use:   "vaptest",
	Short: "ValidatingAdmissionPolicy Test Tool",
	Long:  `A tool for testing Kubernetes ValidatingAdmissionPolicy`,
	// Don't show usage on errors
	SilenceUsage: true,
	// Prevent duplicate error messages
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Run `vaptest --help` for usage")
	},
}

func init() {
	// Set default kubeconfig path
	home, err := os.UserHomeDir()
	if err == nil {
		globalOpts.KubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&globalOpts.OutputFormat, "output", "table", "Output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolVar(&globalOpts.Quiet, "quiet", false, "Suppress progress information")
	rootCmd.PersistentFlags().StringVar(&globalOpts.KubeconfigPath, "kubeconfig", globalOpts.KubeconfigPath, "Path to kubeconfig file (default: \"~/.kube/config\")")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.Verbose, "verbose", "v", false, "Show detailed output")

	// Copy global options to each command's options
	cobra.OnInitialize(func() {
		// Run command options
		runOpts.OutputFormat = globalOpts.OutputFormat
		runOpts.Quiet = globalOpts.Quiet
		runOpts.Verbose = globalOpts.Verbose
		runOpts.Kubeconfig = globalOpts.KubeconfigPath

		// Check command options
		checkOpts.OutputFormat = globalOpts.OutputFormat
		checkOpts.Quiet = globalOpts.Quiet
		checkOpts.Verbose = globalOpts.Verbose
		checkOpts.Kubeconfig = globalOpts.KubeconfigPath

	})

	// Add subcommands
	rootCmd.AddCommand(commands.NewRunCommand(runOpts))
	rootCmd.AddCommand(commands.NewCheckCommand(checkOpts))
	rootCmd.AddCommand(commands.NewVersionCommand())
}

func main() {
	// Since SilenceErrors is set to true,
	// error messages are displayed by PrintError in each command,
	// and won't be duplicated here
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}