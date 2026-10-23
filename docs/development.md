# Development

Copiggy is a single Go module. `main.go` starts the application; all product UI is rendered through MyGo Native UI.

```text
main.go
mygo.json       App identity, version, macOS packaging and icon configuration
go.mod / go.sum
internal/
  app/         Composition, lifecycle, serialized commands and platform adapters
    nativeui/  Native views, menus, localization and presentation caches
  clipboard/   History transactions, capture, trusted copy and local enhancements
  clipmodel/   Records, exact representations, deduplication and retention rules
  contentactions/ Pure derivation of explicit mail, phone, map, file and calendar actions
  settings/    Preference values, defaults and validation
  storage/     Versioned JSON repositories, legacy migration and atomic FIFO writes
  linkicons/   Restricted-origin HTTP icon retrieval and decoded image cache
  linkmetadata/ Bounded copied-HTML parsing and public-page title/cover retrieval
  macos/
    pasteboard/ Native observation and verified clipboard writes
    guard/      Revision-bound policy, protected-copy requests and native tap
    appkit/     Source icons, NSMenu, glass, popover appearance and MapKit snapshots
    internal/objc/ Shared low-level runtime helpers
resources/
  resources.go Embedded runtime artwork, fonts and catalogs
  brand/       Approved PNG artwork, menu-bar templates and Copiggy.icon source
  fonts/       Inter and regional Noto Sans faces, provenance and licenses
  icons/       SVG icons and license
  flags/       Embedded numbering-region SVG flags, provenance and license
  licenses/    Upstream numbering and geocoding metadata license
  locales/     Multilingual content-type search and recognition labels
scripts/
  env.sh       Shared pinned toolchain, cache and production environment
  go.sh        Go command wrapper
  check.sh     Headless tests, static checks and production binary
  package.sh   Staged MyGo bundle, metadata verification and signing
  release.sh   Versioned ZIP and checksum for GitHub uploads
  run.sh       Package, then open the official application bundle
build/         Generated output; ignored
  Copiggy.app  Official application bundle and launch path
  check/       Binary produced by check.sh
  releases/    Release attachments grouped by version (v0.1.1, etc.)
  archive/     Retained older local outputs; not used by build scripts
  .staging/    Temporary packaging directories, cleaned after each run
.cache/        Toolchain, modules and build cache; ignored
```

## Responsibilities and threading

`main.go` calls `app.Run`. The app composes services and native adapters; `nativeui` renders MyGo Native UI and owns only transient presentation state. `clipboard.History` owns records and their exact representations together, so the UI cannot maintain a second copy of business state. Copy commands accept a trusted record ID plus an allowlisted format, or a validated draft ID; the copy service derives content and allowed apps itself. UI-provided replacement text or policy is never authoritative. Successful variant/draft writes leave the original history record unchanged and do not append another record; consuming a draft only clears that transient work.

Business commands run in order on an application worker. Native clipboard reads/writes and window operations execute on the AppKit thread; representation parsing, PNG validation, history encoding and disk I/O execute away from it. Queries read the last committed snapshot while disk writes are pending. Enhancement cancellation and image completions use the UI generation to reject obsolete results. Image decoding has two concurrent slots and a bounded bitmap cache. Quitting closes command admission, drains accepted work and flushes persistence before terminating the event loop.

`storage` owns file formats and atomic writes. It never decides which record is current or protected. `clipmodel` owns pure record rules, and `settings` owns preference defaults. `linkicons` has no MyGo dependency and returns ordinary Go images; the UI alone creates and caches bitmaps. Its entry point rejects protected records before any cache or network lookup.

`clipmodel.AnalyzeInRegion` derives typed display values from captured text and the resolved phone region; saved records use `AnalyzeItem` so preference changes cannot reinterpret existing numbers. Explicit HEX/RGB/HSL, dialing-number and timestamp-unit choices select a session-only copy format; `clipmodel.CopyText` derives its plain text again from the trusted saved item. Opening preview alone keeps the default copy behavior. Finder file references always copy as full path text, while other records copy their exact original representations unless a variant is selected. Capturing and previewing file references never modify the system clipboard or read file contents. UTC/local affects only the date explanation; milliseconds-to-seconds output truncates subsecond digits while saved milliseconds remain intact. Derived formats and masked strings never replace persisted clipboard authority. Restore reclassifies legacy text records while retaining their exact representations and scope. `contentactions` derives external targets from trusted saved records and does no I/O; the UI launches them only after an explicit action. Calendar exports use a private temporary file and the system calendar import flow. Format choices and sensitive reveal state belong to one preview session and clear on close or navigation. Reveal and zoom do not select a copy format; changing any preview control alone never writes the clipboard.

