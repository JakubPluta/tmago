package main

import "github.com/JakubPluta/tmago/cmd"

// main is the main entry point for the application.
//
// It just calls cmd.Execute() and let Cobra handle the command-line
// flags and configuration.
func main() {
	cmd.Execute()
}
