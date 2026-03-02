package go3a

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/asciimoth/go3a/internal"
)

type ColorPair struct {
	FG string // empty => default
	BG string // empty => default
}

type Header struct {
	Title        string
	Authors      []string
	OrigAuthors  []string
	Src          string
	Editor       string
	License      string
	DelayGlobal  uint
	DelayFrame   map[int]uint
	Loop         bool // default true
	Preview      int  // default 0
	ColorsEnable *bool
	Cols         map[rune]ColorPair
	Tags         []string
}

func (h *Header) parseDelayVal(val string) error {
	// tokens separated by spaces. Exactly one token must be global (unsigned decimal integer without ':')
	if val == "" {
		return nil
	}
	toks := strings.Fields(val)
	foundGlobal := false
	for _, t := range toks {
		if strings.Contains(t, ":") {
			parts := strings.SplitN(t, ":", 2)
			if len(parts) != 2 {
				continue
			}
			frame, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			delay, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			if frame < 0 || delay < 0 {
				continue
			}
			h.DelayFrame[frame] = uint(delay)
		} else {
			// global
			if foundGlobal {
				// duplicate globals: last one wins
			}
			g, err := strconv.Atoi(t)
			if err != nil {
				return fmt.Errorf("invalid global delay %q", t)
			}
			if g < 0 {
				return fmt.Errorf("negative global delay %d", g)
			}
			h.DelayGlobal = uint(g)
			foundGlobal = true
		}
	}
	// if not provided, default is already set to 50
	return nil
}

var colTokenRe = regexp.MustCompile(`(?i)^(.)\s*(.*)$`) // capture single-char name then rest

func (h *Header) parseColMapping(val string) error {
	trim := strings.TrimSpace(val)
	if trim == "" {
		return fmt.Errorf("empty col mapping")
	}
	m := colTokenRe.FindStringSubmatch(trim)
	if m == nil {
		return fmt.Errorf("invalid col map")
	}
	name := m[1]
	if len([]rune(name)) != 1 {
		return fmt.Errorf("col name must be single character")
	}
	nr := []rune(name)[0]
	rest := strings.Fields(m[2])
	var fg, bg string
	for _, tok := range rest {
		if strings.HasPrefix(strings.ToLower(tok), "fg:") {
			fg = strings.TrimSpace(tok[3:])
		} else if strings.HasPrefix(strings.ToLower(tok), "bg:") {
			bg = strings.TrimSpace(tok[3:])
		} else {
			// maybe token is color code itself (legacy?) - ignore for now
		}
	}
	// name may be predefined; user-defined mapping may override
	h.Cols[nr] = ColorPair{FG: fg, BG: bg}
	return nil
}

func (h *Header) parseHeaderBlock(lines []string) error {
	// header supports comments (lines starting with ';;'), keys, and tag lines (start with '#')
	// parse line by line
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, ";;") {
			continue
		}
		if strings.HasPrefix(trim, "#") {
			// tag line: every word that begins with '#' is a tag
			words := strings.Fields(trim)
			for _, w := range words {
				if strings.HasPrefix(w, "#") && len(w) > 1 {
					h.Tags = append(h.Tags, strings.TrimSpace(w))
				}
			}
			continue
		}
		// otherwise key/value pair: first word is key, remainder is value
		parts := strings.Fields(trim)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		val := ""
		if len(trim) > len(key) {
			val = strings.TrimSpace(trim[len(key):])
		}
		// collapse multiple spaces in title/author-like keys per spec
		switch key {
		case "title":
			h.Title = internal.CollapseSpaces(strings.TrimSpace(val))
		case "author":
			h.Authors = append(h.Authors, internal.CollapseSpaces(strings.TrimSpace(val)))
		case "orig-author":
			h.OrigAuthors = append(h.OrigAuthors, internal.CollapseSpaces(strings.TrimSpace(val)))
		case "src":
			h.Src = strings.TrimSpace(val)
		case "editor":
			h.Editor = strings.TrimSpace(val)
		case "license":
			h.License = strings.TrimSpace(strings.ToLower(val))
			if h.License == "" {
				h.License = "proprietary"
			}
		case "delay":
			if err := h.parseDelayVal(val); err != nil {
				return fmt.Errorf("invalid delay value %q: %w", val, err)
			}
		case "loop":
			l, err := internal.ParseYesNo(val)
			if err != nil {
				return fmt.Errorf("invalid loop value %q: %w", val, err)
			}
			h.Loop = l
		case "preview":
			if val == "" {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return fmt.Errorf("invalid preview value %q: %w", val, err)
			}
			h.Preview = n
		case "col":
			if err := h.parseColMapping(val); err != nil {
				return fmt.Errorf("invalid col mapping %q: %w", val, err)
			}
		case "colors":
			b, err := internal.ParseYesNo(val)
			if err != nil {
				return fmt.Errorf("invalid colors value %q: %w", val, err)
			}
			h.ColorsEnable = &b
		}
	}
	return nil
}
