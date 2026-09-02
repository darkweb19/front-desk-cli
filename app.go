package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tm/internal/entries"
	"tm/internal/report"
)

const appDirectoryName = "tm"

type application struct {
	configDir string
	outputDir string
	template  []byte
	now       func() time.Time
	stdout    io.Writer
}

func (app application) run(args []string) error {
	if len(args) == 0 {
		return errors.New("please provide an argument")
	}

	appDir := filepath.Join(app.configDir, appDirectoryName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	store := entries.Store{Path: filepath.Join(appDir, "tasks.json")}
	if args[0] == "--generate" {
		return app.generateReport(store)
	}

	items, err := store.Load()
	if err != nil {
		return fmt.Errorf("error loading entries: %w", err)
	}

	items = append(items, entries.Entry{
		Time:    app.now(),
		Message: strings.Join(args, " "),
	})

	if err := store.Save(items); err != nil {
		return fmt.Errorf("error saving entries: %w", err)
	}

	fmt.Fprintln(app.stdout, items)
	return nil
}

func (app application) generateReport(store entries.Store) error {
	items, err := store.Load()
	if err != nil {
		return fmt.Errorf("error loading entries: %w", err)
	}

	now := app.now()
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return fmt.Errorf("error loading Toronto timezone: %w", err)
	}

	filename := now.In(location).Format("20060102") + "2.docx"
	outputPath := filepath.Join(app.outputDir, filename)
	if err := report.Generate(outputPath, app.template, items, now); err != nil {
		return fmt.Errorf("error generating report: %w", err)
	}

	fmt.Fprintln(app.stdout, "Report generated:", filename)
	if err := store.Clear(); err != nil {
		return fmt.Errorf("report generated, but could not clear tasks: %w", err)
	}

	fmt.Fprintln(app.stdout, "Tasks cleared.")
	return nil
}
