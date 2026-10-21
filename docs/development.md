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
  settings/    Preference values, defaults and validation
  storage/     Versioned JSON repositories, legacy migration and atomic FIFO writes
  linkicons/   Restricted-origin HTTP icon retrieval and decoded image cache
  macos/
    pasteboard/ Native observation and verified clipboard writes
    guard/      Revision-bound policy, protected-copy requests and native tap
    appkit/     Source icons, NSMenu, glass and owned popover appearance
    internal/objc/ Shared low-level runtime helpers
resources/
  resources.go Embedded runtime artwork and catalogs
  brand/       Approved PNG artwork, menu-bar templates and Copiggy.icon source
  icons/       SVG icons and license
  locales/     Multilingual content-type search keywords
scripts/
  env.sh       Shared pinned toolchain, cache and production environment
  go.sh        Go command wrapper
  check.sh     Headless tests, static checks and production binary
  package.sh   Staged MyGo bundle, metadata verification and signing
  run.sh       Package, then open the official application bundle
build/         Generated output; ignored
.cache/        Toolchain, modules and build cache; ignored
```

## Responsibilities and threading

`main.go` calls `app.Run`. The app composes services and native adapters; `nativeui` renders MyGo Native UI and owns only transient presentation state. `clipboard.History` owns records and their exact representations together, so the UI cannot maintain a second copy of business state. Copy commands accept trusted record or draft IDs and derive content and protection scope inside that service.

Business commands run in order on an application worker. Native clipboard reads/writes and window operations execute on the AppKit thread; representation parsing, PNG validation, history encoding and disk I/O execute away from it. Queries read the last committed snapshot while disk writes are pending. Enhancement cancellation and image completions use the UI generation to reject obsolete results. Image decoding has two concurrent slots and a bounded bitmap cache. Quitting closes command admission, drains accepted work and flushes persistence before terminating the event loop.

`storage` owns file formats and atomic writes. It never decides which record is current or protected. `clipmodel` owns pure record rules, and `settings` owns preference defaults. `linkicons` has no MyGo dependency and returns ordinary Go images; the UI alone creates and caches bitmaps. Its entry point rejects protected records before any cache or network lookup.

Tests live beside their responsibility: domain/content rules in `clipmodel`, transaction/guard-observation tests in `clipboard`, file compatibility and failure tests in `storage`, and in-memory native rendering tests in `app/nativeui`. Native adapters retain separate initialization and failure boundaries. A popover appearance handle is an owned object with an idempotent `Close`; raw Objective-C ownership does not escape `appkit`.

## Commands

Run commands from the project root:

```sh
./scripts/check.sh
./scripts/package.sh
./scripts/run.sh
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

`scripts/package.sh` uses the CLI to create a production bundle in a temporary directory, then normalizes the executable to `copiggy`, verifies metadata and icon resources, removes duplicated embedded source artwork, retains licenses and signs the result again. Only a verified bundle replaces `build/Copiggy.app`; a failed build preserves the previous bundle. The identifier remains `dev.copiggy`. `run.sh` opens that official bundle through Launch Services.

Packaging requires full Xcode 26 or later for Icon Composer compilation; `DEVELOPER_DIR` is set only for the command when the standard Xcode location is available. The project does not change `xcode-select`. `resources/brand/Copiggy.icon` uses an exact copy of the approved `app-icon.png`, with warm light/dark backgrounds and no added glass, gloss or shadow on the artwork. Apple’s asset compiler produces both `Assets.car` and the ICNS fallback. Bundled launches retain that system icon; only a bare development executable calls `Dock.SetIcon` with the PNG. The application itself still targets macOS 13 or later.

The menu-bar pig and clipboard are drawn from native vector paths in `internal/app/nativeui/tray_icon.go`. Their 72px transparent PNG exports in `resources/brand/` must match the runtime templates; `TestTrayBrandAssetsMatchRuntimeTemplates` checks parity. MyGo sets their height to 18pt, and AppKit handles the light/dark template tint. Keep protection indicators tied to the live guard, not the saved preference.

The empty-history mascot uses two embedded PNG frames and three native rounded strokes aligned to the clipboard's tilt. Native timers schedule the occasional blink and animate each stroke only during its reveal or fade, sleeping through pauses. The empty pinned list uses two aligned pin-press poses with a long rest between gestures. Reduce Motion holds a still frame without timers. Search with no results retains its semantic icon. The [brand design notes](brand-design.md) describe the selected Little collector artwork and its generation prompts; [icon alternatives](brand-options/README.md) preserve the earlier choices.

Changing the bundle location or rebuilding an ad-hoc-signed binary can require the user to refresh Accessibility permission. Do not modify the TCC database or automate authentication.

History and preferences remain in `~/Library/Application Support/Copiggy/`. `history.json` is a versioned, atomic snapshot of ordinary and pinned records and their exact representations; `preferences.json` holds language, appearance, recording, desired Paste Guard state and retention. First launch imports the old `pinned.json` only when the unified history file is absent. Tests use temporary fixtures; never copy real clipboard contents into repository fixtures or documentation.

Preference changes enqueue one complete immutable snapshot and report actual save failures. Startup restores them before creating the panel or enabling recording/guard behavior. Explicit `light` or `dark` takes precedence over the system appearance; older language-only files remain supported. Failed saves show localized status instead of silently claiming success. UI tests inject a preference-saving port; repository tests use temporary paths.

### Persistence boundaries

| State | Behavior |
| --- | --- |
| Language, appearance, pause, desired guard and retention | Stored in `preferences.json` |
| Ordinary and pinned records, provenance, time and allowed apps | Stored in `history.json`; pins are exempt from age expiry |
| History limits | 100 ordinary / 100 pinned records, 64 MiB shared content; ordinary age limit 1/7/30 days or none |
| Current clipboard revision and active protection binding | Memory only; never restored as authority over a newer clipboard |
| Search, pin filter, selection, scroll and preview | Session-only |
| Unconfirmed enhancement drafts | Discarded on exit; never applied automatically |
| Website icons and decoded images | Bounded memory cache, rebuilt as needed |
| Webpage cookies and website data | WebKit's independent persistent store |

Save and load use identical record validation. Corrupt or unsupported saved history is never silently overwritten. If startup migration or expiry cannot write, the validated original records remain available and saving can retry later. Dirty state avoids rewriting unchanged history during periodic polls. Pin/unpin/delete actions validate and save their proposed state before reporting success. Age expiry runs even while recording is paused. Keep the desired guard switch separate from actual listener availability and never persist runtime disable events as a change of user preference.

The old web prototype, seeded diagnostic UI, self-test CLI modes and generated verification outputs are retired. The [archive](archive/README.md) records earlier decisions and checks; it does not define current behavior or prove the current build has passed manual verification.
