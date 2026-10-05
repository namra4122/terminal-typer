# Terminal Typer (`tt`)

`tt` is a local, keyboard-driven typing test for the terminal. Bare launches use Charm; explicit legacy test inputs retain the `tcell` route. It supports timed and word-count tests, quotes, files, and piped text, with local settings and results. It does not require an account or send test data over the network.

![Terminal Typer demonstration](demo.gif)

The demonstration is illustrative; the current source and manual define behavior.

## Current checkout and roadmap

The executable still reports version `0.4.2`. The source checkout also contains work beyond the historical `0.4.2` release notes, including the `Ctrl-P` Settings modal and shared `tcell` layout and theme helpers. The prebuilt release commands below refer to the `v0.4.2` artifacts; build from source to use the current checkout.

| Available in this checkout | Proposed in the [roadmap](.agents/plans/README.md) |
| --- | --- |
| Remembered timed, count, and quote tests; bundled word lists, custom words, files, and stdin | More content packs and advanced modifiers |
| Live WPM, word skipping, backspace controls, themes, and optional legacy key/error sounds | Command palette, focus and additional sound controls |
| Final measurements, JSON/CSV output, locally saved mistakes, durable Charm History, and weakness practice | Detailed graphs, personal bests, adaptive training, keyboard layouts, presets, and Funbox |

The [plan index](.agents/plans/README.md) labels implemented records, design references, and proposed work. A feature listed only in a plan is not available in the executable.

## Build from source

Go 1.26.0 or newer is required. Bare and presentation-only launches use Bubble Tea v2 and Lip Gloss v2; explicit test-defining flags, files, and piped input retain the existing `tcell` renderer. Set `TT_UI=legacy` to temporarily use the original renderer for a bare launch.

```sh
go mod download
make build
./bin/tt
```

`make install` installs the binary and manual under `/usr/local`; override `PREFIX` or `DESTDIR` as needed. Installing the manual requires Pandoc and gzip. Contributors should run `make verify` and `make smoke`; see [CONTRIBUTING.md](CONTRIBUTING.md).

`TT_UI=charm ./bin/tt -n 10 -g 2` still opts a supported explicit terminal word test into Charm. Other explicit inputs retain the original renderer.

## Prebuilt 0.4.2 release

These commands fetch the historical `v0.4.2` release, whose behavior may differ from this source checkout.

### Linux

```sh
sudo curl -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt-linux -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -o /usr/share/man/man1/tt.1.gz -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt.1.gz
```

### macOS

```sh
mkdir -p /usr/local/bin /usr/local/share/man/man1
sudo curl -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt-osx -o /usr/local/bin/tt && sudo chmod +x /usr/local/bin/tt
sudo curl -o /usr/local/share/man/man1/tt.1.gz -L https://github.com/lemnos/tt/releases/download/v0.4.2/tt.1.gz
```

To remove those installations, delete the installed `tt` binary and `tt.1.gz` manual from their respective directories.

## Use the current checkout

Running `tt` starts a fresh, remembered 30-second test using the bundled `1000en` list. The timer starts with accepted typing, not at launch. Press `Ctrl-K` to Configure a timed test (15/30/60/120 seconds or 5-3600 custom), a word-count test (10/25/50/100 words or 1-500 custom), or an authored quote. Start saves the selection and generates fresh material; browsing and Cancel do not save. Timed tests extend their word buffer while typing and Retry reuses the generated prompt prefix. Explicit `-n`, `-g`, `-t`, `-words`, `-quotes`, `-start`, `-raw`, `-multi`, a positional file, or piped stdin use legacy test resolution without inheriting the saved timer or modifiers.

```sh
./bin/tt -n 10 -g 5
./bin/tt -t 30
./bin/tt -quotes en
./bin/tt -theme gruvbox
./bin/tt path/to/file.txt
cat path/to/text.txt | ./bin/tt
```

`-words` selects a bundled or local word list. `-quotes` selects a bundled or local JSON quote file. A positional file is split into paragraph-based tests; `-start` selects the starting paragraph and `-start 0` resets its saved position. Piped stdin is accepted as custom text. `-multi` treats each input paragraph as a separate test; `-raw` preserves the input's line breaks instead of reflowing text. See [man.md](man.md) or `tt -help` for every flag.

