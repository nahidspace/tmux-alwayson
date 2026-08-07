package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Codex implements Agent for Codex CLI (https://github.com/openai/codex).
//
// Codex has changed how it persists sessions across versions: newer
// releases (~0.118+) keep a `threads` table in a versioned SQLite file
// (~/.codex/state_N.sqlite); older releases wrote one rollout JSONL file
// per session under ~/.codex/sessions/YYYY/MM/DD/ with a session_meta
// record naming the id and cwd. ValidSession checks the SQLite path first
// and falls back to scanning rollout files, so this works against either
// generation without needing to know which one is installed.
//
// Unlike Claude, this has not been exercised against a live Codex install
// -- it's implemented from Codex's documented storage format and the
// existing bash hooks' own handling of it, not verified end-to-end the way
// Claude was.
type Codex struct {
	Home string
}

var _ Agent = Codex{}

func (Codex) Name() string { return "codex" }

var codexCmdlinePattern = regexp.MustCompile(`(^|/)codex(\s|$)`)

func (Codex) Matches(cmdline string) bool {
	return codexCmdlinePattern.MatchString(cmdline)
}

func (c Codex) home() string {
	if c.Home != "" {
		return c.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (c Codex) codexDir() string {
	return filepath.Join(c.home(), ".codex")
}

// newestStateDB returns the highest-versioned state_*.sqlite file, if any
// (the DB is versioned and bumps on schema changes, e.g. state_5.sqlite).
func (c Codex) newestStateDB() (string, bool) {
	matches, err := filepath.Glob(filepath.Join(c.codexDir(), "state_*.sqlite"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return matches[0], true
}

func (c Codex) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	if db, ok := c.newestStateDB(); ok {
		query := fmt.Sprintf(
			"SELECT 1 FROM threads WHERE id = %s AND cwd = %s AND (archived IS NULL OR archived = 0) LIMIT 1;",
			sqliteQuoteString(sessionID), sqliteQuoteString(cwd),
		)
		if _, ok := querySQLiteOne(db, query); ok {
			return true
		}
	}
	_, cwdMatches := findRolloutSession(filepath.Join(c.codexDir(), "sessions"), sessionID, cwd)
	return cwdMatches
}

func (c Codex) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	if db, ok := c.newestStateDB(); ok {
		query := fmt.Sprintf(
			"SELECT id FROM threads WHERE cwd = %s AND (archived IS NULL OR archived = 0) ORDER BY updated_at DESC LIMIT 1;",
			sqliteQuoteString(cwd),
		)
		if id, ok := querySQLiteOne(db, query); ok {
			return id, true
		}
	}
	return newestRolloutSession(filepath.Join(c.codexDir(), "sessions"), cwd)
}

func (Codex) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"codex"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "resume", sessionID)
	return cmd
}

type rolloutSessionMeta struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
}

// readRolloutMeta reads just the first line of a rollout JSONL file, where
// the session_meta record (id, cwd) lives.
func readRolloutMeta(path string) (rolloutSessionMeta, bool) {
	f, err := os.Open(path)
	if err != nil {
		return rolloutSessionMeta{}, false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return rolloutSessionMeta{}, false
	}
	var meta rolloutSessionMeta
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil || meta.ID == "" {
		return rolloutSessionMeta{}, false
	}
	return meta, true
}

func findRolloutSession(sessionsRoot, sessionID, cwd string) (found, cwdMatches bool) {
	var result bool
	filepathWalk(sessionsRoot, func(path string) bool {
		meta, ok := readRolloutMeta(path)
		if ok && meta.ID == sessionID {
			result = meta.Cwd == cwd
			return false // stop walking
		}
		return true
	})
	return result, result
}

func newestRolloutSession(sessionsRoot, cwd string) (string, bool) {
	type candidate struct {
		id      string
		modTime int64
	}
	var best candidate
	filepathWalk(sessionsRoot, func(path string) bool {
		meta, ok := readRolloutMeta(path)
		if !ok || meta.Cwd != cwd {
			return true
		}
		info, err := os.Stat(path)
		if err != nil {
			return true
		}
		if info.ModTime().UnixNano() > best.modTime {
			best = candidate{id: meta.ID, modTime: info.ModTime().UnixNano()}
		}
		return true
	})
	return best.id, best.id != ""
}

// filepathWalk visits every regular file under root, calling visit(path)
// for each; visit returns false to stop early. Missing root is not an
// error -- it just means nothing to walk.
func filepathWalk(root string, visit func(path string) bool) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !visit(path) {
			return filepath.SkipAll
		}
		return nil
	})
}