Phone numbering and geographic descriptions use embedded libphonenumber metadata. Recognition takes an explicit international prefix or the saved regional context, independently of interface language. Preview shows an embedded SVG flag and may load a city, province or state description asynchronously; the result belongs to the current preview and language, and stale completions are discarded. Detailed geocoding never runs during capture or card rendering, makes no network requests, and describes the numbering area rather than the subscriber's current location.

MapKit snapshots use at most three concurrent native requests/decodes. Visible unrestricted cards are checked against the trusted store again before starting work, and completions check cache identity and record scope. Preview/card snapshots and link metadata share a bounded 24-entry/24 MiB memory cache. Hiding or quitting cancels pending work; map rendering preserves system attribution. Native integration remains manually verified, while pure validation, cancellation and cache behavior are tested in memory.

Link metadata retrieval uses at most three workers and starts only for visible or nearby unrestricted public cards, never during capture. It prefers matching copied HTML; remote retrieval requests the saved path and query with the fragment removed, then parses Open Graph/Twitter metadata and fetches a supported cover. Requests carry no browser cookies, authentication or referrers and execute no scripts. Private/local destinations and protected records are excluded. Authority is rechecked before work starts and before results reach the UI. Successful covers expire after six hours; failures and coverless results after ten minutes. The cache is memory-only. Website icons retain their separate origin-only policy.

Tests live beside their responsibility: domain/content rules in `clipmodel`, transaction/guard-observation tests in `clipboard`, file compatibility and failure tests in `storage`, and in-memory native rendering tests in `app/nativeui`. Native adapters retain separate initialization and failure boundaries. A popover appearance handle is an owned object with an idempotent `Close`; raw Objective-C ownership does not escape `appkit`.

## Commands

Run commands from the project root:

```sh
./scripts/check.sh
./scripts/package.sh
./scripts/run.sh
./scripts/release.sh
```

`scripts/env.sh`, shared by every entry point, pins Go 1.27.1, disables cgo and VCS stamping, and puts `GOPATH` and `GOCACHE` under `.cache/`. It also works from a source folder without Git metadata. Use it for focused checks as well:

```sh
./scripts/go.sh test ./internal/clipmodel
./scripts/go.sh test ./internal/app/nativeui -run TestNativePanelKeyboardAndCardGeometry
```

The default check runs headless tests, MyGo vet (including `go vet`), source-artwork parity and a production build. It does not launch the application, change the system clipboard, post keyboard events or generate screenshots. Native callback tests that inspect desktop state are gated by the `integration` build tag; do not enable them as a routine check. Visual appearance, real clipboard behavior, protected-copy shortcuts and webpage interaction are verified manually by the user. After a change, build and restart the app rather than driving those flows automatically.

## Localization

Product source strings and catalog keys are concise English. `internal/app/nativeui/i18n.go` supplies English, Simplified Chinese, Traditional Chinese and Japanese copy for controls, hints, status and accessibility labels. Keep all four translations and format arguments together when adding copy. Default tests check catalog coverage and reject hardcoded CJK runtime strings outside translation resources.

Typed capture/model/platform errors map to short localized UI messages. Detailed diagnostics belong in logs, not the menu bar. Actual clipboard content and source app names are never translated. `resources/locales/search_terms.json` keeps content-type search available in every supported language, independently of the interface language.

## Native boundaries

The tray hosts a MyGo surface inside `NSGlassEffectView` when available, with `NSVisualEffectView` as the older-system fallback. AppKit owns the preview `NSPopover`, including its arrow and placement. Native app controls, toolbar, loading states and error states stay in MyGo.

The menu-bar settings use a system `NSMenu` with template SF Symbol icons and native key equivalents. The adapter consumes the same localized menu model as the MyGo fallback, and returns the selected ID after tracking ends. AppKit controls menu typography, spacing and selection; rebuilding on each open reflects current settings. The panel shortcut remains registered globally once.

A link preview creates a system `WKWebView` only after the user opens that preview. It loads the saved HTTP/HTTPS address and has no application bindings. Automatic downloads and new windows are denied. It uses WebKit's normal persistent website data store, not a private browsing session. Closing the preview destroys its webpage view.

Builds set `-X github.com/egoist/mygo.production=1` and `mygo_noinspector`. Keep that production flag when changing packaging: it prevents a loopback webpage from being treated as a trusted development frontend.

## Packaging and local data

`mygo.json` is the single source of app identity, version and macOS packaging options. It intentionally has no web frontend command, frontend development server or JavaScript bindings. The pinned MyGo CLI is a Go tool dependency, so the framework and CLI versions stay aligned.

