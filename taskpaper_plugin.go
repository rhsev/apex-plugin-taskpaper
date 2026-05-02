package main

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	reFenceOpen  = regexp.MustCompile("^([ \t]*)(`{3,})")
	reFenceClose = regexp.MustCompile("^([ \t]*)(`{3,})\\s*$")
	reProject    = regexp.MustCompile(`^(\t*)[^\s#>].*:\s*(@\w+(\([^)]*\))?\s*)*$`)
	reTabTask    = regexp.MustCompile(`^(\t+)- (.*)`)
	reH12        = regexp.MustCompile(`^(##?) (.+)$`)
	reH34        = regexp.MustCompile(`^(####?) (.+)$`)
	reFMLine     = regexp.MustCompile(`^([^:\n]+):\s*(.*)`)
	reMMDLine    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 _-]*:\s+\S`)
)

const doubleNbsp = "  "

func nbspIndent(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(doubleNbsp, n)
}

// splitLines splits text into lines preserving line endings.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	var lines []string
	for text != "" {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			lines = append(lines, text)
			break
		}
		lines = append(lines, text[:i+1])
		text = text[i+1:]
	}
	return lines
}

func fmLineDisplay(line string) string {
	chomped := strings.TrimRight(line, "\n")
	if m := reFMLine.FindStringSubmatch(chomped); m != nil {
		return "**" + m[1] + ":**{.fmkey} " + m[2] + "  \n"
	}
	return line
}

func processFrontmatter(text string) string {
	if strings.HasPrefix(text, "---\n") {
		endPos := -1
		if i := strings.Index(text[4:], "\n---\n"); i >= 0 {
			endPos = 4 + i
		} else if i := strings.Index(text[4:], "\n...\n"); i >= 0 {
			endPos = 4 + i
		}
		if endPos >= 0 {
			var sb strings.Builder
			for _, line := range splitLines(text[4:endPos]) {
				sb.WriteString(fmLineDisplay(line))
			}
			return "\\-\\-\\-\n" + sb.String() + "\\-\\-\\-\n\n" + text[endPos+5:]
		}
	}

	lines := splitLines(text)
	mmdEnd := 0
	for mmdEnd < len(lines) && reMMDLine.MatchString(lines[mmdEnd]) {
		mmdEnd++
	}
	if mmdEnd > 0 && (mmdEnd >= len(lines) || strings.TrimRight(lines[mmdEnd], "\n") == "") {
		var sb strings.Builder
		for _, line := range lines[:mmdEnd] {
			sb.WriteString(fmLineDisplay(line))
		}
		restStart := mmdEnd
		if mmdEnd < len(lines) {
			restStart = mmdEnd + 1
		}
		return sb.String() + "\n" + strings.Join(lines[restStart:], "")
	}

	return text
}

func leadingSpaces(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			return i
		}
	}
	return len(s)
}

func processLines(text string) string {
	fenceIndent := -1
	fenceTicks := -1
	var sb strings.Builder

	for _, line := range splitLines(text) {
		chomped := strings.TrimRight(line, "\n")

		if fenceIndent < 0 {
			if m := reFenceOpen.FindStringSubmatch(chomped); m != nil {
				fenceIndent = len(m[1])
				fenceTicks = len(m[2])
				sb.WriteString(line)
				continue
			}
		} else {
			if m := reFenceClose.FindStringSubmatch(chomped); m != nil &&
				len(m[1]) <= fenceIndent && len(m[2]) >= fenceTicks {
				fenceIndent = -1
				fenceTicks = -1
			}
			sb.WriteString(line)
			continue
		}

		switch {
		case reProject.MatchString(chomped):
			m := reProject.FindStringSubmatch(chomped)
			tabs := len(m[1])
			noTabs := strings.TrimLeft(chomped, "\t")
			sb.WriteString(nbspIndent(max(tabs-1, 0)) + "**" + strings.TrimRight(noTabs, " \t") + "**{.h4text}  \n")

		case reTabTask.MatchString(chomped):
			m := reTabTask.FindStringSubmatch(chomped)
			tabs := len(m[1])
			sb.WriteString(nbspIndent(max(tabs-1, 0)) + "\\- " + strings.TrimRight(m[2], " \t") + "  \n")

		case strings.HasPrefix(chomped, "    "):
			spaces := leadingSpaces(chomped)
			level := spaces / 4
			rest := chomped[spaces:]
			if strings.HasPrefix(rest, "- ") {
				sb.WriteString(nbspIndent(max(level-1, 0)) + "\\- " + strings.TrimRight(rest[2:], " \t") + "  \n")
			} else {
				sb.WriteString(line)
			}

		case reH12.MatchString(chomped):
			m := reH12.FindStringSubmatch(chomped)
			hashes := m[1]
			content := strings.TrimRight(m[2], " \t")
			prefix := strings.Repeat("\\#", len(hashes))
			sb.WriteString(hashes + " **" + prefix + " " + content + "**\n")

		case reH34.MatchString(chomped):
			m := reH34.FindStringSubmatch(chomped)
			prefix := strings.Repeat("\\#", len(m[1]))
			sb.WriteString("**" + prefix + " " + strings.TrimRight(m[2], " \t") + "**{.h3text}  \n")

		default:
			sb.WriteString(line)
		}
	}

	return sb.String()
}

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var payload map[string]json.RawMessage
	json.Unmarshal(data, &payload)
	var text string
	json.Unmarshal(payload["text"], &text)

	os.Stdout.WriteString(processLines(processFrontmatter(text)))
}
