# Content recognition rules

This describes the implemented rules, including deliberately unsupported forms. There are **16 current presentation types**. The legacy `files` type exists only for migration; a single Finder file is shown as a file path and multiple files as multiline text. Web links and inferred domains use `link`; App schemes use the independent `app-link` type (`clipmodel.TypeAppLink`), and Markdown uses `markdown`. QR is a presentation mode, not another type. Older saved App destinations are reclassified from `link` or `text` without changing their original representations, identity, source, timestamps, pins or allowed applications.

Recognition is local and matches complete values. It does not extract addresses, dates, codes or credentials from arbitrary sentences, read files, validate credentials online, or fetch pages during capture. A recognized type describes the content; it does not establish that an email, phone number, URL, file or credential works.

## Selection and precedence

The clipboard may supply several representations of the same content. Presentation uses this order:

1. Valid local Finder/file references: one full path or multiple newline-separated paths.
2. Nonempty plain text, including the native adapter's supported text decoding.
3. A valid visible PNG. MyGo can materialize TIFF as PNG. When HTML accompanies a transparent or 1×1 placeholder, use the HTML's visible text instead.
4. Visible text extracted from HTML, without executing scripts or loading resources.
5. A complete supported HTTP(S)/app URI list when no preceding representation provides content. One URL becomes a Link or App scheme card according to its scheme; multiple URLs become multiline text.

Original saved representations remain available for exact copying. The existing Finder exception deliberately copies derived full path text, never native file references. Semantic display normalization, including whitespace trimming or interpreting a direction/unit, does not rewrite the original text. Copy variants still require an explicit user choice and inherit the saved allowed-app set.

For text, leading/trailing Unicode whitespace is ignored for recognition. The order is:

`JSON → private key → token → verification code → file path → link/App scheme → email → color → coordinates → date → timestamp/phone → Markdown → text`.

JSON objects/arrays, complete private-key blocks and Markdown documents may be multiline. After the first two checks, text containing NUL, CR or LF, or longer than 8,192 bytes skips the single-value recognizers and goes directly to the bounded Markdown check. Markdown rejects NUL; unrecognized or oversized documents remain text. Private-key and Markdown recognition are each limited to 64 KiB. Other recognizers have smaller limits where noted below. A bare number that is both a valid phone and a timestamp remains text; an explicit timestamp unit disambiguates it. Bare four/six-digit values have not become verification codes.

Lists consisting only of complete file paths or URLs remain literal multiline text, even when their names contain Markdown punctuation. Explicit Markdown headings, list markers or link markup still opt into Markdown reading.

## Current types

