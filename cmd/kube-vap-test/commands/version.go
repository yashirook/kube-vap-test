package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information (set by ldflags)
var (
	Version   = "v1.31.0" // Default version, overridden by build
	Commit    = "unknown" // Git commit, set by build
	BuildDate = "unknown" // Build date, set by build
)

// NewVersionCommand creates a new version command
func NewVersionCommand() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			if short {
				fmt.Println(Version)
			} else {
				fmt.Printf("kube-vap-test version %s\n", Version)
				fmt.Printf("  commit: %s\n", Commit)
				fmt.Printf("  built: %s\n", BuildDate)
				fmt.Printf("  go: %s\n", runtime.Version())
				fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			}
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "Print version only")

	return cmd
}