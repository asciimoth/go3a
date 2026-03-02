package go3a

import (
	"errors"
	"fmt"
	"io"
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
	// Body consists of frames separated by one or more blank lines. A frame consists of lines.
	// But when colors enabled and channels not pinned, each "logical row" is represented by two consecutive rows:
	//  - first is text row
	//  - second is corresponding colors row
	// That applies line-by-line in the frame.
	//
	// We'll split frames first (preserving blank lines inside a frame is not needed).
	allFrames := internal.SplitFrames(lines)
	colorsEnabled := false
	if art.Header.ColorsEnable != nil {
		colorsEnabled = *art.Header.ColorsEnable
	}

	// If both text and colors pinned or text pinned and colors disabled, body can be omitted - but if present, we'll still parse.
	for fi, frame := range allFrames {
		if len(frame) == 0 {
			// empty frame -> produce empty
			art.TextFrames = append(art.TextFrames, []string{})
			art.ColorFrames = append(art.ColorFrames, []string{})
			continue
		}
		// We'll inspect rows: if colors enabled and neither channel is pinned, expect pairs
		if colorsEnabled && len(art.TextPin) == 0 && len(art.ColorPin) == 0 {
			// expect even number of rows (pairs)
			if len(frame)%2 != 0 {
				// tolerate: treat last unmatched as text-only row with empty colors row
				// but return error? Spec suggests equal lengths; we'll be lenient but warn via error
				// For now, we return an error to be strict:
				return fmt.Errorf("frame %d: expected pairs of rows for text+color, got odd row count %d", fi, len(frame))
			}
			var textRows []string
			var colorRows []string
			for i := 0; i < len(frame); i += 2 {
				trow := frame[i]
				crow := frame[i+1]
				// Trim trailing spaces on color row? Colors row length must equal text row length - we keep raw but validate lengths.
				if len([]rune(trow)) != len([]rune(crow)) {
					// if mismatch, try to pad shorter with default mapping '_' to match rune length
					// but to be conservative, return error
					return fmt.Errorf("frame %d: row %d text/col length mismatch (text %d runes, colors %d runes)", fi, i/2, len([]rune(trow)), len([]rune(crow)))
				}
				textRows = append(textRows, trow)
				colorRows = append(colorRows, crow)
			}
			art.TextFrames = append(art.TextFrames, textRows)
			art.ColorFrames = append(art.ColorFrames, colorRows)
		} else if colorsEnabled && len(art.TextPin) > 0 {
			// text pinned: each line is colors row
			art.ColorFrames = append(art.ColorFrames, append([]string{}, frame...))
			// text frames are derived from text pin (applies to all frames), we'll leave TextFrames empty
		} else {
			// colors disabled or colors pinned: each line is text row
			art.TextFrames = append(art.TextFrames, append([]string{}, frame...))
			if !colorsEnabled && len(art.ColorPin) == 0 {
				// no color frames
			} else if len(art.ColorPin) > 0 {
				// colors pinned elsewhere
			}
		}
	}
	return nil
}
