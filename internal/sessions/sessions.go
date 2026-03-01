package sessions

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nickheyer/greetdeez/internal/config"
)

type Session struct {
	Name    string   `json:"name"`
	Cmd     []string `json:"cmd"`
	Type    string   `json:"type"`
	Desktop string   `json:"desktop"`
}

// List discovers available desktop sessions by parsing .desktop entry files
// from the configured session directories.
func List(dirs []config.SessionDir) []Session {
	var out []Session

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir.Path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".desktop") {
				continue
			}

			s, err := parseDesktopEntry(filepath.Join(dir.Path, entry.Name()), dir.Type)
			if err != nil {
				continue
			}
			out = append(out, *s)
		}
	}

	// Deduplicate: if two sessions share the same name+type, keep only the first.
	seen := make(map[[2]string]bool)
	deduped := out[:0]
	for _, s := range out {
		key := [2]string{s.Name, s.Type}
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, s)
		}
	}
	out = deduped

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "wayland"
		}
		return out[i].Name < out[j].Name
	})

	return out
}

func parseDesktopEntry(path, sessType string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var name, execStr, desktopNames, tryExec string
	var hidden, noDisplay bool
	inDesktopEntry := false
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Only parse keys within the [Desktop Entry] section.
		if strings.HasPrefix(line, "[") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch key {
		case "Name":
			name = val
		case "Exec":
			execStr = val
		case "DesktopNames":
			desktopNames = val
		case "TryExec":
			tryExec = val
		case "Hidden":
			hidden = val == "true"
		case "NoDisplay":
			noDisplay = val == "true"
		}
	}

	if name == "" || execStr == "" {
		return nil, os.ErrNotExist
	}

	if hidden || noDisplay {
		return nil, os.ErrNotExist
	}

	if tryExec != "" {
		if _, err := exec.LookPath(tryExec); err != nil {
			return nil, err
		}
	}

	return &Session{
		Name:    cleanSessionName(name),
		Cmd:     parseExecString(execStr),
		Type:    sessType,
		Desktop: desktopNames,
	}, nil
}

// This is absolute SLOP
// parenHintRe matches parenthesized compositor/protocol hints like
// "(Wayland)", "(X11)", "(Xorg)", "(X.Org)", case-insensitive.
var parenHintRe = regexp.MustCompile(`(?i)\s*\((wayland|x11|xorg|x\.org)\)\s*$`)

// Strips redundant session-type hints from the display name.
func cleanSessionName(name string) string {
	name = strings.TrimSuffix(strings.TrimSuffix(name, " on Wayland"), " on Xorg")
	name = parenHintRe.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

// parseExecString splits an Exec= value into arguments, respecting
// double-quoted strings and backslash escapes (simple shlex-style).
func parseExecString(s string) []string {
	var args []string
	var cur []byte
	inQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			cur = append(cur, c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if c == ' ' && !inQuote {
			if len(cur) > 0 {
				args = append(args, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		args = append(args, string(cur))
	}
	return args
}
