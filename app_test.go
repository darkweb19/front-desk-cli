package main

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tm/internal/entries"
	"tm/internal/updater"
)

func TestRunRequiresArgumentWithoutCreatingAppDirectory(t *testing.T) {
	configDir := t.TempDir()
	app := application{
		configDir: configDir,
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &bytes.Buffer{},
	}

	if err := app.run(nil); err == nil {
		t.Fatal("run() error = nil, want missing argument error")
	}
	if _, err := os.Stat(filepath.Join(configDir, appDirectoryName)); !os.IsNotExist(err) {
		t.Fatalf("app directory was created without a command: %v", err)
	}
}

func TestRunAddsJoinedEntry(t *testing.T) {
	configDir := t.TempDir()
	now := time.Date(2026, time.September, 2, 10, 30, 0, 0, time.FixedZone("EDT", -4*60*60))
	app := application{
		configDir: configDir,
		outputDir: t.TempDir(),
		template:  templateData,
		now:       func() time.Time { return now },
		stdout:    &bytes.Buffer{},
	}

	if err := app.run([]string{"Checked", "the", "lobby"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(items) != 1 || !items[0].Time.Equal(now) || items[0].Message != "Checked the lobby" {
		t.Fatalf("saved entries = %#v", items)
	}
}

func TestRunGenerateCreatesReportAndClearsEntries(t *testing.T) {
	configDir := t.TempDir()
	outputDir := t.TempDir()
	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	entryTime := time.Date(2026, time.September, 2, 14, 5, 0, 0, time.UTC)
	if err := store.Save([]entries.Entry{{Time: entryTime, Message: "Smoke <test> & entry"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	now := time.Date(2026, time.January, 1, 4, 30, 0, 0, time.UTC)
	var output bytes.Buffer
	app := application{
		configDir: configDir,
		outputDir: outputDir,
		template:  templateData,
		now:       func() time.Time { return now },
		stdout:    &output,
	}

	if err := app.run([]string{"--generate", "ignored"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	reportPath := filepath.Join(outputDir, "202512312.docx")
	reader, err := zip.OpenReader(reportPath)
	if err != nil {
		t.Fatalf("generated report is not a DOCX ZIP: %v", err)
	}
	reader.Close()

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("entries after generation = %#v, want empty", items)
	}
	if got := output.String(); !strings.Contains(got, "Report generated: 202512312.docx") || !strings.Contains(got, "Tasks cleared.") {
		t.Fatalf("output = %q", got)
	}
}

func TestRunGenerateFailurePreservesEntriesAndExistingReport(t *testing.T) {
	configDir := t.TempDir()
	outputDir := t.TempDir()
	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	items := make([]entries.Entry, 25)
	for index := range items {
		items[index] = entries.Entry{Time: time.Now(), Message: "activity"}
	}
	if err := store.Save(items); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	reportPath := filepath.Join(outputDir, "202609022.docx")
	previousReport := []byte("existing report")
	if err := os.WriteFile(reportPath, previousReport, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	app := application{
		configDir: configDir,
		outputDir: outputDir,
		template:  templateData,
		now:       func() time.Time { return now },
		stdout:    &bytes.Buffer{},
	}

	if err := app.run([]string{"--generate"}); err == nil {
		t.Fatal("run() error = nil, want capacity error")
	}
	remaining, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(remaining) != len(items) {
		t.Fatalf("entries after failed generation = %d, want %d", len(remaining), len(items))
	}
	gotReport, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(gotReport, previousReport) {
		t.Fatal("existing report changed after failed generation")
	}
}

func TestRunListUsesTorontoTime(t *testing.T) {
	configDir := t.TempDir()
	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	if err := store.Save([]entries.Entry{{
		Time:    time.Date(2026, time.January, 2, 20, 30, 0, 0, time.UTC),
		Message: "Checked the lobby",
	}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var output bytes.Buffer
	app := application{
		configDir: configDir,
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &output,
	}
	if err := app.run([]string{"--list"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := output.String(), "15:30 - Checked the lobby\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunEditUpdatesMostRecentMatchingTorontoTimeAndPreservesTime(t *testing.T) {
	configDir := t.TempDir()
	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	firstTime := time.Date(2026, time.January, 2, 19, 30, 1, 0, time.UTC)
	secondTime := time.Date(2026, time.July, 2, 18, 30, 59, 0, time.UTC)
	original := []entries.Entry{
		{Time: firstTime, Message: "Earlier duplicate"},
		{Time: secondTime, Message: "Most recent duplicate"},
	}
	if err := store.Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var output bytes.Buffer
	app := application{
		configDir: configDir,
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &output,
	}
	if err := app.run([]string{"--edit", "14:30", "Updated", "task"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got[0].Message != original[0].Message {
		t.Fatalf("first duplicate message = %q, want %q", got[0].Message, original[0].Message)
	}
	if got[1].Message != "Updated task" {
		t.Fatalf("most recent duplicate message = %q, want %q", got[1].Message, "Updated task")
	}
	if !got[1].Time.Equal(secondTime) {
		t.Fatalf("edited time = %v, want preserved %v", got[1].Time, secondTime)
	}
	if want := "Updated 14:30 - Updated task\n"; output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestRunEditInsertsMissingTimeInChronologicalPosition(t *testing.T) {
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	tests := []struct {
		name     string
		time     string
		wantTask []string
	}{
		{name: "before", time: "08:30", wantTask: []string{"New task", "08:55 task", "09:55 task", "11:40 task", "12:55 task"}},
		{name: "between", time: "11:30", wantTask: []string{"08:55 task", "09:55 task", "New task", "11:40 task", "12:55 task"}},
		{name: "after", time: "13:30", wantTask: []string{"08:55 task", "09:55 task", "11:40 task", "12:55 task", "New task"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := t.TempDir()
			store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
			original := []entries.Entry{
				{Time: time.Date(2026, time.September, 3, 8, 55, 0, 0, location), Message: "08:55 task"},
				{Time: time.Date(2026, time.September, 3, 9, 55, 0, 0, location), Message: "09:55 task"},
				{Time: time.Date(2026, time.September, 3, 11, 40, 0, 0, location), Message: "11:40 task"},
				{Time: time.Date(2026, time.September, 3, 12, 55, 0, 0, location), Message: "12:55 task"},
			}
			if err := store.Save(original); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			var output bytes.Buffer
			app := application{
				configDir: configDir,
				outputDir: t.TempDir(),
				template:  templateData,
				now: func() time.Time {
					return time.Date(2026, time.September, 3, 15, 0, 0, 0, location)
				},
				stdout: &output,
			}
			if err := app.run([]string{"--edit", test.time, "New", "task"}); err != nil {
				t.Fatalf("run() error = %v", err)
			}

			got, err := store.Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(got) != len(test.wantTask) {
				t.Fatalf("saved entries = %d, want %d", len(got), len(test.wantTask))
			}
			for index, want := range test.wantTask {
				if got[index].Message != want {
					t.Fatalf("entry %d message = %q, want %q", index, got[index].Message, want)
				}
			}

			requestedTime, err := time.Parse("15:04", test.time)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			var added entries.Entry
			for _, item := range got {
				if item.Message == "New task" {
					added = item
					break
				}
			}
			addedToronto := added.Time.In(location)
			if addedToronto.Year() != 2026 || addedToronto.Month() != time.September || addedToronto.Day() != 3 || addedToronto.Hour() != requestedTime.Hour() || addedToronto.Minute() != requestedTime.Minute() {
				t.Fatalf("added time = %v, want 2026-09-03 %s Toronto", addedToronto, test.time)
			}
			if want := "Added " + test.time + " - New task\n"; output.String() != want {
				t.Fatalf("output = %q, want %q", output.String(), want)
			}
		})
	}
}

func TestRunEditRejectsInvalidTime(t *testing.T) {
	configDir := t.TempDir()
	store := entries.Store{Path: filepath.Join(configDir, appDirectoryName, "tasks.json")}
	entry := entries.Entry{Time: time.Date(2026, time.July, 2, 18, 30, 0, 0, time.UTC), Message: "Original"}
	if err := store.Save([]entries.Entry{entry}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	app := application{
		configDir: configDir,
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &bytes.Buffer{},
	}

	if err := app.run([]string{"--edit", "2:30", "Changed"}); err == nil || !strings.Contains(err.Error(), "24-hour format") {
		t.Fatalf("invalid time error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 || got[0].Message != entry.Message || !got[0].Time.Equal(entry.Time) {
		t.Fatalf("entry changed after rejected edit: %#v", got)
	}
}

func TestRunHelpDocumentsEdit(t *testing.T) {
	var output bytes.Buffer
	app := application{
		configDir: t.TempDir(),
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &output,
	}
	if err := app.run([]string{"--help"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "tm --edit HH:MM <new task>") || !strings.Contains(got, "tm --upgrade") || !strings.Contains(got, "tm --version") {
		t.Fatalf("help output = %q", got)
	}
}

func TestRunUpgradeUsesCurrentVersion(t *testing.T) {
	var output bytes.Buffer
	var gotVersion string
	app := application{
		configDir: t.TempDir(),
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &output,
		version:   "v1.0.0",
		upgrade: func(_ context.Context, currentVersion string) (updater.Result, error) {
			gotVersion = currentVersion
			return updater.Result{FromVersion: currentVersion, ToVersion: "v1.1.0", Updated: true, Deferred: true}, nil
		},
	}

	if err := app.run([]string{"--upgrade"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if gotVersion != "v1.0.0" {
		t.Fatalf("upgrade current version = %q, want v1.0.0", gotVersion)
	}
	if got := output.String(); got != "Upgrade downloaded: v1.1.0 (installs after tm exits)\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRunVersion(t *testing.T) {
	var output bytes.Buffer
	app := application{
		configDir: t.TempDir(),
		outputDir: t.TempDir(),
		template:  templateData,
		now:       time.Now,
		stdout:    &output,
		version:   "v1.2.3",
	}

	if err := app.run([]string{"--version"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := output.String(); got != "v1.2.3\n" {
		t.Fatalf("output = %q, want v1.2.3", got)
	}
}
