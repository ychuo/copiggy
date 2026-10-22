# Copiggy design system

Copiggy uses a compact native interface in which copied content is the main visual element. Shared controls, metadata, spacing, and interaction behave consistently across every content type. The contents of a card may look like an envelope, calendar, map, or color swatch without changing the surrounding control system.

The values below describe the implemented visual system in **logical macOS points**, not screenshot pixels. Shared values live in `internal/app/nativeui/native_design.go`; content-specific artwork stays within those common controls and layout rules.

## Layout and spacing

| Element | Shared specification |
| --- | --- |
| Main tray | 296 high; 80% of the active display's work-area width |
| History viewport | Full width; 22 padding only at the beginning and end of the scrollable content |
| Card | 188 × 188; 20 corner radius |
| Selection | 3-point stroke aligned with the card's outer contour |
| Card content | 16 inset; 10-point vertical row gaps |
| Card spacing | 14 between cards |
| Card header / footer | 24 / 16 high |
| Preview toolbar | 30 high; 8 between actions |
| Preview control radius | 6 |
| Preview reading-surface radius | 12; background and content use the same contour |
| Preview structure | Top toolbar, content, then metadata; no bottom copy/close keyboard hints |

Alignment and whitespace establish groups. Do not introduce general-purpose divider lines inside the tray or cards. A fine separator may distinguish a preview's metadata from its reading surface when the approved content design calls for one.

The native NSPopover owns its outline, continuous arrow, screen-edge placement, and relationship to the selected card. An inset content surface ends before the arrow; never append a separately drawn pointer or leave a colored surface accidentally cut off at its base.

## Icons and controls

Glyph size and click-target size are separate. Keep glyphs restrained while preserving usable click targets. An application icon is source information, not a competing hero image.

| Role | Glyph or artwork size | Control size, where applicable |
| --- | --- | --- |
| Card content-type icon | 16 | — |
| Pin on a card | 16 | 22 |
| Copy beside metadata | 16 | 22 |
| Search | 16 | 30 |
| Preview toolbar action | 18 | 30 |
| Source application | 24 | — |
| Protection badge / lock | 15 / 10 | — |
| Paste Guard toggle | — | 24 high |
| Keyboard hint keycap | — | 20 high |

Use simple rounded glyphs with comparable optical weight; evaluate the visible shape as well as its layout box. The refreshed shared set uses a 24-unit grid with regular 1.8-unit strokes and rounded caps and joins: Envelope, overlapping-page Copy, square-and-pencil Compose, rounded Phone handset, Pin and its filled state, Trash, Close, Plus, Minus, Calendar, Clock, Palette, Folder, Folder Open, File, Password, Key, Eye, Eye Slash, Text, JSON braces, Image, magnifiers, Link, Globe, external Open, refresh/reset, comparison, and confirmation. MapPin uses a compact filled location silhouette on the same grid. This is an updated subset, not a claim that every existing application glyph has been converted. The larger on-map marker is a separate 26 × 36 illustration with a white edge.

A pinned card always shows its filled pin. An unpinned card shows its outline pin only while hovered or focused; Cmd+P and the preview action remain available. Do not enlarge common icons to match a content illustration.

External actions (Compose Mail, Call, Maps, Calendar, and Finder) use the shared custom glyph family at 18 points. Real application artwork is reserved for the captured source; action icons stay consistent even when the default handler differs.

Interface icons use soft neutral gray, adjusted for the surface underneath. Source application icons keep their original artwork. Lock badges express the actual protection policy; the menu-bar shield follows the live guard listener. Icon-only actions retain concise localized accessibility labels.

## Typography

Use `system-ui` for interface text and prose, allowing Core Text to handle optical sizing and CJK/emoji fallback. Use `monospace` for code and data whose alignment depends on character width. No downloaded font is required. A screenshot cannot establish an exact font family; these choices provide a consistent native implementation.

| Role | Size | Weight |
| --- | --- | --- |
| Metadata: timestamps, dimensions, type labels | 11 | Regular 400 |
| Control labels | 12 | Medium 500 |
| Short content values: coordinates, original email address | 11 | Medium 500 |
| Base interface / search text | 13 | Regular 400 |
| Keyboard hint labels | 10 | Regular 400 |
| Card prose | 15 | Regular 400 |
| Short card titles | 18 | Medium 500 |
| Preview prose | 17 | Regular 400 |