### Keys and Settings

- `Escape` restarts the current test.
- `Left` and `Right` move between tests.
- `Ctrl-C` exits; `Ctrl-L` refreshes the terminal.
- `Ctrl-K` opens Configure in Charm. `Tab` and `Shift-Tab` change groups; arrows select rows; `Space` or `Enter` changes a control. Start saves intentional choices and begins a new test. Cancel returns to the same session without writing settings. Replacing a test after meaningful input requires confirmation.
- `Ctrl-P` opens Settings during an active test and pauses its timer. `Up` and `Down` select a row, `Space` or `Enter` changes it, and `Escape` or `Ctrl-P` saves and returns to the same test.
- Backspace corrects input; `Ctrl-W`, `Ctrl-Backspace`, or `Alt-Backspace` deletes a word where the terminal reports those keys.

The Configure groups are Test, Content, Typing, Display, Sound, Data, and Help. Only implemented controls are actionable. Reset test, appearance, or a section without touching history; reset-all requires confirmation and also leaves history untouched.

Settings contains Show WPM, Skip word on Space, Allow Backspace, Cursor style, Typed text weight, and Word highlighting. Saved values and remembered test configuration go to `$XDG_DATA_HOME/tt/settings.json`, or `~/.local/share/tt/settings.json` if `XDG_DATA_HOME` is unset. Explicit live-setting flags override the matching saved control only for that invocation. Existing version-1 settings are migrated with a verified private backup under `backups/`; corrupt or unknown settings stay unchanged and use read-only defaults until repaired. To roll back to older code, restore a verified version-1 backup into a separate data root; older code cannot read the version-2 file.

### Results and local resources

The completed-test Results screen reports effective and raw WPM, CPM, input accuracy, consistency, error totals, active/paused time, configuration, source attribution, retry status, eligibility reasons, and local save status. Speeds are unavailable when active duration is zero; short samples are labeled. A retry reuses the same prompt and is PB-ineligible; Enter starts the next test. Press `h` on Results to open History, `t` to toggle regular/practice results, arrows to select or page, and Escape to return to the same result. Failed saves leave Results visible and offer `s` to retry with the same result ID.

On Charm, press `p` after a regular test to review measured weak words when suitable evidence exists. Review shows the reasons, sample window, and current/history contributions; `Up`/`Down` select, `Delete` dismisses, `r` restores, `Enter` starts the 25-word drill, and `Escape` cancels without recording another test. The drill interleaves 15 weak-word/context slots with 10 neutral words from the matching embedded word list. Results compare item accuracy, error rate, and median time per Unicode scalar. Speed change is unavailable unless both baseline and drill include at least three complete occurrences. Press `a` to practice the same curriculum again with a fresh shuffle, `n` to generate a fresh regular test with the previous configuration, or `Escape` to return to the originating result.

Rich Charm results are retained indefinitely as one versioned JSON file per session under `$XDG_DATA_HOME/tt/history-v1`, or `~/.local/share/tt/history-v1` if `XDG_DATA_HOME` is unset. Session files are authoritative; `index.json` is rebuilt automatically when missing or stale. Embedded prompts may retain bounded per-word occurrence summaries for practice; private input contributes no word candidates. Local word lists, files, stdin, complete prompts, complete typed responses, raw input events, and source paths are not retained in rich history. If an authoritative store or session file is malformed, History reports its exact path and does not rewrite it; typing and new in-memory Results remain available. If `.write-lock` remains after a crash, verify that no `tt` writer is active before removing only that lock directory.

Themes, word lists, quotes, and sounds can come from an explicit path, a matching directory under `~/.tt` or `/etc/tt`, or embedded resources, in that order. List embedded names with `tt -list themes`, `tt -list words`, `tt -list quotes`, or `tt -list sounds`. `-sound` and `-error-sound` accept WAV or MP3 resources. A terminal with truecolor and cursor-shape support shows the intended styling most faithfully.
