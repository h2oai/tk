package web

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ago formats the age d as "now", "5m ago", "3h ago", "2d ago", "3w ago",
// "4mo ago" or "2y ago", floored to its largest unit. A month is 30 days and a
// year 365; a negative age (a time in the future) is "now".
func ago(d time.Duration) string {
	const day = 24 * time.Hour
	units := []struct {
		size time.Duration
		name string
	}{{365 * day, "y"}, {30 * day, "mo"}, {7 * day, "w"}, {day, "d"}, {time.Hour, "h"}, {time.Minute, "m"}}
	for _, u := range units {
		if d >= u.size {
			return fmt.Sprintf("%d%s ago", d/u.size, u.name)
		}
	}
	return "now"
}

// timeTag renders at relative to now, with the stored UTC time in datetime
// and the server's local time as the hover title.
func timeTag(at, now time.Time) template.HTML {
	return template.HTML(fmt.Sprintf(`<time datetime="%s" title="%s">%s</time>`,
		at.UTC().Format(time.RFC3339), template.HTMLEscapeString(at.Local().Format("2006-01-02 15:04:05 MST")),
		ago(now.Sub(at))))
}

// nowKey carries the request time into the note heading transformer.
var nowKey = parser.NewContextKey()

// noteTimes rewrites every heading that reads exactly "Note <RFC3339>", as
// `tk note` writes it, into "Note · <relative time>". It runs on the parsed
// tree because raw HTML in the markdown would be dropped.
type noteTimes struct{}

func (noteTimes) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	now, ok := pc.Get(nowKey).(time.Time)
	if !ok {
		return
	}
	src := reader.Source()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		stamp, ok := strings.CutPrefix(strings.TrimSpace(string(h.Lines().Value(src))), "Note ")
		if at, err := time.Parse(time.RFC3339, stamp); ok && err == nil {
			h.RemoveChildren(h)
			h.AppendChild(h, &noteTime{At: at, Now: now})
		}
		return ast.WalkSkipChildren, nil
	})
}

var kindNoteTime = ast.NewNodeKind("NoteTime")

// noteTime is a note heading's content, rendered by noteTimeRenderer.
type noteTime struct {
	ast.BaseInline
	At, Now time.Time
}

func (n *noteTime) Kind() ast.NodeKind { return kindNoteTime }

func (n *noteTime) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

type noteTimeRenderer struct{}

func (noteTimeRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(kindNoteTime, func(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			nt := n.(*noteTime)
			_, _ = w.WriteString("Note · " + string(timeTag(nt.At, nt.Now)))
		}
		return ast.WalkContinue, nil
	})
}
