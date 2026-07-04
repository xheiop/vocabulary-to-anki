# iOS Shortcut: "Save to Anki"

On iOS, words are sent by a Shortcut that appends each selected word to a file on iCloud Drive. The Mac daemon watches that file and imports every new line.

## Install

1. On your iPhone, open this link and tap **Add Shortcut** / **Set Up Shortcut**:

   <https://www.icloud.com/shortcuts/6795411007db4be3a43e340891462219>

2. If iOS asks you to grant access, allow it to write to **iCloud Drive**.

The shortcut appends to
`iCloud Drive/vocab2anki/vocab-queue.txt`, which on the Mac is:

```
~/Library/Mobile Documents/com~apple~CloudDocs/vocab2anki/vocab-queue.txt
```

This is already the `queue.file` value in `config.toml`. (If you ever change the
file path inside the Shortcut, update `queue.file` to match.)


## Use it

- In Safari, select a word → **Share** → **Save to Anki**.
- Or long-press a word → **Share** → **Save to Anki**.
- You can also run it from the Home Screen / widget and type a word manually.

The word lands in the iCloud file within seconds. When your Mac is online and running `vocab2anki`, the daemon drains the file, builds the card, and adds it to
Anki; it then syncs to AnkiMobile on the phone for review.

## Verify

1. On the Mac, make sure `vocab2anki` is running (`make run` or the launchd service) and Anki is open.
2. Run the shortcut once with a test word.
3. Watch the log — you should see the word imported:
   ```sh
   make logs   # or: tail -f ~/Library/Logs/vocab2anki.log
   ```
   Look for a line like `intake(icloud): "serendipity"`.

## Notes

- The daemon accepts either a bare word per line or `word<TAB>timestamp`, so the exact line format the Shortcut writes doesn't matter.
- Nothing happens instantly on the phone — enrichment runs on the Mac. If the Mac is asleep/offline, words wait safely in the file until iCloud syncs.
- The Mac clears the file after importing, so it won't grow without bound.
- Duplicates are harmless: the daemon skips words already in the deck.
