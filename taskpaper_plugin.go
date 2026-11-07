package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// tagPattern is a TaskPaper @tag or @tag(value). Letters are Unicode, so
// @fällig is one tag.
const tagPattern = `@[\p{L}\p{N}_-]+(?:\([^)]*\))?`

// Whitespace classes in reProject and reListItem are spelled out (NBSP
// counts) so the Swift port in mdpreview, where ICU's \s is Unicode-wide,
// matches the same lines.
var (
	reFenceOpen  = regexp.MustCompile("^([ \t]*)(`{3,}|~{3,})")
	reFenceClose = regexp.MustCompile("^([ \t]*)(`{3,}|~{3,})\\s*$")
	reProject    = regexp.MustCompile(`^(\t*)[^ \t\x{a0}#>].*:[ \t\x{a0}]*(?:` + tagPattern + `[ \t\x{a0}]*)*$`)
	reListItem   = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]+[.)])(?:[ \t]|$)`)
	reTabTask    = regexp.MustCompile(`^(\t+)- (.*)`)
	reH12        = regexp.MustCompile(`^(##?) (.+)$`)
	reH34        = regexp.MustCompile(`^(####?) (.+)$`)
	reFMLine     = regexp.MustCompile(`^([^:\n]+):\s*(.*)`)
	reMMDLine    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 _-]*:\s+\S`)
	reMMDKey     = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} _-]*:[ \t]+\S`)
	// A tag starts the line or follows whitespace (TaskPaper's rule), so mail
	// addresses and URL handles (youtube.com/@name) stay literal. RE2 has no
	// lookbehind; the whitespace is captured and written back.
	reTag = regexp.MustCompile(`(^|[ \t\x{a0}])(` + tagPattern + `)`)
)

const doubleNbsp = "\u00a0\u00a0"

// trailingSpace is trimmed off emitted lines; a trailing NBSP inside **…**
// would keep the closing ** from closing.
const trailingSpace = " \t\u00a0"

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

// yamlBlock splits a leading `---` block, closed by the first `---` or `...`
// line, into its body and the text after the closing line.
func yamlBlock(text string) (body, rest string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", false
	}
	for off := 4; off < len(text); {
		next := len(text)
		line := text[off:]
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
			next = off + i + 1
		}
		if t := strings.TrimRight(line, " \t"); t == "---" || t == "..." {
			return text[4:off], text[next:], true
		}
		off = next
	}
	return "", "", false
}

// mmdBlock returns the number of leading MultiMarkdown metadata lines: a run
// of `Key: value` lines ended by a blank line or the end of the text.
func mmdBlock(lines []string) int {
	n := 0
	for n < len(lines) && reMMDLine.MatchString(lines[n]) {
		n++
	}
	if n > 0 && (n >= len(lines) || strings.TrimRight(lines[n], "\n") == "") {
		return n
	}
	return 0
}

// metaKey returns the lowercased top-level key of a metadata line, or "" for
// indented lines (nested values, list items) that belong to the key above.
func metaKey(line string) string {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return ""
	}
	if m := reFMLine.FindStringSubmatch(strings.TrimRight(line, "\n")); m != nil {
		return strings.ToLower(strings.TrimSpace(m[1]))
	}
	return ""
}

// fileKeys returns the metadata keys the input file itself declares. The apex
// CLI merges config.yml (terminal.width, …) into the frontmatter it hands to
// plugins, so the file is the only record of what the document says. ok is
// false when there is no file to ask (stdin); then every key is shown.
func fileKeys() (keys map[string]bool, ok bool) {
	path := os.Getenv("APEX_FILE_PATH")
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	var lines []string
	if body, _, found := yamlBlock(text); found {
		lines = splitLines(body)
	} else {
		// apex's own MMD rule, looser than mmdBlock: no blank line needed, so
		// a first line like "Inbox: @today" is metadata to apex.
		for _, line := range splitLines(text) {
			if !reMMDKey.MatchString(line) {
				break
			}
			lines = append(lines, line)
		}
	}
	keys = map[string]bool{}
	for _, line := range lines {
		if k := metaKey(line); k != "" {
			keys[k] = true
		}
	}
	return keys, true
}

