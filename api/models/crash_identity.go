package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

var crashDiagnostics = regexp.MustCompile(`(?im)^\s*(computer|system|hardware|operating system|os|os version|processor|cpu|gpu( model| memory)?|tracked gpu memory|graphics card|video card|memory|ram|directx|display adapter|render surface|window|scene|level|cursemark|scalegrid guard|physics contact guard|machine|device)\s*(information|info|details)?\s*[:=]`)
var crashFilePos = regexp.MustCompile(`^\[?FilePos\(Method\(([^,]*),([^)]*)\),([^,]+),(\d+),([^)]*)\)\]?$`)
var crashFilePosSeparator = regexp.MustCompile(`\),\s*(FilePos\()`)
var crashSourceLine = regexp.MustCompile(`([A-Za-z0-9_./\\-]+\.(?:hx|cpp|cc|c|h|cs|go|js|ts)):\d+(?::\d+)?`)
var crashHex = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
var crashSymbolAddress = regexp.MustCompile(`(?i)^\s*(?:0x[0-9a-f]+\s+)?(.+![A-Za-z_][^+]*?)(\+0x[0-9a-f]+)?\s*$`)

// CrashIdentityV1 is deliberately conservative: the complete ordered stack and
// error are required. Failure source locations/relative instruction offsets
// remain significant; only caller locations, runtime addresses, diagnostics and
// formatting are normalized. No fuzzy/prefix matching or
// transitive clustering; unrelated callers and unsymbolized addresses survive.
// The canonical JSON is exposed to admins so grouping is inspectable.
func CrashIdentityV1(c Crash) (key, canonical string) {
	stack := strings.ReplaceAll(c.Stack, "\r\n", "\n")
	if at := crashDiagnostics.FindStringIndex(stack); at != nil {
		stack = stack[:at[0]]
	}
	stack = crashFilePosSeparator.ReplaceAllString(stack, ")\n$1")
	frames := []string{}
	locationSeen := false
	for _, line := range strings.Split(stack, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if match := crashFilePos.FindStringSubmatch(line); match != nil {
			// Preserve the failure line/column: different failures in one
			// function must not merge just because they share the same callers.
			for i := range match {
				match[i] = strings.TrimSpace(match[i])
			}
			line = "FilePos(Method(" + match[1] + "," + match[2] + ")," + match[3]
			if !locationSeen {
				line += "," + match[4] + "," + match[5]
			}
			line += ")"
			locationSeen = true
		} else if match := crashSymbolAddress.FindStringSubmatch(line); match != nil {
			line = strings.TrimSpace(match[1])
			if !locationSeen {
				line += strings.ToLower(match[2])
			}
			locationSeen = true
		} else if crashSourceLine.MatchString(line) {
			if locationSeen {
				line = crashSourceLine.ReplaceAllString(line, "$1")
			}
			locationSeen = true
		}
		if line != "" {
			frames = append(frames, line)
		}
	}
	message := strings.Join(strings.Fields(c.Message), " ")
	if strings.Contains(strings.ToLower(message), "access violation") && locationSeen {
		message = crashHex.ReplaceAllString(message, "<address>")
	}
	data, _ := json.Marshal([]interface{}{NormalizeProjectName(c.Project), c.Category, c.Platform, message, frames})
	canonical = string(data)
	sum := sha256.Sum256(data)
	return "v1:" + hex.EncodeToString(sum[:]), canonical
}
