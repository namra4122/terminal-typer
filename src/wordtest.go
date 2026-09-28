package main

import "regexp"

func generateWordTest(name string, n int, g int) func() []segment {
	b := readResource("words", name)
	if b == nil {
		die("%s does not appear to be a valid word list. See '-list words' for a list of builtin word lists.", name)
	}
	return generateWordTestFromBytes(b, n, g)
}

func generateWordTestFromBytes(b []byte, n int, g int) func() []segment {
	words := regexp.MustCompile("\\s+").Split(string(b), -1)
	return func() []segment {
		segments := make([]segment, g)
		for i := 0; i < g; i++ {
			segments[i] = segment{randomText(n, words), ""}
		}
		return segments
	}
}
