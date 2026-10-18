package render

import "testing"

func TestLinkify(t *testing.T) {
	href := func(s string) string {
		if s == "fanir7" || s == "igoro8" {
			return "#" + s
		}
		return ""
	}
	cases := []struct{ in, want string }{
		{"see fanir7.", "see [fanir7](#fanir7)."},
		{"fanir7, igoro8 and banto3", "[fanir7](#fanir7), [igoro8](#igoro8) and banto3"},
		{"(fanir7) **fanir7**", "([fanir7](#fanir7)) **[fanir7](#fanir7)**"},
		{"fanir7x fanir7-y xfanir7 fanir", "fanir7x fanir7-y xfanir7 fanir"},
		{"`fanir7` and ``a ` fanir7``", "`fanir7` and ``a ` fanir7``"},
		{"[fanir7](#fanir7) [see fanir7](http://x) ![fanir7](img.png)", "[fanir7](#fanir7) [see fanir7](http://x) ![fanir7](img.png)"},
		{"[fanir7][ref] then fanir7", "[fanir7][ref] then [fanir7](#fanir7)"},
		{"[fanir7]", "[[fanir7](#fanir7)]"},
		{"<https://x/fanir7> <span title=fanir7>", "<https://x/fanir7> <span title=fanir7>"},
		{"https://x/fanir7?q=igoro8 www.x.org/fanir7 then fanir7", "https://x/fanir7?q=igoro8 www.x.org/fanir7 then [fanir7](#fanir7)"},
		{"[ref]: https://x/fanir7 \"fanir7\"", "[ref]: https://x/fanir7 \"fanir7\""},
		{`\` + "`fanir7` " + `\[fanir7\]`, `\` + "`[fanir7](#fanir7)` " + `\[[fanir7](#fanir7)\]`},
		{"```go\nfanir7\n```\nfanir7", "```go\nfanir7\n```\n[fanir7](#fanir7)"},
		{"~~~\nfanir7\n```\nfanir7\n~~~~\nfanir7\n", "~~~\nfanir7\n```\nfanir7\n~~~~\n[fanir7](#fanir7)\n"},
		{"``` not `a fence\nfanir7", "``` not `a fence\n[fanir7](#fanir7)"},
		{"unclosed `code fanir7", "unclosed `code [fanir7](#fanir7)"},
	}
	for _, c := range cases {
		if got := Linkify(c.in, href); got != c.want {
			t.Errorf("Linkify(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}
