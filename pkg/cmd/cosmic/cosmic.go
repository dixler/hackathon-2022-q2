package main

import (
	"github.com/moby/term"
	"github.com/pulumi/pulumi/sdk/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/go/common/util/logging"
	"github.com/spf13/cobra"
)

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewCosmicCmd() *cobra.Command {
	var logFlow bool
	var logToStderr bool
	var tracing string
	var verbose int

	cmd := &cobra.Command{
		Use:   "astro",
		Short: "Pulumi astro command line",
		PersistentPreRun: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// We run this method for its side-effects. On windows, this will enable the windows terminal
			// to understand ANSI escape codes.
			_, _, _ = term.StdStreams()

			logging.InitLogging(logToStderr, verbose, logFlow)
			cmdutil.InitTracing("pulumi-cli", "pulumi", tracing)

			return nil
		}),
	}

	// Common commands:
	//     - Getting Started Commands:
	//cmd.AddCommand(newHelpCmd())
	cmd.AddCommand(newGetCmd())

	return cmd
}
