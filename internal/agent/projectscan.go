package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// genericSessionMeta covers the metadata record shape Gemini CLI and Qwen
// Code both write as the first line of a session's JSONL file. Field names
// are a union of what each has been documented (Gemini CLI source) or
// reported (Qwen Code docs/issues) to use; a record missing some of these
// still parses, just with those fields blank.
type genericSessionMeta struct {
	SessionID   string `json:"sessionId"`
	Cwd         string `json:"cwd"`
	LastUpdated string `json:"lastUpdated"`
	Mtime       string `json:"mtime"`
}

func readGenericSessionMeta(path string) (genericSessionMeta, bool) {
	f, err := os.Open(path)
	if err != nil {
		return genericSessionMeta{}, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return genericSessionMeta{}, false
	}
	var m genericSessionMeta
	if err := json.Unmarshal(scanner.Bytes(), &m); err != nil || m.SessionID == "" {
		return genericSessionMeta{}, false
	}
	return m, true
}

// scanProjectsForSession walks every immediate subdirectory of projectsRoot
// (each one is some project-hash- or project-name-keyed directory; the
// exact naming scheme is deliberately not depended on here) looking under
// <subdir>/<chatsDir>/*.jsonl for a session metadata record whose
// sessionId matches id. A blank cwd in the record is not treated as a
// mismatch -- same leniency as the Pi/OMP JSONL header matching.
func scanProjectsForSession(projectsRoot, chatsDir, id, cwd string) bool {
	found := false
	forEachSessionFile(projectsRoot, chatsDir, func(m genericSessionMeta) bool {
		if m.SessionID == id && (m.Cwd == "" || m.Cwd == cwd) {
			found = true
			return false
		}
		return true
	})
	return found
}

// newestSessionAcrossProjects finds the most recently updated session
// (by LastUpdated, falling back to Mtime, falling back to file mtime)
// across every project directory whose recorded cwd is blank or matches.
func newestSessionAcrossProjects(projectsRoot, chatsDir, cwd string) (string, bool) {
	var bestID, bestStamp string
	forEachSessionFile(projectsRoot, chatsDir, func(m genericSessionMeta) bool {
		if m.Cwd != "" && m.Cwd != cwd {
			return true
		}
		stamp := m.LastUpdated
		if stamp == "" {
			stamp = m.Mtime
		}
		if stamp > bestStamp {
			bestStamp = stamp
			bestID = m.SessionID
		}
		return true
	})
	return bestID, bestID != ""
}

// forEachSessionFile calls visit(meta) for every parseable session file
// under projectsRoot/*/chatsDir/*.jsonl. visit returns false to stop early.
func forEachSessionFile(projectsRoot, chatsDir string, visit func(genericSessionMeta) bool) {
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		return
	}
	for _, projectEntry := range entries {
		if !projectEntry.IsDir() {
			continue
		}
		chats := filepath.Join(projectsRoot, projectEntry.Name(), chatsDir)
		files, err := os.ReadDir(chats)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			meta, ok := readGenericSessionMeta(filepath.Join(chats, f.Name()))
			if !ok {
				continue
			}
			if !visit(meta) {
				return
			}
		}
	}
}
