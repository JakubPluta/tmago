package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFile string
	rootCmd    = &cobra.Command{
		Use:   "tmago",
		Long:  "TestMyAPI is a tool to test APIs, powered by Go and Golang.",
		Short: "API testing tool",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (required)")
	rootCmd.MarkPersistentFlagRequired("config") // wymusza podanie flagi
	rootCmd.AddCommand(runCmd)
}
