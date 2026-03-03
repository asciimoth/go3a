// Package go3a implements a parser for the animated ascii art format.
//
// Example:
//
//	f, _ := os.Open("example.3a")
//	art, err := go3a.Parse3A(f)
//	if err != nil {
//	    log.Fatal(err)
//	}
package go3a

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/asciimoth/go3a/internal"
)

type Art struct {
	Header      Header
	TextFrames  [][]string // frames -> rows (strings)
	ColorFrames [][]string // frames -> rows of single-char mapping names (as strings)
	// If pinned:
	TextPin  []string // rows (single frame) if text pinned
	ColorPin []string // rows (single frame) if colors pinned
	Attach   string
	// unknown/extension blocks (blockName -> lines)
	Extensions map[string][]string
}

// Parse3A reads a 3a document from r and returns parsed Art.
func Parse3A(r io.Reader) (*Art, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// sanitize (remove ignored codepoints, replace tabs/space-separators)
	sanitized := internal.Sanitize(string(raw))

	blocks, err := internal.SplitBlocks(sanitized)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, errors.New("empty input")
	}

	// first block must be header @3a
	firstName := strings.ToLower(blocks[0].Name)
	if firstName != "3a" {
		return nil, fmt.Errorf("first block must be @3a (header); got @%s", blocks[0].Name)
	}

	art := &Art{
		Header: Header{
			DelayFrame:  make(map[int]uint),
			Cols:        make(map[rune]ColorPair),
			Loop:        true, // default
			Preview:     0,
			DelayGlobal: 50, // default
		},
		Extensions: make(map[string][]string),
	}

	if err := art.Header.parseHeaderBlock(blocks[0].Lines); err != nil {
		return nil, err
	}

	// find other blocks
	for i := 1; i < len(blocks); i++ {
		name := strings.ToLower(blocks[i].Name)
		switch name {
		case "body":
			if err := art.parseBodyBlock(blocks[i].Lines); err != nil {
				return nil, err
			}
		case "color-pin":
			// parse as colors pin
			lines := internal.TrimTrailingEmptyLines(blocks[i].Lines)
			art.ColorPin = append([]string{}, lines...)
		case "text-pin": // be permissive
			lines := internal.TrimTrailingEmptyLines(blocks[i].Lines)
			art.TextPin = append([]string{}, lines...)
		case "attach":
			// single-line attach (but allow multiple lines, join with newline)
			art.Attach = strings.Join(internal.TrimLeadingTrailingEmptyLines(blocks[i].Lines), "\n")
		default:
			// extension or unknown block, store raw lines
			art.Extensions[name] = append([]string{}, blocks[i].Lines...)
		}
	}

	// Deduplicate authors and tags (header-level)
	art.Header.Authors = internal.DedupKeepOrder(art.Header.Authors)
	art.Header.OrigAuthors = internal.DedupKeepOrder(art.Header.OrigAuthors)
	art.Header.Tags = internal.DedupKeepOrder(art.Header.Tags)

	// If colors key wasn't present but at least one col exists, mark enabled
	if art.Header.ColorsEnable == nil {
		hasCol := len(art.Header.Cols) > 0
		art.Header.ColorsEnable = &hasCol
	}

	// If body was not present but both pinned blocks exist, it's okay per spec (body can be omitted)
	// If body absent and neither pin exists, that's still valid but art is effectively empty.

	return art, nil
}

func (art *Art) parseBodyBlock(lines []string) error {
	allFrames := internal.SplitFrames(lines)

	colorsEnabled := false
	if art.Header.ColorsEnable != nil {
		colorsEnabled = *art.Header.ColorsEnable
	}

	for fi, frame := range allFrames {
		if len(frame) == 0 {
			art.TextFrames = append(art.TextFrames, []string{})
			art.ColorFrames = append(art.ColorFrames, []string{})
			continue
		}

		var textRows []string
		var colorRows []string

		for ri, row := range frame {
			runes := []rune(row)

			if colorsEnabled && len(art.TextPin) == 0 && len(art.ColorPin) == 0 {
				if len(runes)%2 != 0 {
					return fmt.Errorf(
						"frame %d row %d: odd length (%d), cannot split evenly into text/color halves",
						fi, ri, len(runes),
					)
				}

				mid := len(runes) / 2
				textPart := string(runes[:mid])
				colorPart := string(runes[mid:])

				textRows = append(textRows, textPart)
				colorRows = append(colorRows, colorPart)
			} else if colorsEnabled && len(art.TextPin) > 0 {
				// text pinned → entire row is colors
				colorRows = append(colorRows, row)
			} else {
				// colors disabled or colors pinned → entire row is text
				textRows = append(textRows, row)
			}
		}

		if len(textRows) > 0 {
			art.TextFrames = append(art.TextFrames, textRows)
		}
		if len(colorRows) > 0 {
			art.ColorFrames = append(art.ColorFrames, colorRows)
		}
	}

	return nil
}

