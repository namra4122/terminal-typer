package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	"github.com/gdamore/tcell"
	"github.com/mattn/go-isatty"
)

var scr tcell.Screen
var csvMode bool
var jsonMode bool

type result struct {
	Wpm       int       `json:"wpm"`
	Cpm       int       `json:"cpm"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp int64     `json:"timestamp"`
	Mistakes  []mistake `json:"mistakes"`
}

func die(format string, args ...interface{}) {
	if scr != nil {
		scr.Fini()
	}
	fmt.Fprintf(os.Stderr, "ERROR: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

var results []result

func parseConfig(b []byte) map[string]string {
	if b == nil {
		return nil
	}

	cfg := map[string]string{}
	for _, ln := range bytes.Split(b, []byte("\n")) {
		a := strings.SplitN(string(ln), ":", 2)
		if len(a) == 2 {
			cfg[a[0]] = strings.Trim(a[1], " ")
		}
	}

	return cfg
}

func exit(rc int) {
	speaker.Close()
	scr.Fini()

	if jsonMode {
		//Avoid null in serialized JSON.
		for i := range results {
			if results[i].Mistakes == nil {
				results[i].Mistakes = []mistake{}
			}
		}

		b, err := json.Marshal(results)
		if err != nil {
			panic(err)
		}
		os.Stdout.Write(b)
	}

	if csvMode {
		for _, r := range results {
			fmt.Printf("test,%d,%d,%.2f,%d\n", r.Wpm, r.Cpm, r.Accuracy, r.Timestamp)
			for _, m := range r.Mistakes {
				fmt.Printf("mistake,%s,%s\n", m.Word, m.Typed)
			}
		}
	}

	os.Exit(rc)
}

func showReport(scr tcell.Screen, cpm, wpm int, accuracy float64, attribution string, mistakes []mistake, activeStyles ...Styles) {
	type reportRow struct {
		label string
		value string
		style tcell.Style
	}

	styles := DefaultStyles
	if len(activeStyles) > 0 {
		styles = activeStyles[0]
	}
	rows := []reportRow{
		{label: "WPM", value: fmt.Sprintf("%d", wpm), style: styles.Value},
		{label: "CPM", value: fmt.Sprintf("%d", cpm), style: styles.Value},
		{label: "Accuracy", value: fmt.Sprintf("%.2f%%", accuracy), style: styles.Success},
	}

	if len(mistakes) > 0 {
		words := make([]string, len(mistakes))
		for i, m := range mistakes {
			words[i] = m.Word
		}
		rows = append(rows, reportRow{
			label: "Mistakes",
			value: strings.Join(words, ", "),
			style: styles.Error,
		})
	}
	if attribution != "" {
		rows = append(rows, reportRow{
			label: "Attribution",
			value: attribution,
			style: styles.Muted,
		})
	}

	render := func() bool {
		scr.SetStyle(styles.Text)
		scr.Clear()
		sw, sh := scr.Size()
		if sw <= 0 || sh <= 0 {
			scr.HideCursor()
			scr.Show()
			return false
		}

		title := "Typing complete"
		labelWidth := CellWidth("Accuracy")
		contentWidth := CellWidth(title)
		for _, row := range rows {
			if width := CellWidth(row.value); width > contentWidth {
				contentWidth = width
			}
		}
		contentWidth += labelWidth + 3

		panelWidth := contentWidth + 2
		if panelWidth > sw {
			panelWidth = sw
		}
		panelHeight := len(rows) + 4
		if panelHeight > sh {
			panelHeight = sh
		}
		panelX := (sw - panelWidth) / 2
		panelY := (sh - panelHeight) / 2

		if panelWidth >= 2 && panelHeight >= 2 {
			DrawBox(scr, Rect{X: panelX, Y: panelY, Width: panelWidth, Height: panelHeight},
				RoundedBorder(), styles.Border)
		}

		contentX := panelX + 1
		contentWidth = panelWidth - 2
		if panelWidth < 2 {
			contentX = panelX
			contentWidth = panelWidth
		}
		if contentWidth > 0 {
			DrawTextInRect(scr, Rect{X: contentX, Y: panelY + 1, Width: contentWidth, Height: 1},
				TruncateCells(title, contentWidth), styles.AppTitle)
		}

		if contentWidth > 0 {
			rowLabelWidth := labelWidth
			if rowLabelWidth > contentWidth-1 {
				rowLabelWidth = contentWidth - 1
			}
			if rowLabelWidth < 0 {
				rowLabelWidth = 0
			}
			valueWidth := contentWidth - rowLabelWidth - 1
			rowsBottom := panelY + panelHeight - 2
			for i, row := range rows {
				rowY := panelY + 2 + i
				if rowY >= rowsBottom || valueWidth <= 0 {
					break
				}

				DrawTextInRect(scr, Rect{X: contentX, Y: rowY, Width: rowLabelWidth, Height: 1},
					TruncateCells(row.label, rowLabelWidth), styles.Muted)
				value := TruncateCells(row.value, valueWidth)
				valueX := contentX + contentWidth - CellWidth(value)
				DrawTextInRect(scr, Rect{X: valueX, Y: rowY, Width: valueWidth, Height: 1},
					value, row.style)
			}
		}

		if panelHeight >= 4 && contentWidth > 0 {
			footerY := panelY + panelHeight - 2
			DrawTextInRect(scr, Rect{X: contentX, Y: footerY, Width: contentWidth, Height: 1},
				TruncateCells("Esc close", contentWidth), styles.FooterText)
		}

		scr.HideCursor()
		scr.Show()
		return true
	}

	if !render() {
		return
	}
	for {
		switch event := scr.PollEvent().(type) {
		case *tcell.EventResize:
			render()
		case *tcell.EventKey:
			if event.Key() == tcell.KeyEscape {
				return
			}
			if event.Key() == tcell.KeyCtrlC {
				exit(1)
			}
		}
	}
}

func createDefaultTyper(scr tcell.Screen) *typer {
	return NewTyper(scr, true, tcell.ColorDefault,
		tcell.ColorDefault,
		tcell.ColorWhite,
		tcell.ColorGreen,
		tcell.ColorGreen,
		tcell.ColorMaroon)
}

func createTyper(scr tcell.Screen, bold bool, themeName string) *typer {
	var theme map[string]string

	if b := readResource("themes", themeName); b == nil {
		die("%s does not appear to be a valid theme, try '-list themes' for a list of built in thems.", themeName)
	} else {
		theme = parseConfig(b)
	}

	var bgcol, fgcol, hicol, hicol2, hicol3, errcol tcell.Color
	var err error

	if bgcol, err = newTcellColor(theme["bgcol"]); err != nil {
		die("bgcol is not defined and/or a valid hex colour.")
	}
	if fgcol, err = newTcellColor(theme["fgcol"]); err != nil {
		die("fgcol is not defined and/or a valid hex colour.")
	}
	if hicol, err = newTcellColor(theme["hicol"]); err != nil {
		die("hicol is not defined and/or a valid hex colour.")
	}
	if hicol2, err = newTcellColor(theme["hicol2"]); err != nil {
		die("hicol2 is not defined and/or a valid hex colour.")
	}
	if hicol3, err = newTcellColor(theme["hicol3"]); err != nil {
		die("hicol3 is not defined and/or a valid hex colour.")
	}
	if errcol, err = newTcellColor(theme["errcol"]); err != nil {
		die("errcol is not defined and/or a valid hex colour.")
	}

	return NewTyper(scr, bold, fgcol, bgcol, hicol, hicol2, hicol3, errcol)
}

var usage = `usage: tt [options] [file]

Modes
    -words  WORDFILE    Specifies the file from which words are randomly
                        drawn (default: 1000en).
    -quotes QUOTEFILE   Starts quote mode in which quotes are randomly drawn
                        from the given file. The file should be JSON encoded and
                        have the following form:

                        [{"text": "foo", attribution: "bar"}]

Word Mode
    -n GROUPSZ          Sets the number of words which constitute a group.
    -g NGROUPS          Sets the number of groups which constitute a test.

File Mode
    -start PARAGRAPH    The offset of the starting paragraph, set this to 0 to
                        reset progress on a given file.
Aesthetics
    -showwpm            Display WPM whilst typing.
    -theme THEMEFILE    The theme to use. 
    -w                  The maximum line length in characters. This option is 
    -notheme            Attempt to use the default terminal theme. 
                        This may produce odd results depending 
                        on the theme colours.
    -blockcursor        Use the default cursor style.
    -bold               Embolden typed text.
                        ignored if -raw is present.
Test Parameters
    -t SECONDS          Terminate the test after the given number of seconds.
    -noskip             Disable word skipping when space is pressed.
    -nobackspace        Disable the backspace key.
    -nohighlight        Disable current and next word highlighting.
    -highlight1         Only highlight the current word.
    -highlight2         Only highlight the next word.

Sound
    -sound SOUND        Play SOUND on each keystroke (WAV or MP3).
                        Built-in and ~/.tt/sounds/ sounds available.
    -error-sound SOUND  Play SOUND on incorrect keystrokes. If both
                        -sound and -error-sound are given, correct
                        keys play -sound and incorrect play -error-sound.

Scripting
    -oneshot            Automatically exit after a single run.
    -noreport           Don't show a report at the end of a test.
    -csv                Print the test results to stdout in the form:
                        [type],[wpm],[cpm],[accuracy],[timestamp].
    -json               Print the test output in JSON.
    -raw                Don't reflow STDIN text or show one paragraph at a time.
                        Note that line breaks are determined exclusively by the
                        input.
    -multi              Treat each input paragraph as a self contained test.

Misc
    -list TYPE          Lists internal resources of the given type.
                        TYPE=[themes|quotes|words|sounds]

Version
    -v                  Print the current version.
`

func saveMistakes(mistakes []mistake) {
	var db []mistake

	if err := readValue(MISTAKE_DB, &db); err != nil {
		db = nil
	}

	db = append(db, mistakes...)
	writeValue(MISTAKE_DB, db)
}

type nopReadCloser struct {
	*bytes.Reader
}

func (nopReadCloser) Close() error { return nil }

func loadSound(name string, targetRate beep.SampleRate) (*beep.Buffer, error) {
	data := readResource("sounds", name)

	if data == nil {
		data = readResource("sounds", name+".wav")
	}
	if data == nil {
		data = readResource("sounds", name+".mp3")
	}
	if data == nil {
		return nil, fmt.Errorf("sound %q not found (searched ~/.tt/sounds/, /etc/tt/sounds/, and built-in sounds)", name)
	}

	ext := strings.ToLower(filepath.Ext(name))
	var streamer beep.StreamSeekCloser
	var format beep.Format
	var err error

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(nopReadCloser{bytes.NewReader(data)})
	case ".wav":
		streamer, format, err = wav.Decode(bytes.NewReader(data))
	default:
		streamer, format, err = wav.Decode(bytes.NewReader(data))
		if err != nil {
			streamer, format, err = mp3.Decode(nopReadCloser{bytes.NewReader(data)})
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	defer streamer.Close()

	var s beep.Streamer = streamer
	if format.SampleRate != targetRate {
		s = beep.Resample(3, format.SampleRate, targetRate, streamer)
	}

	buf := beep.NewBuffer(beep.Format{
		SampleRate:  targetRate,
		NumChannels: format.NumChannels,
		Precision:   format.Precision,
	})
	buf.Append(s)
	return buf, nil
}

func main() {
	var n int
	var g int

	var rawMode bool
	var oneShotMode bool
	var noHighlightCurrent bool
	var noHighlightNext bool
	var noHighlight bool
	var maxLineLen int
	var noSkip bool
	var noBackspace bool
	var noReport bool
	var noTheme bool
	var normalCursor bool
	var timeout int
	var startParagraph int

	var listFlag string
	var wordFile string
	var quoteFile string

	var themeName string
	var showWpm bool
	var multiMode bool
	var versionFlag bool
	var boldFlag bool
	var soundFile string
	var errorSoundFile string

	var err error

	flag.IntVar(&n, "n", 50, "")
	flag.IntVar(&g, "g", 1, "")
	flag.IntVar(&startParagraph, "start", -1, "")

	flag.IntVar(&maxLineLen, "w", 80, "")
	flag.IntVar(&timeout, "t", -1, "")

	flag.BoolVar(&versionFlag, "v", false, "")

	flag.StringVar(&wordFile, "words", "", "")
	flag.StringVar(&quoteFile, "quotes", "", "")

	flag.BoolVar(&showWpm, "showwpm", false, "")
	flag.BoolVar(&noSkip, "noskip", false, "")
	flag.BoolVar(&normalCursor, "blockcursor", false, "")
	flag.BoolVar(&noBackspace, "nobackspace", false, "")
	flag.BoolVar(&noTheme, "notheme", false, "")
	flag.BoolVar(&oneShotMode, "oneshot", false, "")
	flag.BoolVar(&noHighlight, "nohighlight", false, "")
	flag.BoolVar(&noHighlightCurrent, "highlight2", false, "")
	flag.BoolVar(&noHighlightNext, "highlight1", false, "")
	flag.BoolVar(&noReport, "noreport", false, "")
	flag.BoolVar(&boldFlag, "bold", false, "")
	flag.BoolVar(&csvMode, "csv", false, "")
	flag.BoolVar(&jsonMode, "json", false, "")
	flag.BoolVar(&rawMode, "raw", false, "")
	flag.BoolVar(&multiMode, "multi", false, "")
	flag.StringVar(&themeName, "theme", "default", "")
	flag.StringVar(&listFlag, "list", "", "")
	flag.StringVar(&soundFile, "sound", "", "")
	flag.StringVar(&errorSoundFile, "error-sound", "", "")

	flag.Usage = func() { os.Stdout.Write([]byte(usage)) }
	flag.Parse()

	if listFlag != "" {
		prefix := listFlag + "/"
		for path, _ := range packedFiles {
			if strings.Index(path, prefix) == 0 {
				_, f := filepath.Split(path)
				fmt.Println(f)
			}
		}

		os.Exit(0)
	}

	if versionFlag {
		fmt.Fprintf(os.Stderr, "tt version 0.4.2\n")
		os.Exit(1)
	}

	visitedFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		visitedFlags[f.Name] = true
	})
	overrides := collectSettingsOverrides(visitedFlags)

	highlight := highlightCurrentAndNext
	if noHighlight {
		highlight = highlightOff
	}
	if noHighlightNext {
		highlight = highlightCurrentOnly
	}
	if noHighlightCurrent {
		highlight = highlightNextOnly
	}
	liveFlags := flagValues{
		ShowWPM:        showWpm,
		SkipWord:       !noSkip,
		AllowBackspace: !noBackspace,
		BlockCursor:    normalCursor,
		BoldTypedText:  boldFlag,
		Highlight:      highlight,
	}

	savedSettings := loadRuntimeSettings(RUNTIME_SETTINGS_DB, os.Stderr)

	if noTheme {
		os.Setenv("TCELL_TRUECOLOR", "disable")
	}

	reflow := func(s string) string {
		sw, _ := scr.Size()

		wsz := maxLineLen
		if wsz > sw {
			wsz = sw - 8
		}

		s = regexp.MustCompile("\\s+").ReplaceAllString(s, " ")
		return strings.Replace(
			wordWrap(strings.Trim(s, " "), wsz),
			"\n", " \n", -1)
	}

	inputFile := ""
	if len(flag.Args()) > 0 {
		inputFile = flag.Args()[0]
	}
	config := resolveTestConfig(testOptions{
		Words:           wordFile,
		Quotes:          quoteFile,
		File:            inputFile,
		StdinIsTerminal: isatty.IsTerminal(os.Stdin.Fd()),
		WordsPerGroup:   n,
		Groups:          g,
		TimeoutSeconds:  timeout,
		Raw:             rawMode,
		Multi:           multiMode,
		StartParagraph:  startParagraph,
	})
	var stdinData []byte
	if config.Source == stdinSource {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		stdinData = b
	}
	testFn := newTestGenerator(config, stdinData)

	scr, err = tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	if err := scr.Init(); err != nil {
		panic(err)
	}

	defer func() {
		if r := recover(); r != nil {
			scr.Fini()
			panic(r)
		}
	}()

	var typer *typer
	if noTheme {
		typer = createDefaultTyper(scr)
	} else {
		typer = createTyper(scr, boldFlag, themeName)
	}

	typer.savedSettings = savedSettings
	typer.overrides = overrides
	typer.flagValues = liveFlags
	typer.applyRuntimeSettings()

	if soundFile != "" || errorSoundFile != "" {
		const targetRate = beep.SampleRate(44100)
		var normalBuf, errorBuf *beep.Buffer

		if soundFile != "" {
			buf, err := loadSound(soundFile, targetRate)
			if err != nil {
				die("loading sound file: %s", err)
			}
			normalBuf = buf
		}
		if errorSoundFile != "" {
			buf, err := loadSound(errorSoundFile, targetRate)
			if err != nil {
				die("loading error-sound file: %s", err)
			}
			errorBuf = buf
		}

		if err := speaker.Init(targetRate, targetRate.N(time.Second/30)); err != nil {
			die("initializing speaker: %s", err)
		}
		typer.soundBuffer = normalBuf
		typer.errorSoundBuffer = errorBuf
	}

	var tests []*Test
	var idx = 0

	for {
		if idx >= len(tests) {
			tests = append(tests, testFn())
		}

		if tests[idx] == nil {
			exit(0)
		}

		nerrs, ncorrect, t, rc, mistakes := typer.Start(displaySegments(tests[idx], reflow), config.TimeLimit)
		saveMistakes(mistakes)

		switch rc {
		case TyperNext:
			idx++
		case TyperPrevious:
			if idx > 0 {
				idx--
			}
		case TyperComplete:
			cpm := int(float64(ncorrect) / (float64(t) / 60e9))
			wpm := cpm / 5
			accuracy := float64(ncorrect) / float64(nerrs+ncorrect) * 100

			results = append(results, result{wpm, cpm, accuracy, time.Now().Unix(), mistakes})
			if !noReport {
				showReport(scr, cpm, wpm, accuracy, tests[idx].Attribution, mistakes, typer.styles)
			}
			if oneShotMode {
				exit(0)
			}

			idx++
		case TyperSigInt:
			exit(1)

		case TyperResize:
			//Resize events restart the test, this shouldn't be a problem in the vast majority of cases
			//and allows us to avoid baking rewrapping logic into the typer.

			//TODO: implement state-preserving resize (maybe)
		}
	}
}
