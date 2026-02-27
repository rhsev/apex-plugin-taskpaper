---
project: apex-plugin-taskpaper
area: development
status: active
---

# apex-plugin-taskpaper

A note that mixes Markdown and TaskPaper. YAML frontmatter above is normally
hidden by Markdown renderers — this plugin displays it.

## Tasks

Work:
	- Write README @done(2026-02-27)
	- Add example file @done(2026-02-27)
	- Take screenshot @priority(1)
	- Submit to apex-plugins @na
	Backlog:
		- Add Ruby fallback instructions
		Test:
			- Test on Linux

### Notes
H3 and H4 headings are rendered as inline bold (no heading block) to avoid
apex inserting a forced blank line after them.

#### Details
This keeps the output tight when alternating between headings and task blocks.

```yaml
type: project
name: „Apex Plugin"
org: „rhsev"
status: active
```

## Links

- https://github.com/ApexMarkdown/apex
- https://github.com/ttscoff/na_gem
