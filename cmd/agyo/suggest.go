package main

import (
	"strings"
)

type commandCandidate struct {
	suggest string
	alias   string
}

var commandCandidates = []commandCandidate{
	// top-level commands
	{"init", "init"},
	{"session", "session"},
	{"checkpoint", "checkpoint"},
	{"rollback", "rollback"},
	{"browser", "browser"},
	{"notebooklm", "notebooklm"},
	{"notebooklm", "notebook"},
	{"notebooklm", "nblm"},
	{"dashboard", "dashboard"},
	{"doctor", "doctor"},
	{"sync", "sync"},
	{"hook", "hook"},
	{"google", "google"},
	{"google", "gsuite"},
	{"google", "workspace"},
	{"study", "study"},
	{"completion", "completion"},
	{"version", "version"},
	{"about", "about"},

	// common aliases & subcommands
	{"study ingest", "ingest"},
	{"study exam-prep", "exam-prep"},
	{"study quiz", "quiz"},
	{"google calendar", "calendar"},
	{"google docs", "docs"},
	{"google auth", "auth"},
	{"session status", "status"},
	{"session status", "session status"},
	{"session compact", "compact"},
	{"session compact", "session compact"},
	{"session archive", "archive"},
	{"session archive", "session archive"},
	{"session list", "list"},
	{"session list", "session list"},
	{"session restore", "restore"},
	{"session restore", "session restore"},
	{"session watch", "watch"},
	{"session watch", "tail"},
	{"session export", "export"},

	{"browser start", "start"},
	{"browser start", "browser start"},
	{"browser status", "browser status"},
	{"browser stop", "stop"},
	{"browser stop", "browser stop"},
	{"browser tabs", "tabs"},
	{"browser tabs", "browser tabs"},
	{"browser open", "open"},
	{"browser open", "browser open"},
	{"browser shot", "shot"},
	{"browser shot", "screenshot"},
	{"browser eval", "eval"},

	{"notebooklm status", "notebooklm status"},
	{"notebooklm open", "notebooklm open"},
	{"notebooklm list", "notebooklm list"},
	{"notebooklm ask", "notebooklm ask"},
	{"notebooklm push", "notebooklm push"},
	{"notebooklm add-source", "notebooklm add-source"},
	{"notebooklm mcp", "mcp"},

	{"hook install", "hook install"},
	{"hook antigravity", "hook antigravity"},
	{"hook run", "hook run"},
}

func levenshtein(a, b string) int {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	la := len(a)
	lb := len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			min := d[i-1][j] + 1
			if d[i][j-1]+1 < min {
				min = d[i][j-1] + 1
			}
			if d[i-1][j-1]+cost < min {
				min = d[i-1][j-1] + cost
			}
			d[i][j] = min
		}
	}
	return d[la][lb]
}

func suggestCommand(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if len(input) == 0 {
		return ""
	}

	bestDist := 999
	bestSuggest := ""

	for _, c := range commandCandidates {
		dist := levenshtein(input, c.alias)
		if dist < bestDist {
			bestDist = dist
			bestSuggest = c.suggest
		}
	}

	maxAllowed := 2
	if len(input) >= 7 {
		maxAllowed = 3
	}
	if bestDist <= maxAllowed {
		return bestSuggest
	}
	return ""
}
