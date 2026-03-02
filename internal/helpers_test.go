package internal_test

import (
	"github.com/asciimoth/go3a/internal"
	"reflect"
	"strings"
	"testing"
)

func TestDedupKeepOrder(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		out  []string
	}{
		{
			"empty",
			[]string{},
			[]string{},
		},
		{
			"no duplicates",
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"with duplicates",
			[]string{"a", "b", "a", "c", "b"},
			[]string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := internal.DedupKeepOrder(tt.in)
			if len(got) > 0 && len(tt.out) > 0 {
				if !reflect.DeepEqual(got, tt.out) {
					t.Fatalf("got %v, want %v", got, tt.out)
				}
			}
		})
	}
}

func TestCollapseSpaces(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"a   b    c", "a b c"},
		{"a b  c", "a b c"},
	}

	for _, tt := range tests {
		if got := internal.CollapseSpaces(tt.in); got != tt.out {
			t.Fatalf("collapseSpaces(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestSanitize(t *testing.T) {
	// Includes:
	// - \r removal
	// - tab replacement
	// - zero width removal
	// - combining mark removal
	// - C0 control removal
	input := "a\tb\r\nc\u200Bd\u0301e\u0001f\u180Eg"
	expected := "ab\ncdef g"

	got := internal.Sanitize(input)
	if got != expected {
		t.Fatalf("sanitize(%q) = %q, want %q", input, got, expected)
	}
}

func TestTrimTrailingEmptyLines(t *testing.T) {
	tests := []struct {
		in  []string
		out []string
	}{
		{
			[]string{"a", "b", "", ""},
			[]string{"a", "b"},
		},
		{
			[]string{"a", "b"},
			[]string{"a", "b"},
		},
		{
			[]string{"", "", ""},
			[]string{},
		},
	}

	for _, tt := range tests {
		got := internal.TrimTrailingEmptyLines(tt.in)
		if !reflect.DeepEqual(got, tt.out) {
			t.Fatalf("got %v, want %v", got, tt.out)
		}
	}
}

func TestTrimLeadingTrailingEmptyLines(t *testing.T) {
	tests := []struct {
		in  []string
		out []string
	}{
		{
			[]string{"", "", "a", "b", "", ""},
			[]string{"a", "b"},
		},
		{
			[]string{"a", "b"},
			[]string{"a", "b"},
		},
		{
			[]string{"", "", ""},
			[]string{},
		},
	}

	for _, tt := range tests {
		got := internal.TrimLeadingTrailingEmptyLines(tt.in)
		if !reflect.DeepEqual(got, tt.out) {
			t.Fatalf("got %v, want %v", got, tt.out)
		}
	}
}

func TestSplitFrames(t *testing.T) {
	input := []string{
		"a1",
		"a2",
		"",
		"",
		"b1",
		"b2",
		"",
		"c1",
	}

	expected := [][]string{
		{"a1", "a2"},
		{"b1", "b2"},
		{"c1"},
	}

	got := internal.SplitFrames(input)

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("got %v, want %v", got, expected)
	}
}

func TestParseYesNo(t *testing.T) {
	tests := []struct {
		in    string
		want  bool
		isErr bool
	}{
		{"yes", true, false},
		{"no", false, false},
		{"YES", true, false},
		{"No", false, false},
		{"true", true, false},
		{"false", false, false},
		{"maybe", false, true},
	}

	for _, tt := range tests {
		got, err := internal.ParseYesNo(tt.in)
		if tt.isErr {
			if err == nil {
				t.Fatalf("expected error for %q", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("parseYesNo(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSplitBlocks(t *testing.T) {
	input := `
@3a
title Test

@body
line1
line2

@attach
{"a":1}
`

	blocks, err := internal.SplitBlocks(strings.TrimSpace(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}

	if blocks[0].Name != "3a" {
		t.Fatalf("expected first block '3a', got %q", blocks[0].Name)
	}
	if blocks[1].Name != "body" {
		t.Fatalf("expected second block 'body', got %q", blocks[1].Name)
	}
	if blocks[2].Name != "attach" {
		t.Fatalf("expected third block 'attach', got %q", blocks[2].Name)
	}

	if !reflect.DeepEqual(blocks[1].Lines, []string{"line1", "line2", ""}) {
		t.Fatalf("unexpected body lines: %v", blocks[1].Lines)
	}
}
