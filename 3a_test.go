package go3a_test

import (
	"strings"
	"testing"

	"github.com/asciimoth/go3a"
)

func parseMust(t *testing.T, s string) *go3a.Art {
	t.Helper()
	art, err := go3a.Parse3A(strings.NewReader(s))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return art
}

func parseErr(t *testing.T, s string) {
	t.Helper()
	_, err := go3a.Parse3A(strings.NewReader(s))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestMinimalHeaderOnly(t *testing.T) {
	src := `@3a
`
	art := parseMust(t, src)

	if art.Header.DelayGlobal != 50 {
		t.Fatalf("expected default delay 50, got %d", art.Header.DelayGlobal)
	}
	if art.Header.Loop != true {
		t.Fatalf("expected default loop=true")
	}
	if art.Header.ColorsEnable == nil || *art.Header.ColorsEnable != false {
		t.Fatalf("expected colors disabled by default")
	}
}

func TestHeaderFields(t *testing.T) {
	src := `@3a
title   A   Cool   Art
author  Alice
author  Bob
author  Alice
orig-author   Someone
src https://example.com
editor nvim
license CC0-1.0
delay 10 2:100 5:200
loop no
preview 3
#ascii #ansi
#art
`
	art := parseMust(t, src)

	if art.Header.Title != "A Cool Art" {
		t.Fatalf("title collapse failed: %q", art.Header.Title)
	}

	if len(art.Header.Authors) != 2 {
		t.Fatalf("expected 2 unique authors, got %v", art.Header.Authors)
	}

	if art.Header.DelayGlobal != 10 {
		t.Fatalf("global delay incorrect")
	}

	if art.Header.DelayFrame[2] != 100 || art.Header.DelayFrame[5] != 200 {
		t.Fatalf("frame delays incorrect: %v", art.Header.DelayFrame)
	}

	if art.Header.Loop != false {
		t.Fatalf("loop parsing failed")
	}

	if art.Header.Preview != 3 {
		t.Fatalf("preview parsing failed")
	}

	if len(art.Header.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %v", art.Header.Tags)
	}
}

func TestColImplicitColorsEnable(t *testing.T) {
	src := `@3a
col + fg:red bg:blue
`
	art := parseMust(t, src)

	if art.Header.ColorsEnable == nil || *art.Header.ColorsEnable != true {
		t.Fatalf("colors should be implicitly enabled when col exists")
	}

	spec, ok := art.Header.Cols['+']
	if !ok {
		t.Fatalf("col '+' not parsed")
	}
	if spec.FG != "red" || spec.BG != "blue" {
		t.Fatalf("col values incorrect: %+v", spec)
	}
}

func TestTextOnlyBody(t *testing.T) {
	src := `@3a

@body
hello
world

foo
bar
`
	art := parseMust(t, src)

	if len(art.TextFrames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(art.TextFrames))
	}

	if art.TextFrames[0][0] != "hello" || art.TextFrames[0][1] != "world" {
		t.Fatalf("frame 0 incorrect: %v", art.TextFrames[0])
	}

	if art.TextFrames[1][0] != "foo" || art.TextFrames[1][1] != "bar" {
		t.Fatalf("frame 1 incorrect: %v", art.TextFrames[1])
	}
}

func TestTextColorPairedBody(t *testing.T) {
	src := `@3a
colors yes

@body
ab
12
cd
34
`
	art := parseMust(t, src)

	if len(art.TextFrames) != 1 {
		t.Fatalf("expected 1 frame")
	}
	if len(art.TextFrames[0]) != 2 {
		t.Fatalf("expected 2 rows")
	}
	if art.TextFrames[0][0] != "ab" || art.ColorFrames[0][0] != "12" {
		t.Fatalf("pair row 0 incorrect")
	}
	if art.TextFrames[0][1] != "cd" || art.ColorFrames[0][1] != "34" {
		t.Fatalf("pair row 1 incorrect")
	}
}

func TestTextColorOddRowsError(t *testing.T) {
	src := `@3a
colors yes

@body
ab
12
cd
`
	parseErr(t, src)
}

func TestTextPin(t *testing.T) {
	src := `@3a

@text-pin
hello
world

@body
11111
22222
`
	art := parseMust(t, src)

	if len(art.TextPin) != 2 {
		t.Fatalf("text pin not parsed")
	}
	if art.TextPin[0] != "hello" {
		t.Fatalf("text pin incorrect")
	}
}

func TestColorPin(t *testing.T) {
	src := `@3a

@color-pin
111
222
`
	art := parseMust(t, src)

	if len(art.ColorPin) != 2 {
		t.Fatalf("color pin not parsed")
	}
}

func TestAttachBlock(t *testing.T) {
	src := `@3a

@attach
{ "key": "value" }
`
	art := parseMust(t, src)

	if !strings.Contains(art.Attach, `"key"`) {
		t.Fatalf("attach block not parsed")
	}
}

func TestSanitizeRemovesCRAndControl(t *testing.T) {
	src := "@3a\r\n" +
		"title A\r\n" +
		"\x00\x01\x02"

	art := parseMust(t, src)

	if art.Header.Title != "A" {
		t.Fatalf("sanitize failed to remove control chars")
	}
}

func TestMissingHeaderError(t *testing.T) {
	src := `@body
hello
`
	parseErr(t, src)
}