Use tabular numerals for times and dimensions. Avoid making all copied text bold. Content display typography may be larger when its meaning calls for it, such as an email account, calendar day, telephone number, or color value. Those sizes belong to the content illustration; metadata and controls still use the shared scale.

Action names use secondary text ink, with softer gray reserved for their glyphs. Times, type captions, and explanatory metadata remain regular; medium weight distinguishes a key content value from its caption. Keep the native system font and approved hero sizes rather than making every text role heavier.

Long content must fit, wrap, or scroll deliberately. Rendering never rewrites saved content. Truncation on a card must not prevent access to the complete value in preview.

## Surfaces and appearance

Separate **application chrome** from **content surfaces**. The tray, popover glass, search, toolbars, and other shared controls follow the selected light or dark appearance. The tray uses NSGlassEffectView, with NSVisualEffectView as the older-system fallback; preview uses NSPopover. Native content surfaces remain transparent where the system material should show through. The main tray retains the adaptive Regular glass style. Light mode applies a restrained native neutral-gray tint (white 0.55, alpha 0.14) through NSGlassEffectView.tintColor; dark mode clears it. This is material tinting, not an opaque overlay. Regular becomes more opaque at larger sizes and brightens over light backgrounds; Dock and popover materials need not render identically. See Apple’s [materials guidance](https://developer.apple.com/design/human-interface-guidelines/materials) and [NSGlassEffectView API](https://developer.apple.com/documentation/appkit/nsglasseffectview).

A content surface owns its background, primary ink, secondary ink, and embedded icon colors together. Never apply light text from the outer dark theme to a pale content surface. Image metadata uses small neutral-gray frosted backplates with readable ink, not full-width black scrims.

Email, phone, coordinates, and date keep their light content materials in both appearances: blue envelope and warm paper for email, pale mint for phone, a light map for coordinates, and warm calendar paper with a coral header for date. File paths keep their warm folder material, verification codes their lavender paper, and private keys their cream sealed-paper cards. Text uses warm note paper and links use warm white paper. JSON, timestamp, and token cards keep dark surfaces; private-key previews use a dark reading surface. Color swatches retain the actual captured color, and images retain their original pixels. Each content surface keeps its recognizable material in both appearances while the surrounding application chrome follows the theme.

## Interaction and motion

- Selection, preview and format changes do not write to the clipboard. Enter or double-click explicitly copies the selected content, closes the panel, and leaves pasting to the user. With no explicit format choice, copy the exact saved representations. An accepted JSON draft or a chosen color, phone or timestamp format produces plain-text clipboard output without editing the original or adding history. Derive output from the trusted record/draft and inherit its saved allowed apps. Closing or navigating resets the choice; Reveal and zoom never select a copy format.
- Space or Cmd+Y opens preview; arrow keys navigate cards. Search retains arrow keys for its caret. Toolbar clicks preserve the appropriate history/search focus.
- Cmd+P pins or unpins. Cmd+Backspace deletes the selected record unless search owns input. Deletion leaves the system clipboard and its active protection policy unchanged.
- Pin and Delete are available in preview. The top pin control filters history. Pause/resume, language, and appearance belong in the menu-bar menu.
- Use brief native search, scroll, and control transitions; honor Reduce Motion. Hide the history scrollbar without removing scrolling.
- Routine actions stay quiet. Only protection updates and actionable errors appear briefly beside the menu-bar icon. Do not create tooltip or toast windows.

Default interface copy is concise English, with English, Simplified Chinese, Traditional Chinese, and Japanese in the shared catalog. Actual application names and copied content are not translated.

## Content variation: email

Email uses the same frame, header, metadata, source icon, pin, and preview controls as the rest of the application. Its distinctive elements are the folded envelope artwork, split account/domain display, and warm reading paper.

### Envelope and paper

| Role | Value |
| --- | --- |
| Main ink / secondary ink / embedded control ink | `#202a33` / `#697784` / shared light icon ink `#78818b` |
| Reading paper | Opaque `#fdfbf8` |
| Envelope paper | `#f3f8ff` → `#dfedff`, 145° |
| Left fold | `#dcecff` → `#bed7f5`, 135° |
| Right fold | `#e4f1ff` → `#c9dff8`, 225° |
| Front flap | `#c4ddfa` → `#d5e7fc`, 180° |
| Crease | `#acc9eb`, 0.7-point stroke at 85% opacity |
| Fold shading | Restrained `#6f97c6` shadows and white edge light |
| Fold shoulders / tip | 60% / 96% of card height |
| Front flap peak | 86% of card height, rounded tip |

Keep the diagonal folds continuous and subtle, without a horizontal divider across the envelope. Source artwork identifies the actual copying application; a Mail icon in a reference is not a replacement for it.

### Display typography and preview

| Element | Specification |
| --- | --- |
| Card account name | 34 / 500; 1.15 line height |
| Card domain | 17 / 400; 1.2 line height |
| Card address group | Centered; 2-point interline gap; shared hero geometry compensates for the unequal header/footer heights and centers the complete group at (94, 94) |
| Preview content size | 520 × 264, subject to available screen space |
| Preview outer spacing | Top 12; sides and bottom 8; toolbar-to-paper gap 8 |
| Reading surface | 12 corner radius; 14 content inset |
| Preview account name | 46 / 500; 1.15 line height |
| Preview domain | 28 / 400; 1.2 line height |
| Preview address group | Centered; 3-point interline gap; reading-area top inset 20 |
| Metadata separator | 0.5-point stroke; secondary ink at 18% opacity |
| Metadata row | 22 high to retain the shared 22-point Copy target; shared metadata type and copy glyph; 8-point gaps |

Card account names longer than 10 characters start at 24 points. Measured text fits into 148 points, down to 14; domains fit down to 10.5. Native ellipsis handles remaining single-line overflow. Preview names longer than 10 characters use 30 points; names longer than 24 use 24. Long values wrap and scroll inside the reading area without moving the toolbar or metadata.

The preview toolbar contains Compose Mail, Pin, Delete, and Close. Its glyphs and labels use the common control scale. The metadata row shows Copy and the original address on the left, with the localized type label on the right. Captured spelling and case remain intact in the original value; the display domain may be normalized by semantic analysis.

The reading surface is inset 8 points from the popover's sides and bottom so that warm paper ends deliberately before the glass arrow. The arrow and toolbar follow the surrounding appearance; paper content keeps its own readable ink.

## Content variation: phone

Phone follows the shared frame, header, source icon, metadata, pin behavior, and toolbar controls. Its content variation is a prominent grouped number over a pale mint surface, with a quiet dial-pad motif on the card. Display-number sizes are semantic typography; classification and dialing metadata use the shared base and preview text sizes.

### Card and color

| Element | Specification |
| --- | --- |
| Card surface | `#edfcef` → `#ddf5e7`, 135° |
| Reading surface | `#fbfefa` → `#edf9f2`, 135° |
| Main ink / secondary ink | `#202a33` / `#697784` |
| Embedded control ink | Shared light icon ink `#78818b` |
| Card number | System sans-serif, 24 / 500; 1.15 line height; tabular numerals |
| Card number fitting | Fit within the shared 156-point content width, down to 14 points |
| Number placement | 8-point body top inset; a single line adds 14 points to preserve its visual center |
| Dial-pad motif | 3 × 3 circles; diameter 9; pitch 15; first row at y116; ends before the shared metadata row |
| Dial-pad ink | `#b6e8c6` at 52% opacity |

Preserve the existing display grouping. Prefer a two-line break after a closing parenthesis when a number continues; otherwise break at the existing space nearest a balanced split. An ungrouped number stays intact. Line breaking does not invent a country code or change the stored number.

The card and reading surface retain these mint colors and dark ink in both appearances. The tray, popover glass, toolbar, and selection treatment continue to follow the chosen theme.

### Preview

| Element | Specification |
| --- | --- |
| Content size | 520 × 360, subject to available screen space |
| Outer spacing | Top 12; sides and bottom 8; toolbar-to-paper gap 8 |
| Reading surface | 12 corner radius; 16 content inset |
| Display number | 48 / 500; 1.15 line height; tabular numerals; fit down to 24 points within preview width minus 64 |
| Classification | Shared body size 15 / 400; localized Phone or International label |
| Dialing number | Shared preview size 17 / 400; tabular numerals |
| Number-to-details gap | 20 |
| Classification-to-dialing-number gap | 6 |
| Toolbar | Call, display-format toggle, Pin, Delete, Close; shared 18-point glyphs and 30-point click targets |

The format action switches between the original grouping and dialing representation. Choosing Dialing number makes the next Enter copy the normalized dialing number; Original number restores exact-original copying. The switch itself never writes, and neither choice edits the saved record or adds history. Call uses the dialing number. Do not add another Copy button or a divider line to this preview.

The 8-point glass margin keeps the mint reading surface visually separate from the system popover arrow. Content may scroll within the reading area while the toolbar stays in place. Protection scope, when present, uses the reading surface's own ink and the existing live guard state.

Use the native element gradient background for this surface, so it follows the shared reading-surface radius. A custom paint callback is not rounded by the element's child clipping and must not paint a rectangular fill over the glass corners.

## Content variation: coordinates

Coordinates use an actual MapKit snapshot as their content, with a central coral marker and compact metadata. The map remains light in both appearances; the tray, popover glass, toolbar, and surrounding metadata follow the selected appearance. This content variation keeps the shared card frame, icon sizes, click targets, and typography.

### Card and map rendering

| Element | Specification |
| --- | --- |
| Map appearance | Light system snapshot, rendered with Contain to preserve the complete image and MapKit labels |
| Raster density | Keep logical MapKit size; explicitly select/render the native 2× representation before PNG encoding: 376 × 376 pixels for a 188-point card, 1008 × 352 for the ordinary preview |
| Fallback surface | `#eff3ed`; main ink `#202a33`; secondary ink `#697784` |
| Location marker | Coral `#ff786e` → `#f25450`, white edge and center; dedicated 26 × 36 multicolor SVG rendered as artwork, never as a monochrome icon mask |
| Card location glyph | Gray filled location silhouette with a true center cutout; shared 16-point size |
| Marker positioning | SVG tip at (13, 34) aligns to the requested coordinate at the center of the snapshot |
| Marker scale | 1 on the card; 1.3 in preview |
| Card metadata | Shared 11-point type on small light glass backplates |
| Card footer | Shared 16-point inset and 16-point row; Pin retains the common 22-point target at x150, y153 within the 188-point card |

Loading and completed maps use the same footer geometry as every other card. Keep the complete snapshot using Contain; do not crop or enlarge it beyond its card to adjust controls. The small 188-point snapshot does not establish a separate attribution strip, so do not invent one by raising its metadata and Pin. The wider preview reserves its lower map edge for system map labeling. The marker's tip, not its overall bounding-box center, identifies the coordinate.

Until a usable snapshot exists, show a local semantic icon and a concise Loading map or Map unavailable status. Protected records show Protected location without automatically requesting a map over the network. The fallback remains a native local view.

### Preview

| Element | Specification |
| --- | --- |
| Content size | 520 × 264, subject to available screen space |
| Outer spacing | Top 12; sides and bottom 8; 8-point gaps between toolbar, map, and metadata |
| Ordinary map area | 504 × 176; 12 corner radius; clipped to the continuous rounded shape |
| Toolbar | Maps, Pin, Delete, Close; shared 18-point glyphs and 30-point click targets; Maps uses the shared location glyph, matching the other external actions |
| Zoom controls | Shared Plus and Minus controls in a small light glass group; 10-point right inset |
| Zoom range | −2 through 3; disable the respective action at each bound |
| Attribution clearance | Controls stay above the bottom 24 points of the map |
| Metadata row | 22 high; shared Copy control, original coordinates, and WGS 84 label |

The map area is the remaining flexible space; an additional protection-scope row can reduce its height. Preserve the map's system attribution and keep the 8-point glass edge distinct from the native popover arrow.

Opening Maps is an explicit action. Copy and Enter use the original saved clipboard representation, not a reformatted coordinate string or the rendered map image. Zoom only changes the preview snapshot. Protected records retain their local fallback and do not expose automatic zoom requests; their protection policy and copying rules remain unchanged.

## Content variation: color

A color card is the color itself, with adaptive contrasting ink and no decorative lighting that would alter the swatch. Opaque colors render their exact RGB channels; alpha colors composite over a neutral 12-point checkerboard. Both appearances use the same color surface. Shared card header, source icon and Pin positions stay unchanged.

- Card value: system sans-serif, 26 / 500, measured to fit the shared content width down to 18 points; secondary RGB value uses the shared 11 / 400 metadata scale. Alpha values keep their alpha component in the secondary representation.
- Preview: 520 × 232; the common top-12 / side-and-bottom-8 glass margin, 8-point toolbar gap and 12-point reading-surface radius. Custom swatch painting lives inside the rounded clipping container.
- Toolbar: Palette and localized Color label, grouped HEX / RGB / HSL display choices, then shared Pin, Delete and Close actions.
- Preview value: 32 / 500 for HEX; 26 / 500 for RGB or HSL; secondary value 17 / 400. Long format strings wrap within the color surface.
- The protection scope, when present, uses the same contrast-tested ink as the swatch label. Explicit HEX / RGB / HSL selection chooses the next copied representation; Enter confirms it. Simply opening preview still copies the captured original, even when HEX is displayed by default. Conversion never edits the saved value or adds a history item.

## Content variation: date

The card is a single calendar page. Its coral header is 55 points high (about 29% of the card), with the month centered near y45. The day/caption group is centered at the square’s midpoint, below the coral month band; do not place a second calendar tile inside the card.

- Card paper: `#fffefd` → `#fffaf5`; coral band `#fb8175` → `#f36f65`, 135°. Use `#202a33` primary and `#697784` secondary ink in both appearances.
- Card month: 12 / 500, white. Day: 50 / 500 with tabular numerals; short weekday and time: 12 / 500. The shared footer shows an explicit UTC offset when present, otherwise the year.
- Preview: 520 × 300; common 8-point glass margin and 12-point rounded paper. The Calendar action and shared Pin, Delete, Close stay in the themed toolbar.
- A 128 × 128 calendar tile with a 33-point coral month band sits beside month/year (22 / 500), weekday (18 / 400) and time (21 / 400). A restrained shadow separates the tile from paper. The large tile day is 64 / 500.
- Metadata uses the shared 22-point Copy control, exact original text and an explicitly captured timezone. Date-only values show All day and never invent a timezone. Opening Calendar remains an explicit action through the trusted content-action pipeline.

## Content variation: timestamp

The timestamp is a dark counter in both appearances, with the original numeric value, scale ticks and a converted-time caption. It uses the same card geometry, metadata rows, Pin and source icon as the other types.

- Card: `#3c4a60` → `#29313b`, 135°; main ink `#f1f5fc`, secondary `#afc0d8`. Counter 22 / 500 monospace for seconds, 17 / 500 for milliseconds. The 17 scale ticks center at y110; the middle tick is taller and brighter.
- Preview: 520 × 310; common 8-point glass margin, 12-point rounded dark reading surface and 24-point content inset. Counter 42 / 500 monospace, left aligned and measured to fit narrower previews; date and clock 23 / 500; alternate-zone time 15 / 400.
- Toolbar: compact s / ms choices with localized full accessibility names, a local/UTC display toggle, then shared Pin, Delete and Close. The local label shows the actual offset for that instant, including daylight-saving rules.
- Explicit s / ms selection sets the next copy unit; no clipboard write happens until Enter. Milliseconds converted to seconds drop subsecond digits in that output only; choosing milliseconds again derives the complete value from the original. Local/UTC affects only the date explanation and never the copy unit. These choices never replace saved representations, add history or change protection scope. Converted clock time keeps captured subsecond precision even while the number is shown in seconds. When local and UTC fall on different dates, the alternate line includes its date. Date ordering follows the interface language. At reduced heights, the reading area scrolls above the stationary protection scope.


## Content variation: file path

A file-path card is a warm folder with a small document, file name and parent directory. Its light palette stays fixed in both appearances; shared toolbar controls still follow the surrounding glass.

- Card: `#fff3d5` → `#ffe8ae`; folder tab begins at y42 and its continuous face at y54. The document is 38 × 51 with a folded corner, three small content strokes and the copied extension. Names use 18 / 500, or 15 / 500 for longer names, with two lines; the parent directory uses 12 / 400.
- Preview: 520 × 300; common 8-point glass margin and 12-point rounded reading paper. A 70 × 92 document sits beside the full file name (25 / 500) and path (17 / 400), separated by 24 points. Long names and paths wrap in the scrollable reading area above the stationary metadata.
- Toolbar: outgoing-folder Finder action, Pin, Delete, Close. Metadata: shared Copy control and localized File path label, followed by any actual protection scope.
- Rendering uses the copied path alone. It never inspects the target or reads a file just to display its card. Finder explicitly resolves the trusted saved path. Copy retains the exact captured representation, including a home-relative path or encoded file URL.

## Content variation: verification code

Verification codes use pale lavender paper in both appearances. All digits are immediately readable in the card and preview.

- Card: `#f5f2ff` → `#e7e3fb`; primary ink `#262e40`, metadata `#626b83`. Four to eight 22 / 500 tabular digits sit above matching underlines, centered at the square's midpoint. All digit slots use equal spacing, including the gap between the third and fourth digits. The shared footer shows the digit count and Pin.
- Preview: 520 × 300, shared 8-point glass margin and 12-point paper radius; `#faf7ff` → `#eeebfc`. Verification code title uses 25 / 500; readable digits use 32 / 500 with tabular numerals.
- Toolbar: Pin, Delete, Close; no Reveal control. Footer: shared Copy control and digit count; actual protection scope appears only when present. A reduced-height reading area scrolls without displacing the footer.
- Copy retains the complete original clipboard representation, including any copied label or spacing. Displaying digits does not write or alter protection.

## Content variation: token and private key

Tokens and private keys retain their distinct cards while sharing a dark, rounded preview for reading credentials.

- Both cards use a centered multiline credential block: three rows for tokens and four for private keys. The first six characters start the first row; the last six end the last row, with bullets masking every intermediate position. Card text uses 13 / 500 monospace. Private keys abbreviate the parsed PEM's base64 payload, not its BEGIN/END lines or arbitrary headers. Short values reduce the visible ends so some content always remains concealed.
- Token card: `#3b444f` → `#272d34`, pale `#dbe2f1` ink and `#b7c0d1` metadata; recognized format appears in the shared footer.
- Token preview: 520 × 264; common 8-point glass margin and 12-point reading-surface radius; `#343d48` → `#29323d`. Reveal / Hide starts the toolbar, followed by shared Pin, Delete and Close on the right; no redundant Token title. The same multiline abbreviation uses 20 / 500 monospace above actual protection scope and recognized format metadata.
- Private-key card: warm `#fffaf0` → `#f3e7d1` paper, a top-right fold and a subtle sealed lower flap. The central seal leaves the common Pin anchor clear. These marks are paper decoration, not a claim that Paste Guard is enabled.
- Private-key preview: 520 × 320, the same dark reading paper. A successfully parsed, allowlisted PEM boundary may appear above and below its abbreviated payload. The toolbar starts with Reveal / Hide; footer shows only actual protection scope and PEM or OpenSSH format.
- Revealed token/key content uses the exact original text in 14-point monospace with 1.6 line height, scrolling within the fixed reading area. Closing or changing records returns to the abbreviated value. Invalid or stale sensitive annotations fail closed; the concealed middle never enters visible text or accessibility labels before Reveal.

## Content variation: plain text

Text is a warm note, with the copied prose itself as its main visual element. It uses the common header, source icon, footer and Pin anchor.

- Card: `#fff6e3` → `#ffedcc`; primary ink `#272f3a`, metadata `#6b7584`. Prose is 15 / 400 with four visible lines; short single-line text may use 18 / 500. The footer reads Plain text.
- Preview: 520 × 328; shared glass margins and a 12-point rounded `#fdfbf8` reading surface. Text uses 17 / 400 with 1.5 line height, retaining original paragraphs in a scrollable reading area.
- Toolbar: Pin, Delete and Close only. A fine separator precedes the stationary metadata row: File and Plain text on the left. No source icon or application name appears in the preview footer. Any protection scope remains visible below it.
- Display and scrolling never change saved content. The card’s source icon continues to identify the real copying application.

## Content variation: JSON

JSON keeps a charcoal code surface in both appearances, with aligned indentation and violet keys, mint strings and gold values.

- Card: `#353d48` → `#252c35`; default ink `#eef2fa` and metadata `#b7c1d3`. Code uses 11.5 / 400 monospace with 1.4 line height and up to five visible lines. The footer counts top-level properties or array items; scalar values show JSON.
- Preview: 520 × 328; shared glass margins, 12-point rounded dark reading surface, and 22-point inner content padding. Code preserves alignment and scrolls in both directions. Metadata below the surface shows the property/item count and Original, Formatted draft, or Formatted copy provenance.
- Toolbar: Format at the left, then shared Pin, Delete and Close. A formatting draft replaces Format with the existing comparison, cancel and explicit confirmation controls.
- Indentation in the default reading view is presentation only. Explicit Format creates a local, cancellable draft; Return or Copy result writes that draft with the source record’s saved protection scope. Comparing Original displays the exact captured text, and Copy original uses its exact saved representations. Confirming a draft consumes it without editing the source or adding a history record; closing or navigating discards it.

## Content variation: image

Image cards use the captured image edge to edge, with the shared rounded contour. Type/time and dimensions sit on compact gray glass backplates; Pin keeps the common anchor. No full-width scrim or image tint is added.

- Preview: 520 × 420; common glass margins and a 12-point rounded viewport. Fit displays the complete bitmap in its original aspect ratio, including transparency, centered within the available area.
- Toolbar: Zoom out, Zoom in, Fit, Pin, Delete and Close. Zoom stays in this same preview, with five levels from Fit through 1.5⁴ times the fitted size. Overflow scrolls in both directions; a letterboxed axis stays centered.
- Metadata remains below the image and shows the captured dimensions and format. A protection row, when present, reduces the image viewport rather than displacing the controls.
- Zoom resets on navigation or close. Missing or decoding imagery has a neutral semantic fallback. No preview operation changes the saved bitmap or clipboard.

## Content variation: link

A link with a cover uses an upper image and lower warm-white website summary. Matching copied HTML is preferred; visible or nearby unrestricted public cards can lazily retrieve the page's title and cover. Without a cover, show the actual website icon and available title, falling back to the saved URL. Never invent a cover.

- Card: warm `#fdfbf8` paper, `#223338` primary ink and `#697784` metadata. A cover is 106 points high; its 20-point website icon and 14 / 500 title begin at y114. The title has up to two lines. Without a cover, use a 28-point website icon and 15 / 500 title. The original domain and Pin use the shared footer.
- Preview: 720 × 500; common glass margins and a 12-point rounded actual-page viewport. Native loading/error states use matching warm paper.
- Toolbar: Back when available, Reload/Stop, current website icon and domain on the left; Open in browser, Pin, Delete and Close on the right. A cached icon is reused only while the visible page has the original scheme, host and port.
- Footer: shared Copy control and the original saved domain. Navigation changes the toolbar's location; Copy and Enter still use the exact saved record. Protection scope appears only when present.
- Metadata retrieval requests the saved path and query without its fragment, parses Open Graph/Twitter metadata, and fetches the cover without browser cookies, authentication, referrers or scripts. It never starts during capture or for protected/private/local links. At most three link workers share the bounded memory cache with maps; cover results last six hours, failed or coverless results ten minutes. Pending work cancels on hide/quit; nothing in this cache is persisted.
- The interactive page is created only when its preview opens, using WebKit's normal persistent data store. Existing navigation, download and new-window restrictions remain in force. Website-icon retrieval keeps its origin-only rules; source application artwork remains separate from the website icon.
