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
	if args[0] == "--generate" || args[0] == "-g" {
		return app.generateReport(store)
	}

	if args[0] == "--clear" || args[0] == "-c" {
		return app.clearTasks(store)
	}

	if args[0] == "--list" || args[0] == "-l" {
		// print all the tasks in the json printed as this format 14:55 - Task Message
		return app.listTasks(store)
	}

	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(app.stdout, "Usage: tm [--generate] [--clear] [--list] [task message]")
		return nil
	}

	if args[0] == "--upgrade" || args[0] == "-u" {
		return app.upgradeVersion(store)
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


func (app application) listTasks(store entries.Store) error {
	items, err := store.Load()
	if err != nil {
		return fmt.Errorf("error loading entries: %w", err)
	}
	for _, item := range items {
		fmt.Fprintln(app.stdout, item.Time.Format("15:04") + " - " + item.Message)
	}
	return nil
}

func (app application) clearTasks(store entries.Store) error {
	if err := store.Clear(); err != nil {
		return fmt.Errorf("error clearing tasks: %w", err)
	}
	fmt.Fprintln(app.stdout, "Tasks cleared.")
	return nil
}

func (app application) upgradeVersion(store entries.Store) error {
	_, err := store.Load()
	if err != nil {
		return fmt.Errorf("error loading entries: %w", err)
	}
	fmt.Println("Upgrading is cooking")
	// Implement version upgrade logic here
	return nil
}