`scripts/package.sh` uses the CLI to create a production bundle in a temporary directory, then normalizes the executable to `copiggy`, verifies metadata and icon resources, removes duplicated embedded artwork and fonts, retains licenses and signs the result again. Only a verified bundle replaces `build/Copiggy.app`; a failed build preserves the previous bundle. The identifier remains `dev.copiggy`. `run.sh` opens that official bundle through Launch Services.

`scripts/release.sh` packages the app, then creates `Copiggy-v<version>-macos-<arch>.zip` and `SHA256SUMS.txt` together in `build/releases/v<version>/`. The version comes from `mygo.json`. It verifies the archive before publishing the directory and refuses to replace an existing version, preserving previously prepared attachments. Upload both files to GitHub manually; the script does not create a tag or publish a release. Packaging intermediates stay in `build/.staging/`; older local outputs may be retained under `build/archive/`. Keep release archives out of the build root.

Packaging requires full Xcode 26 or later for Icon Composer compilation; `DEVELOPER_DIR` is set only for the command when the standard Xcode location is available. The project does not change `xcode-select`. `resources/brand/Copiggy.icon` uses an exact copy of the approved `app-icon.png`, with warm light/dark backgrounds and no added glass, gloss or shadow on the artwork. Apple’s asset compiler produces both `Assets.car` and the ICNS fallback. Bundled launches retain that system icon; only a bare development executable calls `Dock.SetIcon` with the PNG. The application itself still targets macOS 13 or later.

The menu-bar pig and clipboard are drawn from native vector paths in `internal/app/nativeui/tray_icon.go`. Their 72px transparent PNG exports in `resources/brand/` must match the runtime templates; `TestTrayBrandAssetsMatchRuntimeTemplates` checks parity. MyGo sets their height to 18pt, and AppKit handles the light/dark template tint. Keep protection indicators tied to the live guard, not the saved preference.

The empty-history mascot uses two embedded PNG frames and three native rounded strokes aligned to the clipboard's tilt. Native timers schedule the occasional blink and animate each stroke only during its reveal or fade, sleeping through pauses. The empty pinned list uses two aligned pin-press poses with a long rest between gestures. Reduce Motion holds a still frame without timers. Search with no results retains its semantic icon. Runtime artwork lives only in `resources/brand/`; shared visual rules are documented in the [design system](design/design-system.md).

Changing the bundle location or rebuilding an ad-hoc-signed binary can require the user to refresh Accessibility permission. Do not modify the TCC database or automate authentication.

History and preferences remain in `~/Library/Application Support/Copiggy/`. `history.json` is a versioned, atomic snapshot of ordinary and pinned records and their exact representations and recognition regions; `preferences.json` holds language, appearance, phone region, recording, desired Paste Guard state and retention. First launch imports the old `pinned.json` only when the unified history file is absent. Tests use temporary fixtures; never copy real clipboard contents into repository fixtures or documentation.

Preference changes enqueue one complete immutable snapshot and report actual save failures. Startup restores them before creating the panel or enabling recording/guard behavior. Explicit `light` or `dark` takes precedence over the system appearance; older language-only files remain supported. Failed saves show localized status instead of silently claiming success. UI tests inject a preference-saving port; repository tests use temporary paths.

### Persistence boundaries

| State | Behavior |
| --- | --- |
| Language, appearance, phone region, pause, desired guard and retention | Stored in `preferences.json` |
| Ordinary and pinned records, recognition region, provenance, time and allowed apps | Stored in `history.json`; pins are exempt from age expiry |
| History limits | 100 ordinary / 100 pinned records, 64 MiB shared content; ordinary age limit 1/7/30 days or none |
| Current clipboard revision and active protection binding | Memory only; never restored as authority over a newer clipboard |
| Search, pin filter, selection, scroll, preview and copy-format choices | Session-only |
| Unconfirmed enhancement drafts | Discarded on exit; never applied automatically |
| Website icons, link metadata, map snapshots and decoded images | Bounded memory caches, rebuilt as needed |
| Webpage cookies and website data | WebKit's independent persistent store |

Save and load use identical record validation. Corrupt or unsupported saved history is never silently overwritten. If startup migration or expiry cannot write, the validated original records remain available and saving can retry later. Dirty state avoids rewriting unchanged history during periodic polls. Pin/unpin/delete actions validate and save their proposed state before reporting success. Age expiry runs even while recording is paused. Keep the desired guard switch separate from actual listener availability and never persist runtime disable events as a change of user preference.

Keep design explorations, manual checklists and generated verification outputs out of the repository. Maintain runtime assets, automated regression tests, and current usage, architecture and design-system documentation.
