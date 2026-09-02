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

Review or correct saved entries:

```bash
tm --list
tm --edit 14:30 "Corrected activity description"
```

`--edit` matches the most recent entry at that Toronto time and changes only
its description. The original timestamp is preserved.

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

### Maintain Entries

```bash
tm --list
tm --edit 14:30 "Corrected activity description"
tm --clear
```

### Upgrade

```bash
tm --version
tm --upgrade
```

Upgrades are downloaded from the latest GitHub release and verified against
the published SHA-256 checksum before replacing the installed executable.

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

The CLI is written in Go. It uses the standard library for command handling,
JSON persistence, ZIP processing, and filesystem operations:

```text
os
encoding/json
time
strings
path/filepath
```

It also uses the existing `github.com/beevik/etree` dependency for targeted
WordprocessingML updates inside the DOCX template. No database is required.

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

### Install from a GitHub Release

Windows PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/darkweb19/front-desk-cli/main/install.ps1 -OutFile install.ps1
.\install.ps1
```

The script installs `tm.exe` under `%LOCALAPPDATA%\Programs\tm` and adds that
directory to the user PATH when necessary.

Linux or macOS:

```bash
curl -fsSLO https://raw.githubusercontent.com/darkweb19/front-desk-cli/main/install.sh
sh install.sh
```

The script detects the OS and architecture, verifies the release checksum, and
installs `tm` under `~/.local/bin`.

### Publish a Release

Pushing a tag beginning with `v` verifies the project, cross-compiles supported
binaries, creates `SHA256SUMS`, and publishes a GitHub release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Current Status

Implemented:

* Command-line entry logging
* Automatic Toronto timestamps in generated reports
* JSON persistence in the user configuration directory
* Embedded Word template processing
* Automatic report date updates
* Activity-table population
* Windows-compatible file replacement
* Toronto-time task listing and editing
* Checksum-verified native installation and self-upgrades
* Tagged GitHub release automation

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
