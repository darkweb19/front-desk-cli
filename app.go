package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tm/internal/entries"
	"tm/internal/report"
	"tm/internal/updater"
)

const appDirectoryName = "tm"

type application struct {
	configDir string
	outputDir string
	template  []byte
	now       func() time.Time
	stdout    io.Writer
	version   string
	upgrade   func(context.Context, string) (updater.Result, error)
}

func (app application) run(args []string) error {
	if len(args) == 0 {
		return errors.New("please provide an argument")
	}

	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(app.stdout, "Usage:")
		fmt.Fprintln(app.stdout, "  tm <task message>")
		fmt.Fprintln(app.stdout, "  tm --list")
		fmt.Fprintln(app.stdout, "  tm --edit HH:MM <new task>")
		fmt.Fprintln(app.stdout, "  tm --generate")
		fmt.Fprintln(app.stdout, "  tm --clear")
		fmt.Fprintln(app.stdout, "  tm --upgrade")
		fmt.Fprintln(app.stdout, "  tm --version")
		return nil
	}

	if args[0] == "--upgrade" || args[0] == "-u" {
		return app.upgradeVersion()
	}

	if args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintln(app.stdout, app.version)
		return nil
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
		return app.listTasks(store)
	}

	if args[0] == "--edit" || args[0] == "-e" {
		return app.editTask(store, args[1:])
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
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return fmt.Errorf("error loading Toronto timezone: %w", err)
	}
	for _, item := range items {
		fmt.Fprintln(app.stdout, item.Time.In(location).Format("15:04")+" - "+item.Message)
	}
	return nil
}

func (app application) editTask(store entries.Store, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: tm --edit HH:MM <new task>")
	}

	requestedTime, err := time.Parse("15:04", args[0])
	if err != nil || requestedTime.Format("15:04") != args[0] {
		return fmt.Errorf("invalid time %q: use HH:MM in 24-hour format", args[0])
	}
	message := strings.TrimSpace(strings.Join(args[1:], " "))
	if message == "" {
		return errors.New("new task cannot be empty")
	}

	items, err := store.Load()
	if err != nil {
		return fmt.Errorf("error loading entries: %w", err)
	}
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return fmt.Errorf("error loading Toronto timezone: %w", err)
	}

	for index := len(items) - 1; index >= 0; index-- {
		entryTime := items[index].Time.In(location)
		if entryTime.Hour() != requestedTime.Hour() || entryTime.Minute() != requestedTime.Minute() {
			continue
		}

		items[index].Message = message
		if err := store.Save(items); err != nil {
			return fmt.Errorf("error saving entries: %w", err)
		}
		fmt.Fprintf(app.stdout, "Updated %s - %s\n", args[0], message)
		return nil
	}

	return fmt.Errorf("no task found at %s Toronto time", args[0])
}

func (app application) clearTasks(store entries.Store) error {
	if err := store.Clear(); err != nil {
		return fmt.Errorf("error clearing tasks: %w", err)
	}
	fmt.Fprintln(app.stdout, "Tasks cleared.")
	return nil
}

func (app application) upgradeVersion() error {
	if app.upgrade == nil {
		return errors.New("upgrade is unavailable")
	}

	result, err := app.upgrade(context.Background(), app.version)
	if err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}
	if !result.Updated {
		fmt.Fprintln(app.stdout, "Already up to date:", result.ToVersion)
		return nil
	}
	if result.Deferred {
		fmt.Fprintln(app.stdout, "Upgrade downloaded:", result.ToVersion, "(installs after tm exits)")
		return nil
	}

	fmt.Fprintln(app.stdout, "Upgraded to:", result.ToVersion)
	return nil
}
