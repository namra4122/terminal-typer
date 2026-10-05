% tt(1)

# NAME

tt - A terminal based typing test

# SYNOPSIS

usage: tt \[OPTION\]... \[FILE\]

# DESCRIPTION

  This manual describes the current source checkout. The executable still
  reports version 0.4.2, but the source checkout includes changes beyond the
  historical release notes.

  By default tt starts a fresh remembered 30-second typing test using the
  embedded English 1000-word list. The timer starts on accepted text. Ctrl-K
  opens Configure: choose timed (15/30/60/120 or custom 5-3600 seconds), count
  (10/25/50/100 or custom 1-500 words), or an authored quote. Start saves the
  choice and begins fresh content; Cancel does not save. After meaningful input,
  Start requires replacement confirmation. Timed content extends as necessary.
  If provided with a path, tt will
  use the given file as input treating each paragraph as a separate segment of
  the test. The program will automatically keep track of your position in the
  file so subsequent invocations on the same path will place you at the most
  recent paragraph (-start 0 can be used to reset your position).  
  
  Arbitrary text can also be piped directly into the program to create a custom
  test. Each paragraph of the input is treated as a segment unless '-multi' is
  supplied in which case each paragraph is treated as a separate test. 

  Source builds require Go 1.26.0 or newer. Bare and presentation-only
  launches use Charm. Explicit test-defining flags, files and stdin retain the
  legacy renderer without inheriting a saved timer or modifiers. TT_UI=legacy
  temporarily selects the old renderer for bare launches; TT_UI=charm can
  still opt supported explicit word tests into Charm.

  On the Charm route, completed tests show effective/raw WPM, CPM, input
  accuracy, consistency, error totals, active and paused time, configuration,
  eligibility, available source/retry details, and local save status. Zero-
  duration speeds are unavailable; short samples are labeled. Results are
  written asynchronously to durable local History; the existing process-exit
  JSON/CSV schemas are unchanged. Consistency uses active-time interval raw-WPM
  rates, which can differ from final retained-character speeds.

  On regular Charm Results, **p** opens a keyboard-only review of measured
  weak words and their recent sample when suitable embedded-word evidence
  exists. **up**/**down** select an item; **delete** dismisses it, **r**
  restores it, **enter** starts a 25-word drill, and **esc** cancels without
  recording a test. The drill mixes 15 weak-word slots with 10 neutral words
  from the matching embedded word list. Completed practice is PB-ineligible
  and appears only in practice History by default. Per-item accuracy, error
  rate, and median time per Unicode scalar are compared with the baseline;
  speed change is unavailable if either side has fewer than three complete
  occurrences. On practice Results, **a** starts a fresh shuffle of the
  same curriculum, **n** starts a fresh regular test with the previous
  configuration, and **esc** restores the originating result. Explicit
  quote sources still use the original renderer.


  Charm Ctrl-K Configure has Test, Content, Typing, Display, Sound, Data and
  Help groups. Tab/Shift-Tab change groups; arrows select rows; Space/Enter
  change available controls. Reset-all requires confirmation and leaves
  History untouched. Ctrl-P retains the six live controls. Version-1 settings
  are migrated to version 2 after writing a verified private backup under
  the data directory's backups/ folder. Invalid or unknown settings are left
  intact and use read-only defaults until repaired. To run older code again,
  manually restore a verified version-1 backup in a separate data root;
  older code cannot read version-2 settings.

# OPTIONS

## Modes

-words  *WORDFILE*

: Specifies the file from which words are randomly drawn (default: 1000en).

-quotes *QUOTEFILE*

: Starts quote mode in which quotes are randomly drawn from the given file. The file should be JSON encoded and have the following form:

    [{"text": "foo", "attribution": "bar"}]

## Word Mode

-n *GROUPSZ*

: Sets the number of words which constitute a group.

-g *NGROUPS*

: Sets the number of groups which constitute a test.

## File Mode
-start *PARAGRAPH*

: The offset of the starting paragraph, set this to 0 to reset progress on a given file.

## Aesthetics

-showwpm

: Display WPM whilst typing.

-theme *THEMEFILE*

: The theme to use. 

-notheme

: Attempt to use the default terminal theme. This may produce odd results depending on the theme colours.

-blockcursor

: Use a block cursor.

-bold

: Embolden typed text.

-w *WIDTH*

: The maximum line length in characters. This option is ignored if -raw is present.

## Test Parameters

-t *SECONDS*

: Terminate the test after the given number of seconds.

-noskip

: Disable word skipping when space is pressed.

-nobackspace

: Disable backspace and word deletion.

-nohighlight

: Disable highlighting.

-highlight1

: Only highlight the current word.

-highlight2

: Only highlight the next word.

## Sound

-sound *SOUND*

: Play a WAV or MP3 resource on correct keystrokes, and on incorrect keystrokes unless -error-sound is also set.

-error-sound *SOUND*

: Play a WAV or MP3 resource on incorrect keystrokes.

## Scripting

-oneshot

: Automatically exit after a single run.

-noreport

: Don't show a report at the end of a test.

-csv

: Print CSV formatted results.

	Tests have the form:

	```
	test,[wpm],[cpm],[accuracy],[timestamp].
	```

	Mistakes have the form:

	```
	mistake,[word],[typed]
	```

-json

: Print the test output in JSON.

-raw

: Don't reflow STDIN text or show one paragraph at a time.  Note that line breaks
are determined exclusively by the input.  

-multi 

: Treat each input paragraph as a self contained test.

## Misc

**-list** *TYPE*\

    Lists internal resources of the given type. TYPE=[themes|quotes|words|sounds].

**-v**\

    Print the current version.

# EXAMPLES

Creates a series of tests each consisting of a random quote drawn from the
builtin quote file 'en'.
```
tt -quotes en
```

Creates a series of tests each consisting of 10 random words drawn from
words.txt
```
tt -words words.txt -n 10
```

Starts a sequence of tests in which each test consists of a paragraph from war
and peace starting with paragraph 1.
```
tt -start 1 ~/war_and_peace.txt
```

Produces a test consisting of 40 random words draw from 
the system dictionary (similar to 'tt -n 40').
```
shuf -n 40 /usr/share/dict/words|tt
```

Reads a local JSON quote list from standard input and prints one completed
result in CSV format on exit.
```sh
cat quotes.json | tt -quotes - -oneshot -noreport -csv
```

Starts a new typing test which uses the tt source as input:

```
curl -LsS https://raw.githubusercontent.com/lemnos/tt/master/src/tt.go | head -n 20 | tt -noskip -raw
```

Modify to taste.

# PATHS

  Some options like **-words** and **-theme** accept a path. If the given path does
  not exist, the following directories are searched for a file with the given
  name before falling back to internal resources:

  ~/.tt/words\
  ~/.tt/themes\
  /etc/tt/words\
  /etc/tt/themes

  Live typing controls changed through Settings are saved in
  **$XDG_DATA_HOME/tt/settings.json**, or
  **~/.local/share/tt/settings.json** when **$XDG_DATA_HOME** is unset.
  Explicitly supplied **-showwpm**, **-noskip**, **-nobackspace**,
  **-blockcursor**, **-bold**, **-nohighlight**, **-highlight1**, and
  **-highlight2** flags override matching saved values for the current
  invocation without rewriting them.

  Rich Charm History is retained indefinitely in
  **$XDG_DATA_HOME/tt/history-v1**, or
  **~/.local/share/tt/history-v1** when **$XDG_DATA_HOME** is unset. Each
  authoritative session is a versioned JSON file; **index.json** is a
  rebuildable cache. Embedded prompts may retain bounded word occurrence
  summaries for practice. Private file, stdin, and local word-list input
  contributes no word candidates. Complete prompts, complete typed responses,
  raw input events, and source paths are not retained in rich History.
  Malformed authoritative data is reported with its exact path and is not
  rewritten. If **.write-lock** remains after a crash, verify no tt writer is
  active before removing only that lock directory.

# KEYS

  **esc: ** Restarts the test; on the Results screen retries the same prompt.\
  **C-c: ** Terminates tt\
  **C-p: ** Opens Settings during an active test and pauses its timer. In
  Settings, **up**/**down** select a row, **space** or **enter** changes it, and
  **esc** or **C-p** saves and returns to the same test.\
  **C-backspace: ** Deletes the previous word\
  **r: ** Retries the completed prompt from the Results screen.\
  **s: ** Retries a failed History save from the Results screen.\
  **h: ** Opens History from Results. In History, **up**/**down** select,
  **left**/**right** page, **t** toggles regular/practice, and **esc** returns
  to the same result.\
  **p: ** Reviews weaknesses on regular Charm Results if suitable evidence
  exists; unavailable states show a reason and sample size.\
  **delete: ** Dismisses the selected review item; **r** restores it.\
  **enter: ** Starts a reviewed practice drill. On practice Results, **a**
  starts a fresh drill, **n** returns to a fresh regular test, and **esc**
  restores the originating result.\
  **enter: ** Starts the next test from the Results screen.\
  **right** Move to the next test.\
  **left** Move to the previous test.

# AUTHOR

Aetnaeus (aetnaeus@protonmail.com)

# SEE ALSO

## Project Page

    https://github.com/lemnos/tt

# LICENSE

MIT
