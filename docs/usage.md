# Usage

Copy normally with **⌘C**. Open Copiggy from its menu-bar icon or with **⌃⌥⇧Space** to browse the history. Selecting, searching and previewing a record leave the system clipboard unchanged.

Press **Return** or double-click a card to copy the selected content and close the panel. Without an explicit format choice, Return copies the exact original. If you explicitly choose a supported format, Return copies that format instead. Paste in your destination using **⌘V**; Copiggy does not paste automatically.

## Shortcuts

| Action | Shortcut or control |
| --- | --- |
| Show / hide the panel | ⌃⌥⇧Space or menu-bar icon |
| Search | ⌘F or search button |
| Navigate cards | ← / → |
| Return from search to cards | ↓ |
| Preview selected card | Space or ⌘Y |
| Copy selected content / chosen format | Return |
| Pin / unpin | ⌘P or pin control |
| Delete selected record | ⌘⌫ or preview delete control |
| Close preview / search / panel | Esc |
| Copy with source-app protection | ⌃⌘C, while protection is enabled |

Arrow keys retain normal text-editing behavior while the search field has focus. The top pin button switches between all history and pinned items. Right-click the menu-bar icon for recording, history retention, appearance, language and quit options. Menu items show icons and native shortcuts. Only protection updates and errors appear beside the menu-bar icon; hover and routine successes stay quiet.

When history is empty, Copiggy's pig occasionally blinks while three short lines appear one by one on its clipboard, pause, then fade. The empty pinned list shows the same pig pressing a pin into a note, with a quiet rest between gestures. Both stay still when macOS Reduce Motion is enabled. The menu-bar pig holds a small clipboard and gains a shield when Paste Guard is active.

## Content previews

- **Text:** read the full saved text.
- **JSON:** choose Format, then Return or Copy result to copy the draft. Compare with Original or choose Copy original to keep the captured representation. Cancel or navigate away to discard the draft. Neither copying a result nor cancelling adds a history record.
- **Images:** edge-to-edge image cards with small glass metadata labels. Enlarge, zoom or fit the image in preview.
- **Links:** cards use matching copied metadata when available, or fetch a public page's title and cover when its card comes into view. Missing covers fall back to a website icon and address. Preview loads the actual page; its toolbar offers navigation, refresh/stop and opening in the default browser.
- **Email:** a blue envelope card with the address split into name and domain. Mail opens a draft in the registered mail app.
- **Phone:** a mint card with the number and a subtle dial pad. Choose Dialing number, then Return to copy the normalized number. Original number restores exact-original copying. Call opens the registered phone handler.
- **Coordinates:** a map card with a marker. Preview zooms the map and shows the coordinate pair; Maps opens the location externally.
- **Date:** an integrated calendar card with a compact coral month band. Preview shows the date, weekday and time. Calendar opens an importable event file; the calendar app handles confirmation.
- **Timestamp:** a dark counter card with a readable date. Choose seconds or milliseconds, then Return to copy that unit. Local/UTC changes only the date explanation. Copying seconds from milliseconds drops the subsecond digits; the saved original keeps them.
- **Color:** the color fills the card. Choose HEX, RGB or HSL, then Return to copy that representation. Transparent colors appear over a checkerboard.
- **File path:** a folder card with the filename and parent directory. Finder reveals the local item without executing it.
- **Verification code:** a lavender card with all digits immediately visible.
- **Token:** a dark card with three rows showing the first and last six characters and masking the middle, plus a format label such as JWT. Reveal shows the full saved value.
- **Private key:** an ivory sealed-paper card with four rows showing the first and last six characters of its encoded payload and masking the middle. Reveal shows the full saved value.

Choosing a format changes the next explicit copy, not the clipboard immediately. Copied variants are plain text, keep the saved record’s protection scope, and never replace the original or add another history item. Closing the preview or navigating to another record resets the choice; reopen and press Return without choosing a format to copy the exact original. Reveal and zoom do not change what is copied. Token and private-key previews return to the abbreviated value when closed or changed. Masking is separate from Paste Guard: source-app restrictions appear only when the record actually has them.

