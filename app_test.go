package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tm/internal/entries"
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