| Type | Matching rule and examples | Boundaries |
| --- | --- | --- |
| Text | The fallback for supported text content, including multiline prose and code. HTML-only content may provide visible text. | No guessing from embedded fragments. Weak Markdown signals, CSV, XML and unrecognized semantic formats remain text. |
| JSON | One complete valid JSON object or array, including indentation and line breaks: `{"a":1}`, `[1,2]`. | Scalar roots such as `true`, `null`, `"hello"` and `123` are not classified as JSON. No comments, trailing commas, JSON5, Markdown fences or multiple JSON documents. A scalar may independently match another type. |
| Link | One whole HTTP(S) URL, inferred domain or standalone Markdown link to a web destination. Examples: `https://example.com/a?q=1#part`, `www.example.com/page`, `[Read this](https://example.com/page)`. URI-only clipboard data also supports explicit webpage links. | ≤8,192 bytes, one complete destination, no whitespace within its URL. Inferred domains need valid IDNA/DNS labels and a recognized public suffix; common file endings remain text. No automatic extraction from prose. Full details below; recognition does not grant permission to fetch metadata. |
| App scheme (`app-link`) | One explicit app URI or standalone Markdown link to it, such as `aweme://video/123`, `sslocal://route` or `[Open video](aweme://video/123)`. URI-only clipboard data also supports a single explicit app destination. | ≤8,192 bytes, a valid `scheme://` with nonempty host or route path, no URL whitespace or credentials. Dangerous/specialized schemes are excluded. Recognition does not require an installed handler and never triggers network requests or a WebView. The dedicated preview defaults to QR. |
| Markdown | A complete document with a clear structural signal, such as a heading, two-item list, letter-containing blockquote, nonempty closed fenced code block, table, task checkbox, image/link markup or bold text; alternatively at least two inline-format signals. Supports native Reading/Source preview. | ≤64 KiB, valid UTF-8, no NUL and at most 2,048 LF characters; AST ≤4,096 nodes and depth ≤32, tables ≤16 columns/256 rows including the header. A supported standalone Markdown link becomes Link or App scheme according to its destination. Reading mode never executes HTML, opens inline links or fetches images. Weak signals remain text. |
| Image | Visible PNG data, including PNG materialized from native TIFF. Maximum 24 million pixels; positive dimensions and complete decoding are required. | Fully transparent content is ignored. JPEG/GIF/WebP-only payloads are not currently requested/decoded by the image pipeline; applications offering the same image as PNG/TIFF are supported. A copied image file in Finder is a file path, not image content. |
| Email | A single mailbox such as `alex+tag@example.com`, `<alex@example.com>`, `Alex Doe <alex@example.com>`, or `mailto:alex@example.com`. A quoted display name is supported; percent-encoded mailbox characters in simple mailto values are decoded for presentation/actions. | Extracted mailbox ≤254 bytes, local part ≤64 bytes; domain must contain a dot, with nonempty labels ≤63 bytes made of letters/digits/hyphens and no edge hyphens. Wrapper ≤1,024 bytes. Reject multiple recipients, mail headers, controls, malformed brackets and mailto query/fragment delimiters, including encoded forms. No quoted mailbox local parts. |
| Phone | A valid complete number according to bundled offline numbering metadata. Explicit international prefix or the record's saved numbering region supplies context. Supports full-width digits and common separators, `tel:`, numeric extensions and numeric `phone-context`. | Region is independent of UI language/time zone. No country guessing to make arbitrary numbers fit; no numbers extracted from prose or vanity letters. A number also matching a bare timestamp stays text. See details below. |
| Coordinates | Unlabeled decimal pair, explicit N/S/E/W decimal degrees, degrees/minutes, or degrees/minutes/seconds. Examples: `37.428230, -122.168861`, `122.168861W 37.428230N`, `37°25.6938′N 122°10.13166′W`, `37°25′41.628″N 122°10′7.8996″W`. | Latitude ±90, longitude ±180, validated before rounding. Unlabeled inputs require a comma and decimal point in both numbers. Prefer latitude first when both orders are legal; otherwise accept the reverse only when legal. Explicit directions allow either axis order, comma/whitespace separation and prefix/suffix directions. No signed directional magnitudes, duplicate axes, labels such as `lat=`, `geo:` URIs or incomplete units. |
| Date | Four-digit year first, with matching `-`, `/` or `.` separators and one/two-digit month/day: `2026-10-23`, `2026/10/3`, `2026.1.3`. Optional space or `T` plus hour/minute, seconds and fractional seconds. RFC 3339-style dates accept explicit `Z` or numeric offsets. | 8–40 bytes; years 1900–2199; invalid calendar dates/times/offsets reject. Explicit zones require `YYYY-MM-DD`, `T`, seconds and `Z`/`±HH:MM`, e.g. `2026-10-23T15:30:00+08:00`. The existing parser also accepts one-digit hours and comma fractional seconds. No mixed separators, month-first/day-first dates, relative dates, zone abbreviations or date extraction from prose. Zone-less values keep their civil fields without guessing a zone. |
| Timestamp | Bare 10-digit Unix seconds or 13-digit milliseconds; also integer `s` or `ms` suffixes, joined directly or with one ASCII space: `1792645800s`, `1792645800123 ms`. | Result must fall within 2000–2099 UTC. Explicit units permit 9–10 digits for seconds and 12–13 for milliseconds so the beginning of 2000 is representable. Units are lowercase. No signs, fractions, micro/nanoseconds, arbitrary prefixes, multiple separating spaces or automatic reinterpretation of phone/epoch ambiguity. |
| Color | `#RGB`, `#RGBA`, `#RRGGBB`, `#RRGGBBAA`; `rgb()`/`rgba()`, `hsl()`/`hsla()`, and whitespace-form `hwb()`. Examples: `#abc8`, `rgb(100% 0% 0% / 50%)`, `hsl(0.5turn 100% 50%)`, `hwb(120 20% 30% / 0.5)`. | ≤128 bytes. Finite decimal/scientific numbers only; RGB 0–255 or 0–100%, alpha 0–1 or 0–100%, other percentage channels 0–100%. Hue accepts degrees, `deg`, `rad`, `grad`, `turn`, including wrapping negative angles. No named colors, `0x` colors, `var()`, `calc()`, relative colors, `none`, Lab/OKLCH or wide-gamut color functions. |
| File path | Local absolute `/…`, home-relative `~/…`, or local `file:` URLs. One matching outer single/double quote pair is accepted. Examples: `/Users/alex/My File.txt`, `"~/Downloads/a.txt"`, `FILE://LOCALHOST/tmp/a.txt`. Finder references use their validated full paths. | No disk access during recognition. No remote file hosts, URL credentials/query/fragment, controls, parent traversal, bare root `/`, relative paths, Windows paths or expansion of shell syntax/variables. Quoting is only a wrapper, not a shell parser; spaces/backslashes/opposite quote characters inside remain literal. Multiple Finder paths remain ordinary multiline text. |
| Verification code | A supported whole label followed by a separator and 4–8 digits, or two groups of 2–4 digits separated by one space/hyphen. Examples: `OTP: 123456`, `verification code: 123-456`, `验证码：１２３４５６`. | A label is required. Full-width digits and selected full-width/nonbreaking separators are normalized only for display. No bare six-digit assumption, surrounding messages, arbitrary alphanumeric codes or more than two groups. Full label list below. |
| Token | A structurally recognized JWT or one of the explicit provider prefix patterns listed below. `Bearer TOKEN` and `Authorization: Bearer TOKEN` wrappers are accepted only if TOKEN already matches those rules. | ≤8,192 bytes. No generic long-string guessing, arbitrary opaque Bearer values, multiple headers or prose extraction. Masked display uses the parsed token; Reveal and ordinary copying retain the full original. Structural recognition does not verify signature, issuer, expiry or validity. |
| Private key | One complete PEM block of an allowlisted private-key type, with at least 32 decoded payload bytes and no non-whitespace trailing data. OpenSSH additionally requires its format magic. | ≤64 KiB. No public keys, certificates, bare Base64, PuTTY/PGP private-key formats or multiple concatenated blocks. Recognition does not decrypt or establish cryptographic validity. |

