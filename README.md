# vocab2anki

Select an English word in your browser or on your iPhone and it lands in Anki as a **sentence-mining cloze card** — a real sentence with the target word blanked out, backed by an LLM-written definition, example sentences, IPA, and pronunciation audio — automatically.

```
PC browser (userscript)         -┐
                                 ├─▶  vocab2anki daemon (Mac)  ─▶  Anki desktop (AnkiConnect)  ─▶  AnkiMobile
iPhone (Shortcut → iCloud file) ─┘        │
                                          ├─ Claude: cloze sentence + definition
                                          ├─ Free Dictionary API: IPA + audio
                                          ├─ Youdao: audio fallback
                                          └─ macOS `say`: last-resort audio
```

A single Go binary runs on your Mac. Two thin clients only need to "send a word"; all enrichment and card building happen in the daemon.

## How it works

1. **Intake.** A word arrives two ways:
   - The **browser userscript** POSTs `{word, context, source}` to `http://127.0.0.1:8766/add` (it also grabs the sentence around your selection, which sharpens the generated examples).
   - The **iOS Shortcut** appends the word to a file on iCloud Drive; the daemon watches that file and imports each new line.
2. **Enrich.** Claude produces a cloze sentence (your context sentence when present, otherwise a natural sentence it writes) with the target word wrapped in `{{c1::...}}`, plus a learner-friendly definition, part of speech, extra example sentences, and common collocations. By default this runs through your local `claude` command (Claude Code CLI) — no API key required — but you can switch to the Anthropic HTTP API instead (see _Choosing the Claude backend_). The Free Dictionary API supplies the IPA and a human-recorded pronunciation mp3.
3. **Audio.** Three sources are tried in order of quality: the Free Dictionary API's human recording, then Youdao's pronunciation service (`type=2` is the US voice; use `type=1` for UK), then local synthesis with macOS `say`. The daemon logs which one it used.
4. **Add.** A **Cloze note** is added to Anki via AnkiConnect, with audio stored in the media collection. Duplicates (by word) are skipped. If Anki is closed, the word is parked in a pending file and retried until Anki reopens.

The card **front** is the sentence with the word blanked out (sentence mining), e.g. _"The detective was determined to `[...]` the mystery."_ The **back** reveals the word plus its IPA, auto-played pronunciation, definition, extra examples, and the source link. Light and dark modes are both styled.

## Prerequisites

- **Anki desktop** with the **AnkiConnect** add-on (code `2055492159`). Keep Anki running for real-time adds; it's fine if it's occasionally closed.
- **A Claude backend** — either the **`claude` CLI** logged in (the default; no API key needed) or an **`ANTHROPIC_API_KEY`** if you switch to the API backend.
- **Go 1.22+** to build (only the compiled binary is needed to run).
- **Tampermonkey** (or another userscript manager) in your browser.

### Choosing the Claude backend

Set `[claude].provider` in `config.toml`:

- `provider = "cli"` (default) — runs the local `claude` command in print mode,
  reusing your existing Claude login. No API key. Set `model` to an alias like `"haiku"`.
- `provider = "api"` — calls the Anthropic HTTP API. Requires `ANTHROPIC_API_KEY` in the environment (put it in `.env`). Set `model` to a full id like`"claude-haiku-4-5"`.

## Setup

```sh
make build                    # produces ./bin/vocab2anki
make run                      # run in the foreground (uses ./config.toml)
```

The default CLI backend needs no key — just make sure `claude` is logged in.
If you use `provider = "api"`, first `cp .env.example .env` and set your key.

Adjust `config.toml` if you want a different deck name, port, or paths.

### Run it as a background service (launchd)

```sh
make install     # installs the binary + config, renders the launchd plist
                 # with your API key and paths, and loads it (starts on login)
make logs        # tail ~/Library/Logs/vocab2anki.log
make uninstall   # stop and remove the service
```

The daemon runs with the **installed** config at `~/.config/vocab2anki/config.toml`. `make install` refreshes that copy from the repo's `config.toml` every time (it deletes the old one first), so edit `config.toml` in the repo and re-run `make install` to apply changes — don't hand-edit the installed copy, it will be overwritten.

### Browser userscript

Install `clients/vocab2anki.user.js` in Tampermonkey. Then on any page: select a word → click the little **+ Anki** button (or press **Option+A**). A toast confirms it was queued.

### iPhone

Install the Shortcut from the link in `clients/ios-shortcut.md`. Then: select a word in Safari → **Share** → **Save to Anki**. Cards sync to AnkiMobile after the Mac processes them.

## Configuration

`config.toml` (paths may start with `~`):

| Key                                       | Meaning                                                      |
| ----------------------------------------- | ------------------------------------------------------------ |
| `server.listen`                           | Address of the local HTTP intake (default `127.0.0.1:8766`). |
| `anki.url` / `anki.deck` / `anki.model`   | AnkiConnect endpoint, target deck, note type.                |
| `claude.model` / `claude.max_tokens`      | Generation model and token cap.                              |
| `queue.file`                              | iCloud Drive file the iOS Shortcut appends to.               |
| `pending.file` / `pending.retry_interval` | Offline-retry store and retry cadence (seconds).             |
| `audio.dir`                               | Where pronunciation audio is cached before import.           |

`ANTHROPIC_API_KEY` is read from the environment, never the config file.

## Development

```sh
make test    # go test ./...
make fmt     # gofmt -w .
make tidy    # go mod tidy
```

Layout: `cmd/vocab2anki` (entry point) wires the `internal/` packages — `server` (HTTP intake), `queue` (iCloud file watcher), `enrich` (Claude + dictionary), `audio` (download/`say`), `anki` (AnkiConnect client), `pending` (offline retry), and `process` (the orchestration that ties them together).

## Notes and limits

- Words are lemmatized to their base dictionary form before saving (e.g.
  "running"/"ran" → "run"), and IPA/audio/dedup use that base form. Turn this off
  with `[claude].lemmatize = false` to store words exactly as sent.
- Re-adding an existing word is skipped (context isn't merged into the old card yet).
- Audio falls back to macOS `say`, so that last tier is macOS-only. In practice Youdao covers almost every headword, so `say` is rarely reached.
- The daemon binds to loopback only; nothing is exposed to your network.
