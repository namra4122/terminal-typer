package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell"
)

const (
	TyperComplete = iota
	TyperSigInt
	TyperEscape
	TyperPrevious
	TyperNext
	TyperResize
)

type segment struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution"`
}

type mistake struct {
	Word  string `json:"word"`
	Typed string `json:"typed"`
}

type typer struct {
	Scr              tcell.Screen
	OnStart          func()
	SkipWord         bool
	ShowWpm          bool
	DisableBackspace bool
	BlockCursor      bool
	tty              io.Writer
	now              func() time.Time

	currentWordStyle    tcell.Style
	nextWordStyle       tcell.Style
	incorrectSpaceStyle tcell.Style
	incorrectStyle      tcell.Style
	correctStyle        tcell.Style
	defaultStyle        tcell.Style
	baseCorrectStyle    tcell.Style
	baseCurrentStyle    tcell.Style
	baseNextStyle       tcell.Style
	styles              Styles

	savedSettings runtimeSettings
	settings      runtimeSettings
	overrides     settingsOverrides
	flagValues    flagValues

	soundBuffer      *beep.Buffer
	errorSoundBuffer *beep.Buffer
}

func NewTyper(scr tcell.Screen, emboldenTypedText bool, fgcol, bgcol, hicol, hicol2, hicol3, errcol tcell.Color) *typer {
	var tty io.Writer
	theme := Theme{
		Background:  bgcol,
		Text:        fgcol,
		Muted:       fgcol,
		Subtle:      fgcol,
		Border:      fgcol,
		BorderFocus: hicol2,
		Accent:      hicol2,
		Success:     hicol,
		Warning:     hicol3,
		Error:       errcol,
		Info:        hicol3,
		SelectedBG:  bgcol,
	}
	styles := NewStyles(theme)
	def := styles.Text

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	//Will fail on windows, but tt is still mostly usable via tcell
	if err != nil {
		tty = ioutil.Discard
	}

	correctStyle := styles.Success
	if emboldenTypedText {
		correctStyle = correctStyle.Bold(true)
	}

	return &typer{
		Scr:      scr,
		SkipWord: true,
		tty:      tty,
		now:      time.Now,

		defaultStyle:        def,
		correctStyle:        correctStyle,
		currentWordStyle:    def.Foreground(theme.Accent),
		nextWordStyle:       def.Foreground(theme.Info),
		incorrectStyle:      styles.Error,
		incorrectSpaceStyle: def.Background(theme.Error),
		baseCorrectStyle:    styles.Success,
		baseCurrentStyle:    def.Foreground(theme.Accent),
		baseNextStyle:       def.Foreground(theme.Info),
		styles:              styles,
	}
}

func (t *typer) applyRuntimeSettings() {
	t.settings = effectiveRuntimeSettings(t.savedSettings, t.overrides, t.flagValues)
	t.SkipWord = t.settings.SkipWord
	t.ShowWpm = t.settings.ShowWPM
	t.DisableBackspace = !t.settings.AllowBackspace
	t.BlockCursor = t.settings.BlockCursor
	t.correctStyle = t.baseCorrectStyle.Bold(t.settings.BoldTypedText)

	switch t.settings.Highlight {
	case highlightCurrentOnly:
		t.currentWordStyle = t.baseNextStyle
		t.nextWordStyle = t.defaultStyle
	case highlightNextOnly:
		t.currentWordStyle = t.defaultStyle
		t.nextWordStyle = t.baseNextStyle
	case highlightOff:
		t.currentWordStyle = t.defaultStyle
		t.nextWordStyle = t.defaultStyle
	default:
		t.currentWordStyle = t.baseCurrentStyle
		t.nextWordStyle = t.baseNextStyle
	}

	if t.BlockCursor {
		t.tty.Write([]byte("\033[2 q"))
	} else {
		t.tty.Write([]byte("\033[5 q"))
	}
}

func (t *typer) playKeySound(correct bool) {
	if correct && t.soundBuffer != nil {
		speaker.Play(t.soundBuffer.Streamer(0, t.soundBuffer.Len()))
	} else if !correct {
		if t.errorSoundBuffer != nil {
			speaker.Play(t.errorSoundBuffer.Streamer(0, t.errorSoundBuffer.Len()))
		} else if t.soundBuffer != nil {
			speaker.Play(t.soundBuffer.Streamer(0, t.soundBuffer.Len()))
		}
	}
}

