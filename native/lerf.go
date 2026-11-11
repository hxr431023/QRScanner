package native

// native/lerf.go — 本文件为本项目自己写的代码，不是微软的代码。
// This file is written for this project; no Microsoft code is used here.
//
// LERF is the language file format used by this library. A file looks like:
//
//	LERF.utf8
//	<app name and version>
//	(blank line)
//	0
//	# comment
//	<12>Save as:<12>
//
// The id in angle brackets appears twice: once before the text and once after.
// `key=value` lines are also accepted so the same parser can read simple maps.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Table maps an entry id to its text.
type Table map[string]string

// ParseFile reads a LERF document. ok is false when the file cannot be read or
// holds no usable entries.
func ParseFile(path string) (Table, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Table{}, false
	}
	table := Parse(string(data))
	return table, len(table) > 0
}

// Parse reads a LERF document from memory.
func Parse(text string) Table {
	table := Table{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "<") {
			close := strings.Index(line, ">")
			if close < 0 {
				continue
			}
			id := line[1:close]
			body := line[close+1:]
			if end := strings.LastIndex(body, "<"+id+">"); end >= 0 {
				body = body[:end]
			}
			table[id] = body
			continue
		}
		if eq := strings.Index(line, "="); eq > 0 {
			table[line[:eq]] = line[eq+1:]
		}
	}
	return table
}

// Get returns the text for id, or the id itself when the entry is missing.
// Falling back to the id keeps a missing translation visible instead of blank.
func (t Table) Get(id string) string {
	if v, ok := t[id]; ok {
		return v
	}
	return id
}

// Getf is Get with {0}, {1}, ... placeholders substituted in order.
func (t Table) Getf(id string, args ...string) string {
	s := t.Get(id)
	for i, a := range args {
		s = strings.ReplaceAll(s, "{"+strconv.Itoa(i)+"}", a)
	}
	return s
}

// LoadDir loads <lang>.lerf from dir and falls back to English when the
// requested language is missing. It returns the table and whether the
// requested file itself was found.
func LoadDir(dir, lang, fallback string) (Table, bool) {
	primary := filepath.Join(dir, "resources", "cmd", lang+".lerf")
	if t, ok := ParseFile(primary); ok {
		return t, true
	}
	if fallback != "" && fallback != lang {
		alt := filepath.Join(dir, "resources", "cmd", fallback+".lerf")
		if t, ok := ParseFile(alt); ok {
			return t, false
		}
	}
	return Table{}, false
}
