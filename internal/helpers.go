package internal

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

func DedupKeepOrder(xs []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

// CollapseSpaces collapses consecutive ASCII spaces into a single space and trims ends.
func CollapseSpaces(s string) string {
	// replace any sequence of spaces (U+0020) with single space
	// trailing/leading trimmed
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Sanitize removes/ignores the code points that MUST be ignored and replaces tabs/space separators with ASCII space.
// See spec for list of ranges. We implement according to ranges in the spec.
func Sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		// ignore CR
		if r == '\r' {
			continue
		}
		// newline allowed
		if r == '\n' {
			b.WriteRune('\n')
			continue
		}
		// ignore C0 control characters U+0000..U+001F except newline
		if r >= 0x0000 && r <= 0x001F {
			// we've handled newline and CR already; skip others
			continue
		}
		// DEL and C1 control characters U+007F..U+009F
		if r >= 0x007F && r <= 0x009F {
			continue
		}
		// Zero-width / joiner characters: U+200B–U+200F, U+FEFF, U+FE00–U+FE0F
		if (r >= 0x200B && r <= 0x200F) || r == 0xFEFF || (r >= 0xFE00 && r <= 0xFE0F) {
			continue
		}
		// Combining marks: U+0300..U+036F
		if r >= 0x0300 && r <= 0x036F {
			continue
		}
		// Bidirectional control codes: U+202A..U+202E, U+2066..U+2069
		if (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
			continue
		}
		// Surrogate code points: U+D800..U+DFFF
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		// Replace tab and space separators and U+180E with ASCII space
		if r == '\t' || r == 0x180E || unicode.Is(unicode.Zs, r) {
			b.WriteRune(' ')
			continue
		}
		// otherwise keep rune as-is
		b.WriteRune(r)
	}
	return b.String()
}

func TrimTrailingEmptyLines(xs []string) []string {
	end := len(xs)
	for end > 0 && strings.TrimSpace(xs[end-1]) == "" {
		end--
	}
	return append([]string{}, xs[:end]...)
}

func TrimLeadingTrailingEmptyLines(xs []string) []string {
	start := 0
	for start < len(xs) && strings.TrimSpace(xs[start]) == "" {
		start++
	}
	end := len(xs)
	for end > start && strings.TrimSpace(xs[end-1]) == "" {
		end--
	}
	return append([]string{}, xs[start:end]...)
}

// SplitFrames splits block lines into frames separated by 1 or more blank lines.
// Returns slice of frames; each frame is slice of non-empty lines (but internal empty lines are trimmed).
func SplitFrames(lines []string) [][]string {
	var frames [][]string
	var cur []string
	appendCur := func() {
		// drop leading/trailing empty lines in frame
		cur = TrimLeadingTrailingEmptyLines(cur)
		if len(cur) > 0 {
			frames = append(frames, cur)
		} else {
			// even empty frames possible? we can append empty frame
			frames = append(frames, []string{})
		}
		cur = nil
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			if len(cur) > 0 {
				appendCur()
			} else {
				// multiple blank lines: treat as separator => produce empty frames only if we had created something?
				// We'll continue.
			}
		} else {
			cur = append(cur, ln)
		}
	}
	if cur != nil {
		appendCur()
	}
	// If the input had no blank separators and just rows, that's a single frame
	return frames
}

func ParseYesNo(val string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "yes" || v == "true" {
		return true, nil
	}
	if v == "no" || v == "false" {
		return false, nil
	}
	return false, fmt.Errorf("expected yes/no, got %q", val)
}

type block struct {
	Name  string
	Lines []string
}

// SplitBlocks splits sanitized text into blocks. A block starts with a line beginning with '@' optionally preceded by spaces.
// Block title content is the rest of that line after '@' trimmed. Subsequent lines up to the next line starting with '@' or EOF belong to the block.
// Empty lines are preserved (we trim or keep as appropriate when parsing specific blocks).
func SplitBlocks(s string) ([]block, error) {
	var out []block
	scanner := bufio.NewScanner(strings.NewReader(s))
	var cur *block
	for scanner.Scan() {
		line := scanner.Text()
		trimLeft := strings.TrimLeft(line, " ")
		if after, ok := strings.CutPrefix(trimLeft, "@"); ok {
			// start a new block
			name := strings.TrimSpace(after)
			// name must contain only ASCII alnum and + - _ .
			// We'll accept common variants but normalize.
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, errors.New("empty block title after @")
			}
			b := block{Name: name}
			out = append(out, b)
			curIndex := len(out) - 1
			cur = &out[curIndex]
		} else {
			// line belongs to current block (if any)
			if cur == nil {
				// content before first block: ignore empty lines, otherwise error
				if strings.TrimSpace(line) == "" {
					continue
				}
				return nil, fmt.Errorf("content found before first block: %q", line)
			}
			// append line as-is
			cur.Lines = append(cur.Lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
