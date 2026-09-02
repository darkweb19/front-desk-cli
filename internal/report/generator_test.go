package report

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tm/internal/entries"
)

func TestGenerateUpdatesExpectedCellsAndPreservesArchive(t *testing.T) {
	template := readProjectTemplate(t)
	outputPath := filepath.Join(t.TempDir(), "report.docx")
	items := []entries.Entry{
		{
			Time:    time.Date(2026, time.January, 1, 3, 5, 0, 0, time.UTC),
			Message: "A < B & C > D",
		},
	}
	now := time.Date(2026, time.January, 1, 4, 30, 0, 0, time.UTC)

	if err := Generate(outputPath, template, items, now); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	_, _, document, err := readTemplate(generated)
	if err != nil {
		t.Fatalf("read generated report: %v", err)
	}
	tables := document.FindElements("//w:tbl")
	headerCells := tables[headerTableIndex].FindElements("./w:tr")[headerDateRowIndex].FindElements("./w:tc")
	if got := elementText(headerCells[headerDateCellIndex]); got != "Date: 31 December, 2025." {
		t.Fatalf("report date = %q", got)
	}
	activityRows := tables[activityTableIndex].FindElements("./w:tr")
	fixedCells := activityRows[fixedActivityRow].FindElements("./w:tc")
	if got := elementText(fixedCells[0]); got != fixedActivityTime {
		t.Fatalf("fixed activity time = %q", got)
	}
	if got := elementText(fixedCells[1]); got != fixedActivityMessage {
		t.Fatalf("fixed activity message = %q", got)
	}
	entryCells := activityRows[firstEntryRow].FindElements("./w:tc")
	if got := elementText(entryCells[0]); got != "22:05" {
		t.Fatalf("entry time = %q, want Toronto time 22:05", got)
	}
	if got := elementText(entryCells[1]); got != items[0].Message {
		t.Fatalf("entry message = %q", got)
	}

	assertUnchangedArchiveEntries(t, template, generated)
}

func TestGenerateRejectsTooManyEntriesWithoutReplacingReport(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "report.docx")
	previous := []byte("existing report")
	if err := os.WriteFile(outputPath, previous, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	items := make([]entries.Entry, maxEntries+1)
	err := Generate(outputPath, readProjectTemplate(t), items, time.Now())
	if err == nil {
		t.Fatal("Generate() error = nil, want capacity error")
	}
	got, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if !bytes.Equal(got, previous) {
		t.Fatalf("existing report changed after validation failure")
	}
}

func TestGenerateRejectsMalformedTemplateWithoutCreatingReport(t *testing.T) {
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "report.docx")
	if err := Generate(outputPath, []byte("not a DOCX"), nil, time.Now()); err == nil {
		t.Fatal("Generate() error = nil, want invalid template error")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("report exists after invalid template: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(outputDir, "*.tmp"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary reports left behind: %v", matches)
	}
}

func TestGenerateReplacesExistingReportWithoutTemporaryArtifacts(t *testing.T) {
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "report.docx")
	template := readProjectTemplate(t)
	firstEntries := []entries.Entry{{Time: time.Now(), Message: "first report"}}
	if err := Generate(outputPath, template, firstEntries, time.Now()); err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}

	secondEntries := []entries.Entry{{Time: time.Now(), Message: "replacement report"}}
	if err := Generate(outputPath, template, secondEntries, time.Now()); err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	_, _, document, err := readTemplate(generated)
	if err != nil {
		t.Fatalf("read replacement report: %v", err)
	}
	rows := document.FindElements("//w:tbl")[activityTableIndex].FindElements("./w:tr")
	if got := elementText(rows[firstEntryRow].FindElements("./w:tc")[1]); got != "replacement report" {
		t.Fatalf("replacement message = %q", got)
	}

	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(files) != 1 || files[0].Name() != "report.docx" {
		t.Fatalf("output directory contains unexpected artifacts: %v", files)
	}
}

func readProjectTemplate(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "Templates.docx"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	return data
}

func assertUnchangedArchiveEntries(t *testing.T, before, after []byte) {
	t.Helper()
	beforeFiles := archiveContents(t, before)
	afterFiles := archiveContents(t, after)
	if len(beforeFiles) != len(afterFiles) {
		t.Fatalf("archive entry count changed: %d -> %d", len(beforeFiles), len(afterFiles))
	}
	for name, beforeData := range beforeFiles {
		if name == "word/document.xml" {
			continue
		}
		afterData, ok := afterFiles[name]
		if !ok {
			t.Fatalf("archive entry %q missing", name)
		}
		if !bytes.Equal(beforeData, afterData) {
			t.Fatalf("archive entry %q changed", name)
		}
	}
}

func archiveContents(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open DOCX: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		contents, err := io.ReadAll(stream)
		stream.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		if _, exists := files[file.Name]; exists {
			t.Fatalf("duplicate archive entry %q", file.Name)
		}
		files[file.Name] = contents
	}
	return files
}
