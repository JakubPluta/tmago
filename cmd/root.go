package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "tmago",
		Long:  "TestMyAPI is a tool to test APIs, powered by Go and Golang.",
		Short: "API testing tool",
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// init initializes the root command with a required --config flag and adds the run command to it.
func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (required)")
	rootCmd.MarkPersistentFlagRequired("config")
	rootCmd.AddCommand(runCmd)
}
