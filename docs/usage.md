# Usage

Copy normally with **⌘C**. Open Copiggy from its menu-bar icon or with **⌃⌥⇧Space** to browse the history. Selecting, searching and previewing a record leave the system clipboard unchanged.

Press **Return** or double-click a card to copy that saved record back to the clipboard and close the panel. Paste in your destination using **⌘V**. Copiggy does not paste automatically.

## Shortcuts

| Action | Shortcut or control |
| --- | --- |
| Show / hide the panel | ⌃⌥⇧Space or menu-bar icon |
| Search | ⌘F or search button |
| Navigate cards | ← / → |
| Return from search to cards | ↓ |
| Preview selected card | Space or ⌘Y |
| Copy selected record / accept enhancement | Return |
| Pin / unpin | ⌘P or pin control |
| Delete selected record | ⌘⌫ or preview delete control |
| Close preview / search / panel | Esc |
| Copy with source-app protection | ⌃⌘C, while protection is enabled |

Arrow keys retain normal text-editing behavior while the search field has focus. The top pin button switches between all history and pinned items. Right-click the menu-bar icon for recording, history retention, appearance, language and quit options. Menu items show icons and native shortcuts. Only protection updates and errors appear beside the menu-bar icon; hover and routine successes stay quiet.

When history is empty, Copiggy's pig occasionally blinks while three short lines appear one by one on its clipboard, pause, then fade. The empty pinned list shows the same pig pressing a pin into a note, with a quiet rest between gestures. Both stay still when macOS Reduce Motion is enabled. The menu-bar pig holds a small clipboard and gains a shield when Paste Guard is active.

## Content previews

- **Text:** read the full saved text.
- **JSON:** preview formatting as a draft. Confirm to copy the result; cancel or navigate away to discard it. The source record and its protection scope are retained.
- **Images:** preview the image and dimensions, with an enlargement action.
- **Links:** open the actual webpage inside the preview. The toolbar includes navigation, refresh/stop and opening in the default browser. Copying the record still copies its original saved address.

Exact repeated content with the same protection scope updates the existing record's time and moves it to the front. Records with different protection scopes stay separate. Deleting a history record does not clear the system clipboard or remove protection already bound to its current contents.

## Protected copy

Enable the shield toggle and grant macOS Accessibility permission if requested. In the source application, select content and press **⌃⌘C**. Copiggy performs that copy and binds it to the source application. The source icon gains a lock badge; a brief menu-bar message confirms the restriction.

While the guard is active, standard **⌘V** and **⌘⌥⇧V** are blocked in other applications. Ordinary **⌘C** does not inherit a previous copy's restriction. The menu-bar icon and shield toggle show the current guard state.

This protects against accidental keyboard paste. It does not prevent other applications from reading the clipboard, menu or context-menu paste, custom paste shortcuts, or pasting into the wrong conversation inside the same application. Protection stops when the app quits or its native listener loses permission.

## Storage and network behavior

History is saved locally in `~/Library/Application Support/Copiggy/history.json`, including its exact clipboard content, source, time, pin state and allowed applications. Existing `pinned.json` records are migrated on first use of this version. The previous pin file remains as a migration backup and is not read again after the unified history file exists.

Use the menu-bar **Keep History** setting to choose **1 day**, **7 days**, **30 days** (default) or **Forever**. Pin records to exclude them from age expiry. Up to 100 ordinary records and 100 pins are kept, with a shared 64 MiB content budget; “Forever” removes the age limit, not these size limits. Deletions, unpins and recopy timestamps are saved. Shortening the retention period removes expired ordinary records. Expiry runs at startup, when opening the panel and periodically while running, including while recording is paused.

Language, explicit light/dark choices, recording pause, the Paste Guard preference and retention are saved together in `preferences.json`. Changing one setting preserves the others. Older settings files default to recording on, Paste Guard requested on and 30-day retention; a missing theme uses the system appearance until one is chosen. On restart, Paste Guard is enabled only if requested and macOS permission allows it; the shield always shows the actual listener state.

Restoring history never writes the system clipboard or restores an old clipboard revision's protection. Copying a saved restricted item binds its saved allowed applications to the new write. Search, selection, open previews and unfinished enhancements are not restored.

JSON formatting and clipboard processing run locally. Website icons are fetched only for unrestricted public links, using a bounded cache and the site's origin rather than the copied path or query. Opening a webpage preview explicitly loads the full saved address, including its path and parameters, in Copiggy's WebKit session. There are no cloud AI calls or history uploads.