func (t *typer) Start(text []segment, timeout time.Duration) (nerrs, ncorrect int, duration time.Duration, rc int, mistakes []mistake) {
	timeLeft := timeout

	for i, s := range text {
		startImmediately := true
		var d time.Duration
		var e, c int
		var m []mistake

		if i == 0 {
			startImmediately = false
		}

		e, c, rc, d, m = t.start(s.Text, timeLeft, startImmediately, s.Attribution)

		nerrs += e
		ncorrect += c
		duration += d
		mistakes = append(mistakes, m...)

		if timeout != -1 {
			timeLeft -= d
			if timeLeft <= 0 {
				return
			}
		}

		if rc != TyperComplete {
			return
		}
	}

	return
}

func extractMistypedWords(text []rune, typed []rune) (mistakes []mistake) {
	var w []rune
	var t []rune
	f := false

	for i := range text {
		if text[i] == ' ' {
			if f {
				mistakes = append(mistakes, mistake{string(w), string(t)})
			}

			w = w[:0]
			t = t[:0]
			f = false
			continue
		}

		if text[i] != typed[i] {
			f = true
		}

		if text[i] == 0 {
			w = append(w, '_')
		} else {
			w = append(w, text[i])
		}

		if typed[i] == 0 {
			t = append(t, '_')
		} else {
			t = append(t, typed[i])
		}
	}

	if f {
		mistakes = append(mistakes, mistake{string(w), string(t)})
	}

	return
}
func typerTextDimensions(s string) (width, height int) {
	width = CellWidth(s)
	if s == "" {
		return width, 0
	}

	height = 1
	for _, r := range s {
		if r == '\n' {
			height++
		}
	}
	return width, height
}