## Detailed format rules

### Web links, inferred domains and app links

- All link recognition is local and whole-value, with at most 8,192 bytes after trimming outer whitespace. Invalid UTF-8, internal whitespace/control/Unicode-format characters, backslashes and literal angle brackets in the destination reject. An explicit port must be numeric and in 1–65,535.
- Explicit `http://` and `https://` links require a hostname and accept case-insensitive schemes, paths, queries and fragments. Private/local addresses can be recognized; existing rules still exclude them from automatic metadata/icon retrieval. Credential-bearing web URLs may be recognized, but are excluded from web preview and background retrieval.
- Bare `example.com`, `www.example.com/path?a=1#part`, `example.co.uk:8443/page` and supported international domains derive an HTTPS destination. IDNA normalizes the destination hostname to lowercase ASCII; the bundled public-suffix list must yield a registrable domain. DNS labels are 1–63 ASCII letters/digits/hyphens, without edge hyphens; the complete ASCII hostname is at most 253 bytes. No DNS or reachability check occurs. Bare IP addresses, single-label hosts, unknown suffixes, trailing dots and credentials reject. Ambiguous `.md`, `.zip`, `.mov`, `.sh` and `.rs` endings require an explicit HTTP(S) scheme.
- App links require a valid explicit `scheme://` and a nonempty host or nonempty route path, such as `aweme://video/123`, `sslocal://route?url=…` or `my-app:///route`. The original destination, including path/query/fragment, is preserved. Empty schemes/routes, query/fragment-only routes and URL credentials reject. `javascript`, `data` and `vbscript` are excluded; `mailto`, `tel` and `file` remain with their specialized recognizers. Unknown schemes can be recognized without an installed handler; only an explicit Open in app action asks macOS to open them.
- A whole `[title](destination)` becomes a Link card for web destinations or an App scheme card for app destinations. The nonempty title is at most 1,024 bytes, without nested brackets or escaped-label syntax. Balanced parentheses in the destination and `<destination>` wrapping are supported. Optional Markdown title attributes and larger documents are handled only by Markdown, not by extracting a link card from them.
- Inferred HTTPS destinations are used for explicit page preview, browser opening and QR generation. Ordinary copying retains the exact saved bare domain or Markdown source. App scheme cards use a cream pass and peach sleeve, with the scheme above a two-line host/path excerpt; their preview defaults to QR, with the Link segment displaying the exact saved text and no WebView. Inferred domains and app links never trigger background HTTP icon/title/cover requests; matching metadata already present in copied HTML may still be used locally for inferred domains. Explicit unrestricted public HTTP(S) links retain the existing bounded metadata behavior.

