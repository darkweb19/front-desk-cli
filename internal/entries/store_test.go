package entries

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "tasks.json")}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Load() = %#v, want non-nil empty slice", got)
	}
}

func TestSaveAndLoadRoundTripWithCompatibleSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tm", "tasks.json")
	store := Store{Path: path}
	want := []Entry{
		{
			Time:    time.Date(2026, time.September, 2, 9, 15, 0, 0, time.FixedZone("EDT", -4*60*60)),
			Message: "Checked the front desk handover.",
		},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != len(want) || !got[0].Time.Equal(want[0].Time) || got[0].Message != want[0].Message {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("saved JSON is invalid: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("saved record count = %d, want 1", len(records))
	}
	if len(records[0]) != 2 || records[0]["Time"] == nil || records[0]["Message"] == nil {
		t.Fatalf("saved fields = %v, want exactly Time and Message", records[0])
	}
}

func TestLoadMalformedJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte(`[{"Time":`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := (Store{Path: path}).Load(); err == nil {
		t.Fatal("Load() error = nil, want malformed JSON error")
	}
}

func TestClearWritesEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	store := Store{Path: path}
	if err := store.Save([]Entry{{Time: time.Now(), Message: "activity"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Load() after Clear() = %#v, want non-nil empty slice", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "[]\n" {
		t.Fatalf("cleared file = %q, want %q", data, "[]\\n")
	}
}
