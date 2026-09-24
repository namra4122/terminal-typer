% tt(1)

# NAME

tt - A terminal based typing test

# SYNOPSIS

usage: tt \[OPTION\]... \[FILE\]

# DESCRIPTION

  This manual describes the current source checkout. The executable still
  reports version 0.4.2, but the source checkout includes changes beyond the
  historical 0.4.2 release notes. The repository's five-phase roadmap
  describes proposals, not installed features.

  By default tt creates a test consisting of 50 randomly generated words from
  the top 1000 words in the English language. If provided with a path, tt will
  use the given file as input treating each paragraph as a separate segment of
  the test. The program will automatically keep track of your position in the
  file so subsequent invocations on the same path will place you at the most
  recent paragraph (-start 0 can be used to reset your position).  
  
  Arbitrary text can also be piped directly into the program to create a custom
  test. Each paragraph of the input is treated as a segment unless '-multi' is
  supplied in which case each paragraph is treated as a separate test. 

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

# KEYS

  **esc: ** Restarts the test\
  **C-c: ** Terminates tt\
  **C-p: ** Opens Settings during an active test and pauses its timer. In
  Settings, **up**/**down** select a row, **space** or **enter** changes it, and
  **esc** or **C-p** saves and returns to the same test.\
  **C-backspace: ** Deletes the previous word\
  **right** Move to the next test.\
  **left** Move to the previous test.

# AUTHOR

Aetnaeus (aetnaeus@protonmail.com)

# SEE ALSO

## Project Page

    https://github.com/lemnos/tt

# LICENSE

MIT
