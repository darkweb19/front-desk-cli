# Session handoff — 2026-09-02

## What was done
- Added and pushed root `AGENTS.md` in commit `993cead`.
- Split CLI orchestration into `main.go` and `app.go`.
- Moved JSON persistence into `internal/entries` with atomic writes and compatibility tests.
- Moved DOCX generation into `internal/report` with template validation and safe replacement.
- Removed the old debug-heavy `helpers/inspect.go` implementation.
- Added command, storage, timezone, capacity, DOCX integrity, and failure-preservation tests.
- Updated `README.md`, `AGENTS.md`, and `.gitignore` for the new structure.

## Decisions locked
- CLI usage remains `tm "activity"` and `tm --generate`.
- Storage remains `<os.UserConfigDir()>/tm/tasks.json` with `Time` and `Message` JSON fields.
- Reports remain in the current directory as `YYYYMMDD2.docx` using Toronto time.
- The generated 07:15 fixed activity and all embedded template content remain unchanged.
- No new dependencies or frameworks were added; existing `etree` remains for XML updates.

## Open questions
1. None.

## Next steps
1. Open one generated report in Microsoft Word and visually confirm the production layout.
2. Continue future CLI changes from the tested `application`, `entries`, and `report` boundaries.

## Gotchas
- Python and LibreOffice are unavailable here, so automated visual DOCX rendering was not possible.
- Go's race detector is unavailable because CGO is disabled; normal tests, vet, build, and smoke checks pass.
- The embedded template contains real personal, site, and client details; treat it as sensitive production content.