func (t *typer) start(s string, timeLimit time.Duration, startImmediately bool, attribution string) (nerrs int, ncorrect int, rc int, duration time.Duration, mistakes []mistake) {
	var startTime time.Time
	text := []rune(s)
	typed := make([]rune, len(text))

	sw, sh := t.Scr.Size()
	nc, nr := typerTextDimensions(s)
	x := (sw - nc) / 2
	y := (sh - nr) / 2

	defer t.tty.Write([]byte("\033[2 q"))

	t.Scr.SetStyle(t.defaultStyle)
	idx := 0

	calcStats := func() {
		nerrs = 0
		ncorrect = 0

		mistakes = extractMistypedWords(text[:idx], typed[:idx])

		for i := 0; i < idx; i++ {
			if text[i] != '\n' {
				if text[i] != typed[i] {
					nerrs++
				} else {
					ncorrect++
				}
			}
		}

		rc = TyperComplete
		duration = t.now().Sub(startTime)
	}

	redraw := func() {
		cx := x
		cy := y
		inword := -1

		for i := range text {
			style := t.defaultStyle

			if text[i] == '\n' {
				cy++
				cx = x
				if inword != -1 {
					inword++
				}
				continue
			}

			if i == idx && cx >= 0 && cx < sw && cy >= 0 && cy < sh {
				t.Scr.ShowCursor(cx, cy)
				inword = 0
			}

			if i >= idx {
				if text[i] == ' ' {
					inword++
				} else if inword == 0 {
					style = t.currentWordStyle
				} else if inword == 1 {
					style = t.nextWordStyle
				} else {
					style = t.defaultStyle
				}
			} else if text[i] != typed[i] {
				if text[i] == ' ' {
					style = t.incorrectSpaceStyle
				} else {
					style = t.incorrectStyle
				}
			} else {
				style = t.correctStyle
			}

			cx, cy = DrawText(t.Scr, cx, cy, string(text[i]), style)
		}

		aw, ah := typerTextDimensions(attribution)
		if attribution != "" {
			attributionX := x + nc - aw
			attributionY := y + nr + 1
			DrawTextInRect(t.Scr, Rect{X: attributionX, Y: attributionY, Width: sw, Height: sh - attributionY},
				TruncateCells(attribution, sw), t.styles.Muted)
		}

		if timeLimit != -1 && !startTime.IsZero() {
			remaining := timeLimit - t.now().Sub(startTime)
			timerX := x + nc/2
			timerY := y + nr + ah + 1
			DrawText(t.Scr, timerX, timerY, "      ", t.defaultStyle)
			DrawText(t.Scr, timerX, timerY, strconv.Itoa(int(remaining/1e9)+1), t.styles.Info)
		}

		if t.ShowWpm && !startTime.IsZero() {
			calcStats()
			if duration > 1e7 { //Avoid flashing large numbers on test start.
				wpm := int((float64(ncorrect) / 5) / (float64(duration) / 60e9))
				DrawText(t.Scr, x+nc/2-4, y-2, fmt.Sprintf("WPM: %-10d", wpm), t.styles.Info)
			}
		}

		//Potentially inefficient, but seems to be good enough

		t.Scr.Show()
	}

	deleteWord := func() {
		if idx == 0 {
			return
		}

		idx--

		for idx > 0 && (text[idx] == ' ' || text[idx] == '\n') {
			idx--
		}

		for idx > 0 && text[idx] != ' ' && text[idx] != '\n' {
			idx--
		}

		if text[idx] == ' ' || text[idx] == '\n' {
			typed[idx] = text[idx]
			idx++
		}
	}

	tickerCloser := make(chan bool)

	//Inject nil events into the main event loop at regular invervals to force an update
	ticker := func() {
		for {
			select {
			case <-tickerCloser:
				return
			default:
			}

			time.Sleep(time.Duration(5e8))
			t.Scr.PostEventWait(nil)
		}
	}

	go ticker()
	defer close(tickerCloser)

	if startImmediately {
		startTime = t.now()
	}

	t.Scr.Clear()
	for {
		redraw()

		ev := t.Scr.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventResize:
			rc = TyperResize
			return
		case *tcell.EventKey:
			if runtime.GOOS != "windows" && ev.Key() == tcell.KeyBackspace { //Control+backspace on unix terms
				if !t.DisableBackspace {
					deleteWord()
				}
				continue
			}

			if ev.Key() == tcell.KeyCtrlP {
				opened := t.now()
				committed, interrupted := showSettings(t.Scr, &t.savedSettings, t.overrides, t.flagValues)
				if !startTime.IsZero() {
					startTime = startTime.Add(t.now().Sub(opened))
				}
				if interrupted {
					rc = TyperSigInt
					return
				}
				if committed {
					t.applyRuntimeSettings()
				}
				t.Scr.SetStyle(t.defaultStyle)
				t.Scr.Clear()
				continue
			}

			if startTime.IsZero() {
				startTime = t.now()
			}

			switch key := ev.Key(); key {
			case tcell.KeyCtrlC:
				rc = TyperSigInt

				return
			case tcell.KeyEscape:
				rc = TyperEscape

				return
			case tcell.KeyCtrlL:
				t.Scr.Sync()

			case tcell.KeyRight:
				rc = TyperNext
				return

			case tcell.KeyLeft:
				rc = TyperPrevious
				return

			case tcell.KeyCtrlW:
				if !t.DisableBackspace {
					deleteWord()
				}

			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if !t.DisableBackspace {
					if ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModCtrl {
						deleteWord()
					} else {
						if idx == 0 {
							break
						}

						idx--

						for idx > 0 && text[idx] == '\n' {
							idx--
						}
					}
				}
			case tcell.KeyRune:
				if idx < len(text) {
					if t.SkipWord && ev.Rune() == ' ' {
						if idx > 0 && text[idx-1] == ' ' && text[idx] != ' ' { //Do nothing on word boundaries.
							break
						}

						for idx < len(text) && text[idx] != ' ' && text[idx] != '\n' {
							typed[idx] = 0
							idx++
						}

						if idx < len(text) {
							typed[idx] = text[idx]
							idx++
							t.playKeySound(true)
						}
					} else {
						correct := ev.Rune() == text[idx]
						typed[idx] = ev.Rune()
						idx++
						t.playKeySound(correct)
					}

					for idx < len(text) && text[idx] == '\n' {
						typed[idx] = text[idx]
						idx++
					}
				}

				if idx == len(text) {
					calcStats()
					return
				}
			}
		default: //tick
			if timeLimit != -1 && !startTime.IsZero() && timeLimit <= t.now().Sub(startTime) {
				calcStats()
				return
			}

			redraw()
		}
	}
}
