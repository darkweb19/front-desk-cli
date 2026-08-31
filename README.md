# tm — Shift Report CLI

`tm` is a lightweight Go command-line tool for logging shift activities throughout the day and generating a daily shift report from those entries.

The goal is to make shift reporting faster: instead of manually remembering and typing everything at the end of a shift, entries are recorded as they happen.

## Usage

Add a new shift entry:

```bash
tm "Checked emails and reviewed previous shift reports"
```

Example:

```bash
tm "Resident from Unit 1204 picked up a package"
```

Each entry is automatically recorded with the current timestamp.

Example:

```text
07:15  Checked emails and reviewed previous shift reports
```

Generate the shift report:

```bash
tm --generate
```

## How It Works

When you run:

```bash
tm "Completed parking patrol"
```

`tm`:

1. Reads the existing entries from local storage.
2. Creates a new entry with the current time.
3. Adds the provided description.
4. Saves all entries back to the local JSON file.

The internal data structure is:

```go
type Entry struct {
    Time    time.Time
    Message string
}
```

Entries are persisted between terminal sessions, so closing the terminal does not remove previously recorded tasks.

## Local Storage

`tm` stores its data in the operating system's user configuration directory.

For example, on Linux or GitHub Codespaces:

```text
~/.config/tm/tasks.json
```

Example data:

```json
[
  {
    "Time": "2026-08-28T15:38:27.515167348Z",
    "Message": "Checked emails and previous shift reports"
  }
]
```

The displayed time uses 24-hour format:

```text
15:38
```

## Commands

### Add an Entry

```bash
tm "description"
```

Example:

```bash
tm "Received delivery for Unit 801"
```

### Generate Report

```bash
tm --generate
```

This loads the stored entries and generates the shift report.

## Workflow

```text
During shift
     ↓
tm "task description"
     ↓
tasks.json
     ↓
tm --generate
     ↓
Word shift-report template
     ↓
Completed daily shift report
```

The generated report will contain a two-column activity table:

| Time  | Description                               |
| ----- | ----------------------------------------- |
| 07:15 | Checked emails and previous shift reports |
| 08:30 | Received delivery for Unit 801            |
| 10:20 | Completed parking patrol                  |

The existing fixed entries in the shift-report template remain unchanged while recorded activities are inserted into the appropriate section.

## Technology

The CLI is written in Go and currently uses Go's standard library, including:

```text
os
encoding/json
time
strings
path/filepath
```

No database is required.

## Development

Clone the repository:

```bash
git clone <repository-url>
cd front-desk-cli
```

Run during development:

```bash
go run . "Test shift entry"
```

Generate the report:

```bash
go run . --generate
```

Build the executable:

```bash
go build -o tm
```

For Windows:

```bash
go build -o tm.exe
```

Once the executable is available in your system `PATH`, you can run it from any terminal:

```bash
tm "Completed lobby patrol"
```

## Current Status

Implemented:

* Command-line entry logging
* Automatic timestamps
* JSON persistence
* Loading previous entries
* User configuration directory storage
* `--generate` command handling

In progress:

* Word template manipulation
* Automatic report date updates
* Injecting shift entries into the report table
* Final Windows executable workflow

## Why This Exists

Shift reports often involve writing down small events throughout the day and recreating them manually at the end of the shift.

`tm` simplifies that workflow to:

```bash
tm "what happened"
```

throughout the day, followed by:

```bash
tm --generate
```

at the end of the shift.