// String serializes Art into the 3a textual format.
// The output is deterministic (sorted mappings/pairs) and suitable for round-trip tests.
func (a *Art) String() string {
	var b strings.Builder

	writeLine := func(s string) {
		b.WriteString(s)
		b.WriteByte('\n')
	}

	// Header block (always present)
	writeLine("@3a")

	// Title
	if a.Header.Title != "" {
		writeLine("title " + a.Header.Title)
	}

	// Orig authors
	for _, oa := range a.Header.OrigAuthors {
		writeLine("orig-author " + oa)
	}

	// Authors
	for _, au := range a.Header.Authors {
		writeLine("author " + au)
	}

	// Editor, src
	if a.Header.Editor != "" {
		writeLine("editor " + a.Header.Editor)
	}
	if a.Header.Src != "" {
		writeLine("src " + a.Header.Src)
	}

	// License (emit if set)
	if a.Header.License != "" {
		writeLine("license " + a.Header.License)
	}

	// Preview (only if non-default)
	if a.Header.Preview != 0 {
		writeLine("preview " + strconv.Itoa(a.Header.Preview))
	}

	// Delay: emit global and frame-specific sorted
	// Only emit if global != default 50 or there are frame-specific delays
	if a.Header.DelayGlobal != 50 || len(a.Header.DelayFrame) > 0 {
		parts := []string{strconv.FormatUint(uint64(a.Header.DelayGlobal), 10)}
		// sort frame keys
		keys := make([]int, 0, len(a.Header.DelayFrame))
		for k := range a.Header.DelayFrame {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%d:%d", k, a.Header.DelayFrame[k]))
		}
		writeLine("delay " + strings.Join(parts, " "))
	}

	// Loop: only emit when explicitly "no". Default is yes (omit).
	if !a.Header.Loop {
		writeLine("loop no")
	}

	// Tags: print a single line with all tags (each starts with '#')
	if len(a.Header.Tags) > 0 {
		writeLine(strings.Join(a.Header.Tags, " "))
	}

	// Colors: print if explicit, otherwise if there are col mappings print yes
	if a.Header.ColorsEnable != nil {
		if *a.Header.ColorsEnable {
			writeLine("colors yes")
		} else {
			writeLine("colors no")
		}
	} else if len(a.Header.Cols) > 0 {
		writeLine("colors yes")
	}

	// col mappings: deterministic order by rune
	if len(a.Header.Cols) > 0 {
		keys := make([]rune, 0, len(a.Header.Cols))
		for k := range a.Header.Cols {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, kr := range keys {
			cs := a.Header.Cols[kr]
			// write: col <char> [bg:...] [fg:...]
			parts := []string{string(kr)}
			// prefer bg then fg to match examples (order doesn't matter to parser)
			if cs.BG != "" {
				parts = append(parts, "bg:"+cs.BG)
			}
			if cs.FG != "" {
				parts = append(parts, "fg:"+cs.FG)
			}
			writeLine("col " + strings.Join(parts, " "))
		}
	}

	// blank line after header
	b.WriteByte('\n')

	// Optional text-pin
	if len(a.TextPin) > 0 {
		writeLine("@text-pin")
		for _, ln := range a.TextPin {
			writeLine(ln)
		}
		b.WriteByte('\n')
	}

	// Optional colors pin
	if len(a.ColorPin) > 0 {
		writeLine("@color-pin")
		for _, ln := range a.ColorPin {
			writeLine(ln)
		}
		b.WriteByte('\n')
	}

	// Body: only emit if there is any meaningful body data (frames) OR
	// if both pins are absent but no frames -> emit empty body block (spec requires last block to be body)
	emitBody := false
	if len(a.TextFrames) > 0 || len(a.ColorFrames) > 0 {
		emitBody = true
	}
	// If both pins are present and both channels pinned, body may be omitted; we follow presence of frames.
	if emitBody {
		writeLine("@body")

		colorsEnabled := false
		if a.Header.ColorsEnable != nil {
			colorsEnabled = *a.Header.ColorsEnable
		} else if len(a.Header.Cols) > 0 {
			colorsEnabled = true
		}

		// Determine number of frames to serialize: prefer len(TextFrames) if >0, else len(ColorFrames)
		nFrames := len(a.TextFrames)
		if nFrames == 0 {
			nFrames = len(a.ColorFrames)
		}
		for fi := 0; fi < nFrames; fi++ {
			// if both channels are unpinned and colors enabled: text & colors frames must be paired
			if colorsEnabled && len(a.TextPin) == 0 && len(a.ColorPin) == 0 {
				tf := a.TextFrames[fi]
				cf := a.ColorFrames[fi]
				for r := range tf {
					writeLine(tf[r] + cf[r])
				}
			} else if colorsEnabled && len(a.TextPin) > 0 {
				// text pinned: frame lines are color rows only
				cf := a.ColorFrames[fi]
				for _, ln := range cf {
					writeLine(ln)
				}
			} else {
				// colors disabled or colors pinned: write text rows only if present
				if fi < len(a.TextFrames) {
					for _, ln := range a.TextFrames[fi] {
						writeLine(ln)
					}
				}
			}
			// blank line between frames
			b.WriteByte('\n')
		}
	}

	// Attach block
	if strings.TrimSpace(a.Attach) != "" {
		writeLine("@attach")
		// attach may be multi-line
		for ln := range strings.SplitSeq(a.Attach, "\n") {
			writeLine(ln)
		}
		b.WriteByte('\n')
	}

	// Ensure trailing newline on end (builder already adds them)
	return strings.TrimRight(b.String(), "\n") + "\n"
}
