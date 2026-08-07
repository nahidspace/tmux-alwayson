package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// jsonlSessionHeader is the logical header record Pi and Oh My Pi write as
// one of the first two lines of each session's JSONL file (the very first
// line may instead be a {"type":"title",...} record to skip past).
type jsonlSessionHeader struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
}

// readJSONLHeader reads at most the first two lines of path looking for the
// {"type":"session",...} header record.
func readJSONLHeader(path string) (jsonlSessionHeader, bool) {
	f, err := os.Open(path)
	if err != nil {
		return jsonlSessionHeader{}, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 2 && scanner.Scan(); i++ {
		var h jsonlSessionHeader
		if err := json.Unmarshal(scanner.Bytes(), &h); err != nil {
			continue
		}
		if h.Type == "title" {
			continue // first line was a title record; the header is next
		}
		if h.Type == "session" && h.ID != "" {
			return h, true
		}
		return jsonlSessionHeader{}, false
	}
	return jsonlSessionHeader{}, false
}

// jsonlSessionExists reports whether any *.jsonl file directly under dirs
// has a session header matching id, with a cwd that's either blank or
// equal to cwd (blank header cwd isn't treated as a mismatch -- mirrors
// the existing bash selector's behavior).
func jsonlSessionExists(dirs []string, id, cwd string) bool {
	found := false
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			h, ok := readJSONLHeader(filepath.Join(dir, e.Name()))
			if ok && h.ID == id && (h.Cwd == "" || h.Cwd == cwd) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	return found
}

// newestJSONLSession returns the most recently modified session (by file
// mtime) across dirs whose header cwd is blank or equal to cwd.
func newestJSONLSession(dirs []string, cwd string) (string, bool) {
	var bestID string
	var bestMod int64 = -1
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			h, ok := readJSONLHeader(path)
			if !ok || (h.Cwd != "" && h.Cwd != cwd) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if mt := info.ModTime().UnixNano(); mt > bestMod {
				bestMod = mt
				bestID = h.ID
			}
		}
	}
	return bestID, bestID != ""
}

// sanitizePathComponent mirrors the bash hooks' path sanitization used by
// both Pi and Oh My Pi: strip a leading slash/backslash, then replace every
// remaining /, \, or : with a dash.
func sanitizePathComponent(s string) string {
	s = strings.TrimLeft(s, `/\`)
	s = strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(s)
	return s
}
