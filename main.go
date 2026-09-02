package main

import (
	_ "embed"
	"fmt"
	"os"
	"time"
)

//go:embed Templates.docx
var templateData []byte

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting config directory:", err)
		os.Exit(1)
	}

	outputDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting current directory:", err)
		os.Exit(1)
	}

	app := application{
		configDir: configDir,
		outputDir: outputDir,
		template:  templateData,
		now:       time.Now,
		stdout:    os.Stdout,
	}

	if err := app.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