// displayMeta renders metadata lines as `**key:**{.fmkey} value` with hard
// breaks, nested lines indented below their key. Keys the file doesn't
// declare are dropped; apex flattens nested keys to `parent.child`.
func displayMeta(lines []string) string {
	keys, filter := fileKeys()
	var sb strings.Builder
	keep := true
	for _, line := range lines {
		if k := metaKey(line); k != "" {
			keep = !filter || keys[k] || keys[strings.SplitN(k, ".", 2)[0]]
		}
		chomped := strings.TrimRight(line, "\n")
		trimmed := strings.TrimLeft(chomped, " \t")
		if !keep || trimmed == "" {
			continue
		}
		indent := strings.Repeat("\u00a0", len(chomped)-len(trimmed))
		if m := reFMLine.FindStringSubmatch(trimmed); m != nil && !strings.HasPrefix(trimmed, "- ") {
			sb.WriteString(indent + "**" + m[1] + ":**{.fmkey} " + m[2] + "  \n")
			continue
		}
		if strings.ContainsAny(trimmed[:1], "#->*+") {
			trimmed = "\\" + trimmed
		}
		sb.WriteString(indent + trimmed + "  \n")
	}
	return sb.String()
}

// terminalTarget reports whether the output is apex's terminal format. Since
// 1.1.50 apex prints the document's metadata above terminal output itself.
// apex does not tell plugins the target format, so the caller sets
// APEX_TARGET_FORMAT (e.g. terminal256).
func terminalTarget() bool {
	return strings.HasPrefix(os.Getenv("APEX_TARGET_FORMAT"), "terminal")
}

// processFrontmatter splits leading metadata off as display text (head),
// which must not go through processLines: a value ending in ":" would turn
// into a project header. For terminal output head stays empty.
func processFrontmatter(text string) (head, rest string) {
	if body, rest, ok := yamlBlock(text); ok {
		if terminalTarget() {
			return "", rest
		}
		if meta := displayMeta(splitLines(body)); meta != "" {
			return "\\-\\-\\-\n" + meta + "\\-\\-\\-\n\n", rest
		}
		return "", rest
	}

	lines := splitLines(text)
	if n := mmdBlock(lines); n > 0 {
		restStart := n
		if n < len(lines) {
			restStart = n + 1
		}
		rest := strings.Join(lines[restStart:], "")
		if terminalTarget() {
			return "", rest
		}
		if meta := displayMeta(lines[:n]); meta != "" {
			return meta + "\n", rest
		}
		return "", rest
	}

	return "", text
}

func leadingSpaces(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			return i
		}
	}
	return len(s)
}

// applyTags wraps @tags as `**@tag**{.atag}` outside inline code spans,
// which pass verbatim. Code blocks never get here: processLines hands fenced
// lines through untouched.
func applyTags(s string) string {
	var sb strings.Builder
	for s != "" {
		open := strings.IndexByte(s, '`')
		if open < 0 {
			break
		}
		run := len(s[open:]) - len(strings.TrimLeft(s[open:], "`"))
		closeAt := closingRun(s[open+run:], run)
		if closeAt < 0 {
			// An unmatched run is literal backticks; tags after it still count.
			sb.WriteString(wrapTags(s[:open+run]))
			s = s[open+run:]
			continue
		}
		end := open + run + closeAt + run
		sb.WriteString(wrapTags(s[:open]))
		sb.WriteString(s[open:end])
		s = s[end:]
	}
	sb.WriteString(wrapTags(s))
	return sb.String()
}

// closingRun returns the index of the next run of exactly n backticks in s,
// or -1.
func closingRun(s string, n int) int {
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		if j-i == n {
			return i
		}
		i = j
	}
	return -1
}

func wrapTags(s string) string {
	return reTag.ReplaceAllString(s, "$1**$2**{.atag}")
}

