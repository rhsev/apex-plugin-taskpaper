package main

import "testing"

func TestTags(t *testing.T) {
	cases := []struct{ in, want string }{
		// prose and tasks: tags wrapped, addresses and handles left alone
		{"Text with @na tag.\n", "Text with **@na**{.atag} tag.\n"},
		{"Mail an x@y.de, youtube.com/@name\n", "Mail an x@y.de, youtube.com/@name\n"},
		{"\t- Call @due(2026-10-20) @na\n", "\\- Call **@due(2026-10-20)**{.atag} **@na**{.atag}  \n"},
		// inline code passes verbatim, also with a longer backtick run
		{"Use `@scope/pkg` and ``a ` @b`` but @c\n", "Use `@scope/pkg` and ``a ` @b`` but **@c**{.atag}\n"},
		{"A lone ` then @d\n", "A lone ` then **@d**{.atag}\n"},
		// project lines are wrapped whole; their tags stay plain inside
		{"House: @home\n", "**House: @home**{.h4text}  \n"},
		// code blocks are never touched, TaskPaper blocks included
		{"```tp\nHouse: @home\n\t- Clean @na\n```\nAfter @x\n", "```tp\nHouse: @home\n\t- Clean @na\n```\nAfter **@x**{.atag}\n"},
		{"~~~\n@decorator\n~~~\n", "~~~\n@decorator\n~~~\n"},
	}
	for _, c := range cases {
		if got := processLines(c.in); got != c.want {
			t.Errorf("processLines(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestNestedListKeepsTags(t *testing.T) {
	in := "- item @a\n\t- nested @b\n"
	want := "- item **@a**{.atag}\n\t- nested **@b**{.atag}\n"
	if got := processLines(in); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
