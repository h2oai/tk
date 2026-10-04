package readytree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/h2oai/tk/internal/ticket"
)

func TestRenderColor(t *testing.T) {
	bug := &ticket.Ticket{ID: "a1", Priority: 1, Type: ticket.TypeBug, Title: "Boom", Status: ticket.StatusOpen}
	task := &ticket.Ticket{ID: "b2", Priority: 2, Type: ticket.TypeTask, Title: "Plain", Status: ticket.StatusOpen}
	all := []*ticket.Ticket{bug, task}

	var buf bytes.Buffer
	if err := Render(&buf, all, all, "priority", true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, ansiYellow+"P1"+ansiReset) || !strings.Contains(out, ansiRed+"bug"+ansiReset) {
		t.Errorf("expected yellow P1 and red bug:\n%q", out)
	}
	if !strings.Contains(out, "b2 P2 task Plain") {
		t.Errorf("P2 task should be uncoloured:\n%q", out)
	}

	buf.Reset()
	Render(&buf, all, all, "priority", false)
	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("no escapes expected with color off:\n%q", buf.String())
	}
}