### Link QR presentation

- App-link previews start in QR mode; web links start in Page and use Page / QR segments. Encoding happens only inside an explicitly opened preview, never during capture or card display. It re-reads the saved record and encodes its derived destination, including its original path/query/fragment, rather than a webpage's later navigation or redirect target.
- Bare domains receive HTTPS in the QR target; standalone Markdown links encode their destination; app links retain their scheme. Generation does not open the URL, access the network, write the clipboard or create history. Return/Copy still copies the exact original record with its saved protection scope.
- The target is limited to 2,048 UTF-8 bytes and the encoded symbol to 157 modules per side, including the four-module white quiet zone on each edge. The module limit can reject shorter targets. Symbols use medium error correction, black on white, integer scaling and at least two display pixels per module, with a 320-point display maximum. Both link types use opaque warm paper with a 12-point radius and 14-point inset; the body contains only the complete QR and a 22-point caption row separated by a 12-point gap. There is no scheme heading or icon above the QR. Unreadable/oversized targets show an error instead of a partial symbol.
- App scheme previews use QR / Link segments to switch between the code and exact source; web links use Page / QR. App previews and web QR use 520 × 500 points; web Page uses 720 × 500, subject to screen space. Web QR omits page navigation, favicon and domain from the toolbar. Copy with the exact saved source and any actual protection remain fixed below the QR paper. Closing or changing records clears the cached symbol and resets the mode: App links reopen as QR, web links as pages. Scanning an app link requires a compatible handler on the phone; QR encoding does not establish that the phone can open it.
- Switching a web link between Page and QR retains and hides the same already open WebView while the record, saved destination and popover host are unchanged; returning to Page does not reload it. QR never creates a page view. The hidden page is not paused and may continue its own loading and scripts. Closing the preview, changing records, hiding the panel, quitting or replacing the popover host releases the view.

### Markdown recognition and reading

- Parsing uses CommonMark structure with table, task-list and strikethrough extensions. At least one strong signal is required: a nonempty heading, a list with at least two items, a blockquote containing a letter, a nonempty fenced code block with a closing fence, a table, a task checkbox, image/link markup or bold emphasis. Alternatively, at least two parsed emphasis/code-span/strikethrough nodes suffice.
- `#tag`, a bare `#`/`*`/`---`, one ordinary list item, `> 42`, one italic span, one code span, indented text alone, unclosed/empty fences and HTML alone do not establish Markdown. They may still appear in a document with another sufficient signal. Existing whole-value types take priority; malformed or concatenated `-----BEGIN …` PEM candidates retain plain-text fallback.
- The document limit is 64 KiB after outer whitespace trimming, valid UTF-8, no NUL and at most 2,048 LF characters. The AST is bounded at 4,096 nodes and depth 32. Tables allow at most 16 columns and 256 rows including the header. Exceeding a bound keeps the entire original as ordinary text; no partial document is classified.
- Markdown cards have warm-white paper, a 10-point coral left edge and a dedicated M↓ glyph. They display up to three real blocks within the shared body: a leading heading and the first list entries before the next heading when available, otherwise the first fitting blocks. Headings use 18/500, body 15/400, with preserved list markers and quote/code treatments and table excerpts of up to two columns and three rows. The 600 × 460 Reading preview uses the same paper/edge, 18/500 headings and 17-point body text, with headings, paragraphs, quotes, ordered/unordered lists, task indicators, bold/italic/strikethrough, inline/fenced/indented code and tables; thematic breaks provide spacing. Character count, type and actual protection stay fixed outside the paper. Wide tables scroll horizontally instead of squeezing their columns. Inline links are colored reading text with no open action. Images show alt text, or the destination as literal text when no alt text exists; no image request occurs. Embedded HTML is shown literally, including script markup, and is never executed.
- Source shows the exact saved Markdown text in monospace with a 22-point code inset and scrolling in both directions, preserving indentation and long lines. Reading/Source is presentation-only, resets to Reading on closing/navigation and never changes the next copy. Both modes copy the complete original Markdown, preserving all markers, representations and allowed applications. A fenced JSON block is Markdown source and does not acquire JSON-formatting actions.