func processLines(text string) string {
	fenceIndent := -1
	fenceTicks := -1
	var fenceChar byte
	// afterList: the last non-blank line belongs to a Markdown list passed
	// through as-is, so an indented "- " below it nests in that list rather
	// than starting a TaskPaper task.
	afterList := false
	var sb strings.Builder

	for _, line := range splitLines(text) {
		chomped := strings.TrimRight(line, "\n")

		if fenceIndent < 0 {
			if m := reFenceOpen.FindStringSubmatch(chomped); m != nil {
				fenceIndent = len(m[1])
				fenceTicks = len(m[2])
				fenceChar = m[2][0]
				afterList = false
				sb.WriteString(line)
				continue
			}
		} else {
			if m := reFenceClose.FindStringSubmatch(chomped); m != nil &&
				len(m[1]) <= fenceIndent && len(m[2]) >= fenceTicks && m[2][0] == fenceChar {
				fenceIndent = -1
				fenceTicks = -1
			}
			sb.WriteString(line)
			continue
		}

		if strings.TrimSpace(chomped) == "" {
			sb.WriteString(line)
			continue
		}

		// @tags are wrapped only on task and prose lines. Project and heading
		// lines are wrapped whole in `**…**{.class}`; a `**…**{.atag}` inside
		// would nest emphasis, which apex misparses, so their tags ride along
		// as plain text. passed: the line goes through as-is, tags aside.
		out := line
		passed := true
		isListItem := reListItem.MatchString(chomped)
		indented := chomped[0] == '\t' || chomped[0] == ' '

		switch {
		case afterList && isListItem && indented:
			// nested Markdown list: apex renders it

		case reTabTask.MatchString(chomped):
			// before projects: in TaskPaper, "- " makes a task even when it ends in ":"
			m := reTabTask.FindStringSubmatch(chomped)
			tabs := len(m[1])
			out = nbspIndent(max(tabs-1, 0)) + "\\- " + applyTags(strings.TrimRight(m[2], trailingSpace)) + "  \n"
			passed = false

		// "- x:" is a task in TaskPaper; "00. Inbox:" or "+ Ideas:" stay projects
		case !strings.HasPrefix(strings.TrimLeft(chomped, "\t"), "- ") && reProject.MatchString(chomped):
			m := reProject.FindStringSubmatch(chomped)
			tabs := len(m[1])
			noTabs := strings.TrimLeft(chomped, "\t")
			out = nbspIndent(max(tabs-1, 0)) + "**" + strings.TrimRight(noTabs, trailingSpace) + "**{.h4text}  \n"
			passed = false

		case strings.HasPrefix(chomped, "    "):
			spaces := leadingSpaces(chomped)
			level := spaces / 4
			rest := chomped[spaces:]
			if strings.HasPrefix(rest, "- ") {
				out = nbspIndent(max(level-1, 0)) + "\\- " + applyTags(strings.TrimRight(rest[2:], trailingSpace)) + "  \n"
				passed = false
			}

		case reH12.MatchString(chomped):
			m := reH12.FindStringSubmatch(chomped)
			hashes := m[1]
			content := strings.TrimRight(m[2], trailingSpace)
			prefix := strings.Repeat("\\#", len(hashes))
			out = hashes + " **" + prefix + " " + content + "**\n"
			passed = false

		case reH34.MatchString(chomped):
			m := reH34.FindStringSubmatch(chomped)
			prefix := strings.Repeat("\\#", len(m[1]))
			out = "**" + prefix + " " + strings.TrimRight(m[2], trailingSpace) + "**{.h3text}  \n"
			passed = false
		}

		// only a top-level item starts a Markdown list; indented ones continue it
		afterList = isListItem && passed && (afterList || !indented)
		if passed {
			out = applyTags(line)
		}
		sb.WriteString(out)
	}

	return sb.String()
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	var payload struct {
		Text *string `json:"text"`
	}
	if err == nil {
		err = json.Unmarshal(data, &payload)
	}
	if err == nil && payload.Text == nil {
		err = fmt.Errorf(`request has no "text" field`)
	}
	if err != nil {
		// apex takes stdout as the new document whatever the exit status,
		// so the most this can do is say why it came out empty.
		fmt.Fprintln(os.Stderr, "taskpaper_plugin:", err)
		os.Exit(1)
	}

	// cmark treats CRLF like LF; normalizing keeps every pattern LF-only.
	head, rest := processFrontmatter(strings.ReplaceAll(*payload.Text, "\r\n", "\n"))
	os.Stdout.WriteString(head + processLines(rest))
}
