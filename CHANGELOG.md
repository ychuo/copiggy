# Changelog

## 0.1.2 — 2026-10-24

### Added

- Dedicated App scheme and Markdown cards bring the current presentation types to 16, with matching native previews and four-language controls.
- Recognize bare domains, explicit App schemes and standalone Markdown links. Web links offer Page / QR; App schemes open in QR with a Link view for the exact source.
- Generate scannable QR codes locally inside an opened preview. Switching Page / QR retains the loaded webpage for that preview session.
- Read structured Markdown locally, including lists, quotes, code and tables, or switch to its exact source. No remote image loading or HTML execution.
- Expand bounded recognition for coordinates, colors, dates, timestamps, email wrappers and quoted file paths, and support URL-only clipboard representations. Bare four- and six-digit numbers still require a label to become verification codes.

### Fixed

- Preserve the complete clipboard selection when web/App URIs and file references appear in separate clipboard items, rather than copying only its file subset.
- Restore older mixed-URI history records compatibly while retaining their original representations, metadata and protection scope.
- Keep plain path/URL lists out of Markdown classification and preserve escaped text and complete entity references in Markdown reading.

All preview modes retain the original saved content and its allowed applications; presentation-only switches do not write the clipboard or add history.