### Phones

- The whole input is 7–256 bytes. Main-number syntax permits digits, one leading `+`, balanced parentheses, spaces, hyphens and periods; it contains 7–18 digits including a possible international dialing prefix.
- Supported national/international numbering plans come from bundled libphonenumber metadata. Explicit international context takes precedence. The default context is the system/selected phone region captured with each record; changing preferences does not reinterpret existing records.
- Without any region, only clearly formatted national numbers are checked against the historical US/GB fallback; exactly one valid interpretation is required. Bare numbers do not trigger a country search.
- Extensions contain 1–10 digits and may use `ext`, `ext.`, `extension`, `x`, `#`, or `;ext=`. A `tel:` URI accepts only `ext` and numeric `phone-context`; duplicate/unknown parameters and domain contexts reject.
- Full-width digits/plus/parentheses/period, common Unicode spaces and common dash variants are supported. Line separators do not join separate numbers. IP addresses are not phones.

### Coordinate components

- Unlabeled components use an optional ASCII sign, 1–3 integer digits and a required fractional part. There is no exponential notation.
- Directional components use N/S for latitude and E/W for longitude, case-insensitively, with no separate sign. Plain directional degrees may be integers and may include `°`.
- Degree/minute forms require `°` and a minute mark (`'` or `′`); minute values may have a fractional part. DMS forms use integer minutes and seconds with an optional fractional part, followed by `"` or `″`.
- Minutes/seconds must be less than 60. At 90° latitude or 180° longitude, smaller components must be exactly zero. A complete directional pair is limited to 256 bytes.

### Colors

