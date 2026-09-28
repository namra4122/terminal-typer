# Terminal Typer (`tt`)

`tt` is a local, keyboard-driven typing test for the terminal. The current source checkout runs on Go and `tcell`. It supports generated words, quotes, files, and piped text, with themes, optional sounds, saved live controls, and basic results. It does not require an account or send test data over the network.

![Terminal Typer demonstration](demo.gif)

The demonstration is illustrative; the current source and manual define behavior.

## Current checkout and roadmap

The executable still reports version `0.4.2`. The source checkout also contains work beyond the historical `0.4.2` release notes, including the `Ctrl-P` Settings modal and shared `tcell` layout and theme helpers. The prebuilt release commands below refer to the `v0.4.2` artifacts; build from source to use the current checkout.

| Available in this checkout | Proposed in the [five-phase roadmap](.agents/plans/README.md) |
| --- | --- |
| English 1000-word default, other bundled word lists, quotes, custom words, files, and stdin | Hinglish and programming-language content packs; explicit time, word, custom, and practice modes |
| Optional timer, live WPM, word skipping, backspace controls, themes, and key/error sounds | Raw WPM and other live widgets; focus and tape modes; command palette; more theme and sound controls |
| Final WPM, CPM, accuracy, mistakes, JSON/CSV output, locally saved mistakes, and durable Charm result History | Detailed graphs, personal bests, adaptive practice, keyboard layouts, presets, and Funbox |

The [plan index](.agents/plans/README.md) labels implemented records, design references, and proposed work. A feature listed only in a plan is not available in the executable.

## Build from source

Go 1.26.0 or newer is required. The opt-in contributor renderer also uses Bubble Tea v2 and Lip Gloss v2; the default renderer remains the existing `tcell` implementation.

```sh
go mod download
make build
./bin/tt
```

`make install` installs the binary and manual under `/usr/local`; override `PREFIX` or `DESTDIR` as needed. Installing the manual requires Pandoc and gzip. Contributors should run `make verify` and `make smoke`; see [CONTRIBUTING.md](CONTRIBUTING.md).

For the opt-in Charm typing screen, run `TT_UI=charm ./bin/tt -n 10 -g 2` from an interactive terminal. This route supports terminal word tests and the `-n`, `-g`, `-t`, and six live-setting overrides; other invocations continue through the existing renderer.

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

Running `tt` starts a test using 50 randomly selected words from the bundled `1000en` list. `-n` changes words per group and `-g` changes the number of groups. Adjacent duplicate words are avoided within each generated group. `-t` limits a test to a number of seconds; it does not provide named duration presets yet.

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
- `Ctrl-P` opens Settings during an active test and pauses its timer. `Up` and `Down` select a row, `Space` or `Enter` changes it, and `Escape` or `Ctrl-P` saves and returns to the same test.
- Backspace corrects input; `Ctrl-W`, `Ctrl-Backspace`, or `Alt-Backspace` deletes a word where the terminal reports those keys.

Settings currently contains Show WPM, Skip word on Space, Allow Backspace, Cursor style, Typed text weight, and Word highlighting. Saved values go to `$XDG_DATA_HOME/tt/settings.json`, or `~/.local/share/tt/settings.json` if `XDG_DATA_HOME` is unset. Explicit `-showwpm`, `-noskip`, `-nobackspace`, `-blockcursor`, `-bold`, `-nohighlight`, `-highlight1`, and `-highlight2` flags override the matching saved control for that invocation without rewriting it.

### Results and local resources

The completed-test Results screen reports effective and raw WPM, CPM, input accuracy, consistency, error totals, active/paused time, configuration, source attribution, retry status, eligibility reasons, and local save status. Speeds are unavailable when active duration is zero; short samples are labeled. A retry reuses the same prompt and is PB-ineligible; Enter starts the next test. Press `h` on Results to open History, `t` to toggle regular/practice results, arrows to select or page, and Escape to return to the same result. Failed saves leave Results visible and offer `s` to retry with the same result ID.

Rich Charm results are retained indefinitely as one versioned JSON file per session under `$XDG_DATA_HOME/tt/history-v1`, or `~/.local/share/tt/history-v1` if `XDG_DATA_HOME` is unset. Session files are authoritative; `index.json` is rebuilt automatically when missing or stale. Embedded prompts may retain bounded mistake fragments for analysis. Local word lists, files, stdin, complete prompts, complete typed responses, raw input events, and source paths are not retained in rich history. If an authoritative store or session file is malformed, History reports its exact path and does not rewrite it; typing and new in-memory Results remain available. If `.write-lock` remains after a crash, verify that no `tt` writer is active before removing only that lock directory. `-json` and `-csv` continue to print the existing legacy result schema at process exit. Existing mistakes and file progress remain in their prior files.

Themes, word lists, quotes, and sounds can come from an explicit path, a matching directory under `~/.tt` or `/etc/tt`, or embedded resources, in that order. List embedded names with `tt -list themes`, `tt -list words`, `tt -list quotes`, or `tt -list sounds`. `-sound` and `-error-sound` accept WAV or MP3 resources. A terminal with truecolor and cursor-shape support shows the intended styling most faithfully.
