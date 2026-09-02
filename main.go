package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"tm/internal/updater"
)

//go:embed Templates.docx
var templateData []byte

var version = "dev"

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
		version:   version,
		upgrade: func(ctx context.Context, currentVersion string) (updater.Result, error) {
			return updater.New("darkweb19", "front-desk-cli").Upgrade(ctx, currentVersion)
		},
	}

	if err := app.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
