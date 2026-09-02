# AGENTS.md

## Project overview

`tm` is a small Go CLI for recording timestamped front-desk shift activities and generating a Word shift report from an embedded DOCX template.

## Required behavior

- Keep the existing CLI workflow unchanged:
  - `tm "activity description"` appends an entry.
  - `tm --generate` generates the report and clears saved entries only after success.
- Keep entries in the OS user configuration directory at `tm/tasks.json`.
- Preserve the persisted JSON field names (`Time` and `Message`) so existing task files remain compatible.
- Keep timestamps and report dates in the `America/Toronto` timezone.
- Keep reports in the current working directory with the `YYYYMMDD2.docx` filename format unless a task explicitly changes it.
- Preserve the fixed content and formatting in `Templates.docx` when inserting activities.

## Engineering rules

- Use only the Go standard library unless the existing `github.com/beevik/etree` dependency is required for DOCX XML handling.
- Do not add frameworks or new third-party packages without explicit approval.
- Do not change `go.mod` or `go.sum` unless explicitly requested.
- Prefer small, focused packages and functions with explicit error returns.
- Keep `main` focused on argument handling and orchestration; separate persistence from DOCX generation and XML handling.
- Keep user-facing CLI output concise and actionable.
- Never print or inspect secrets or `.env` contents.
- Preserve user data on failures. Do not clear `tasks.json` until report generation succeeds.
- Treat `Templates.docx` as a runtime contract: table order, row indexes, and XML structure are fragile and must be validated before mutation.
- Preserve every DOCX archive entry except the intentionally updated `word/document.xml`, and close file handles before replacement on Windows.
- Validate entry counts and template structure before creating or replacing an output report. Clean up temporary files after failures.
- Avoid embedding new personal, client, or site data in source code or fixtures.

## Verification

Before committing code changes, run:

```powershell
gofmt -w (Get-ChildItem -Recurse -Filter *.go).FullName
go test ./...
go vet ./...
go build ./...
```

For report-generation changes, also run an isolated end-to-end check using a temporary user configuration directory. Verify that:

- an activity is persisted;
- the generated DOCX is valid and contains the activity;
- the report date is updated;
- saved activities are cleared only after successful generation;
- no generated report or temporary file is left in the repository.
- tests never read, overwrite, or clear the developer's real task file.

## Git hygiene

- Preserve unrelated user changes in the worktree.
- Use small conventional commits such as `docs:`, `refactor:`, `fix:`, and `test:`.
- Do not commit generated reports, binaries, caches, temporary DOCX files, or secrets.