- RGB/HSL accept comma syntax or whitespace syntax with optional `/ alpha`; the two separator styles cannot be mixed. The explicit `rgba`/`hsla` names require an alpha component in the current implementation.
- HSL requires percentage saturation/lightness. HWB requires percentage whiteness/blackness and whitespace syntax. When HWB whiteness plus blackness is at least 100%, their ratio determines the gray value.
- The result is an eight-bit sRGB color. Existing explicit HEX/RGB/HSL copy choices remain available for every recognized color form. Component bounds are strict; out-of-range channels are rejected rather than silently clamped.
- HWB conversion follows [CSS Color 4](https://www.w3.org/TR/css-color-4/#hwb-to-rgb).

### Dates and times

- Year-first civil dates accept one/two-digit months and days. An optional time uses one/two-digit hours, two-digit minutes and optional two-digit seconds. Fractional seconds follow seconds with a dot or comma; parsed precision is nanoseconds. The date/time separator is `T` or one or more ASCII spaces.
- Zoned dates require two-digit months/days, `T`, seconds and either uppercase `Z` or a signed two-digit hour/minute offset. Offset hours are at most 23 and minutes at most 59. For compatibility with the existing Go parser, hours may have one digit and fractional seconds may use a comma, e.g. `2026-10-23T5:30:00,5Z`.
- All forms stay within the 8–40-byte input limit and preserve the original saved text, including any greater fractional precision.

### Verification-code labels

Labels are matched case-insensitively where applicable and are maintained in `resources/locales/content_labels.json`:

| Language | Labels |
| --- | --- |
| English | `verification code`, `security code`, `authentication code`, `2fa code`, `two-factor code`, `one-time code`, `one time code`, `one-time password`, `one time password`, `otp`, `code` |
| Simplified Chinese | 验证码、校验码、动态码、一次性密码 |
| Traditional Chinese | 驗證碼、校驗碼、動態碼、一次性密碼 |
| Japanese | 認証コード、確認コード、ワンタイムパスワード |

The label separator is whitespace or `:`, `：`, `=`, `＝`, with optional surrounding whitespace. Payload groups support ASCII spaces/hyphens, full-width digits and hyphens, nonbreaking/narrow nonbreaking/ideographic spaces, and Unicode hyphen/nonbreaking hyphen. CR/LF and other line-separator characters are not code grouping syntax.

### Tokens and private keys

The exact provider token patterns are:

| Prefix | Accepted suffix |
| --- | --- |
| `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_` | 30–255 ASCII letters/digits |
| `github_pat_` | 40–255 ASCII letters/digits/underscores |
| `sk-` | 20–255 ASCII letters/digits/underscores/hyphens, optionally preceded by `proj-` or `ant-api<digits>-`. These optional prefix characters can also count toward the ordinary suffix length. |
| `xoxb-`, `xoxa-`, `xoxp-`, `xoxr-`, `xoxs-` | 20–255 ASCII letters/digits/hyphens |
| `AKIA` | Exactly 16 uppercase ASCII letters/digits |

JWT recognition requires three unpadded Base64URL segments, a JSON header with a nonempty string `alg` other than `none`, a JSON-object payload whose decoded bytes start directly with `{`, and a decodable signature segment at least 16 encoded characters long. It does not validate the cryptographic signature. Bearer wrappers are whole-value, case-insensitive header/scheme wrappers around these already recognized credentials, following the request-header form described in [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750.html#section-2.1).

PEM block types are `PRIVATE KEY`, `RSA PRIVATE KEY`, `EC PRIVATE KEY`, `ENCRYPTED PRIVATE KEY`, `DSA PRIVATE KEY` and `OPENSSH PRIVATE KEY`. OpenSSH payloads must start with `openssh-key-v1` followed by a NUL byte. Public keys and certificates are intentionally not classified as private keys.

### URI-only clipboard data and size limits

- A URI-only fallback must be valid UTF-8, at most 1 MiB, with no NUL. LF and CRLF line endings, blank lines and `#` comment lines are supported.
- Once the higher-priority file/text/image/HTML choices provide no content, every non-comment line in the URI fallback must be a complete explicit HTTP(S) or supported app URI, at most 8,192 bytes, with no credentials and no whitespace/control characters. Bare domains and Markdown wrappers are not URI-list entries. At most 64 URLs are accepted; web/app mixtures are allowed, but invalid entries or any mixed file URI reject the entire fallback. Existing valid file-reference presentation still takes priority when the clipboard contains files.
- A single URL retains its path, query and fragment for presentation; multiple URLs are displayed as newline-separated ordinary text. The original URI-list bytes are retained for copying and persistence.
- Mixed file/web URI selections are checked across clipboard item boundaries as well as within each list. They never become a file-only or web-only subset. Available plain-text/HTML fallback retains the full saved representations; otherwise new unsupported mixed selections are rejected.
- Older mixed snapshots remain readable only when their former file-only annotation exactly matches the retained file subset. This restoration-only compatibility path preserves their display, provenance, pins and allowed applications; copying uses the entire original selection.
- On explicit copy, macOS's single-URL compatibility representation uses the first actual URL rather than a leading comment or blank line; the complete saved URI list stays intact.
- Existing capture limits still apply: 1 MiB derived text, 16 MiB total representation bytes per record, at most 64 clipboard items and 64 representations per item, and the 24 MP image limit.

## Further expansion requiring separate work

JPEG-only/WebP/GIF and RTF-only clipboard data need bounded native acquisition and compatible representation/preview handling. JSON5/NDJSON and JSON formatting inside Markdown fences need explicit parsing/copy semantics. Standalone `<https://example.com>` wrappers, relative paths, ambiguous dates, naked verification codes and arbitrary long secrets are not inferred as their specialized semantic types. Full Markdown editing, executable inline links, remote Markdown images and HTML rendering are not part of the native reading preview.