Recognition matches whole values conservatively. It supports complete email addresses, international or grouped phone numbers, decimal latitude/longitude pairs, ISO dates, 10/13-digit Unix timestamps within 2000–2099, HEX/RGB/HSL colors, local absolute or `~/` paths, labeled verification codes, known token prefixes/JWTs and complete PEM private keys. Ambiguous values remain ordinary text; a bare six-digit number is not assumed to be a verification code. Restored text records are recognized again without altering their saved content.

Exact repeated content with the same protection scope updates the existing record's time and moves it to the front. Records with different protection scopes stay separate. Deleting a history record does not clear the system clipboard or remove protection already bound to its current contents.

## Protected copy

Enable the shield toggle and grant macOS Accessibility permission if requested. In the source application, select content and press **⌃⌘C**. Copiggy performs that copy and binds it to the source application. The source icon gains a lock badge; a brief menu-bar message confirms the restriction.

While the guard is active, standard **⌘V** and **⌘⌥⇧V** are blocked in other applications. Ordinary **⌘C** does not inherit a previous copy's restriction. The menu-bar icon and shield toggle show the current guard state.

This protects against accidental keyboard paste. It does not prevent other applications from reading the clipboard, menu or context-menu paste, custom paste shortcuts, or pasting into the wrong conversation inside the same application. Protection stops when the app quits or its native listener loses permission.

## Storage and network behavior

History is saved locally in `~/Library/Application Support/Copiggy/history.json`, including its exact clipboard content, source, time, pin state and allowed applications. Existing `pinned.json` records are migrated on first use of this version. The previous pin file remains as a migration backup and is not read again after the unified history file exists.

Use the menu-bar **Keep History** setting to choose **1 day**, **7 days**, **30 days** (default) or **Forever**. Pin records to exclude them from age expiry. Up to 100 ordinary records and 100 pins are kept, with a shared 64 MiB content budget; “Forever” removes the age limit, not these size limits. Deletions, unpins and recopy timestamps are saved. Shortening the retention period removes expired ordinary records. Expiry runs at startup, when opening the panel and periodically while running, including while recording is paused.

Language, explicit light/dark choices, recording pause, the Paste Guard preference and retention are saved together in `preferences.json`. Changing one setting preserves the others. Older settings files default to recording on, Paste Guard requested on and 30-day retention; a missing theme uses the system appearance until one is chosen. On restart, Paste Guard is enabled only if requested and macOS permission allows it; the shield always shows the actual listener state.

Restoring history never writes the system clipboard or restores an old clipboard revision's protection. Copying a saved restricted item binds its saved allowed applications to the new write. Search, selection, open previews, format choices and unfinished enhancements are not restored.

JSON formatting and content recognition run locally. Masking does not encrypt stored history; credentials retain the same history retention and deletion controls as other records.

Link title and cover retrieval starts lazily for visible or nearby unrestricted public cards, never during clipboard capture. It requests the saved address including its path and query, strips the fragment, and reads Open Graph or Twitter metadata and its cover image. Protected records and private or local addresses are not fetched. These requests use no browser cookies, authentication or referrers, and execute no scripts. Website icons keep a separate origin-only retrieval path that omits the copied path and query.

Link metadata and map snapshots share a 24-entry, 24 MiB memory cache. Link retrieval uses at most three workers; covers expire after six hours, and failed or coverless results after ten minutes. Hiding the panel or quitting cancels pending work. This cache is not saved to disk.

Opening a webpage preview explicitly loads the full saved address, including its path and parameters, in Copiggy's WebKit session. Unlike metadata retrieval, this uses WebKit's normal persistent website data store, including its cookies; it is not private browsing.

Visible unrestricted coordinates request system map snapshots from Apple Maps. This sends the coordinate region, not your device's location; it does not request location permission. Protected coordinates never request a snapshot automatically. Map images stay in a bounded memory cache; unavailable maps fall back to the coordinates. Explicit Mail, Call and Maps actions pass their displayed value to the corresponding handler. There are no cloud AI calls or bulk history uploads.
