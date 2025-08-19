package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display version, build time, and runtime information for p0-ssh-agent`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("p0-ssh-agent version %s\n", version)
			fmt.Printf("Build time: %s\n", buildTime)
			fmt.Printf("Git commit: %s\n", gitCommit)
			fmt.Printf("Go version: %s\n", runtime.Version())
			fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}

// GetVersion returns the current version
func GetVersion() string {
	return version
}

// GetBuildTime returns the build time
func GetBuildTime() string {
	return buildTime
}

// GetGitCommit returns the git commit hash
func GetGitCommit() string {
	return gitCommit
}
