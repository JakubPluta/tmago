package cmd

import (
	"context"
	"fmt"

	"github.com/JakubPluta/tmago/internal/config"
	"github.com/JakubPluta/tmago/internal/runner"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
// It runs all the tests in the given config concurrently.
// It will call either runSingle or runConcurrent for each endpoint,
// depending on whether the endpoint has concurrency configuration.
// The function will return an error if any of the calls to runSingle
// or runConcurrent return an error.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run API tests",
	RunE: func(cmd *cobra.Command, args []string) error {
		if configFile == "" {
			return fmt.Errorf("please provide config file")
		}

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		r, err := runner.NewRunner(cfg)
		if err != nil {
			return fmt.Errorf("creating runner: %w", err)
		}
		return r.Run(context.Background())
	},
}
