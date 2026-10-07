package tui

import (
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/h2oai/tk/internal/edit"
)

// execFunc runs an external command with the terminal handed over to it, as
// tea.ExecProcess does.
type execFunc func(*exec.Cmd, tea.ExecCallback) tea.Cmd

// editDoneMsg reports that the editor for s exited, with err if it failed.
type editDoneMsg struct {
	s   *edit.Session
	err error
}

// startEdit copies id's file to a temp file and returns the command that
// suspends the TUI and runs $EDITOR on it. Errors go to the status line.
func (m *Model) startEdit(id string) tea.Cmd {
	if id == "" {
		return nil
	}
	editor, err := edit.Editor()
	if err != nil {
		m.status = err.Error()
		return nil
	}
	s, err := edit.Start(m.t, id, edit.Locker(m.lock))
	// Start reloaded the tree, even if it then failed.
	m.refresh(id, false)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	return m.exec(s.Command(editor), func(err error) tea.Msg { return editDoneMsg{s, err} })
}

// finishEdit saves the edited file, unless the editor failed, and refreshes
// the view from disk.
func (m *Model) finishEdit(msg editDoneMsg) {
	id := msg.s.ID
	if msg.err != nil {
		msg.s.Discard()
		m.status = fmt.Sprintf("editor: %v, nothing saved", msg.err)
		return
	}
	changed, err := msg.s.Finish(edit.Locker(m.lock))
	m.refresh(id, err == nil)
	switch {
	case err != nil:
		m.status = err.Error()
	case changed:
		m.status = "edited " + id
	default:
		m.status = "unchanged " + id
	}
}

// refresh rebuilds the rows and the detail view from the tree, keeping the
// cursor on id. If the ticket is gone the detail view closes. top resets the
// detail view's scroll.
func (m *Model) refresh(id string, top bool) {
	m.rebuild()
	m.follow(id)
	if m.detailID == "" {
		return
	}
	if m.t.Get(m.detailID) == nil {
		m.closeDetail()
		return
	}
	m.detail = renderDetail(m.t, m.detailID, m.width)
	if top {
		m.scroll = 0
	}
	m.scrollBy(0)
}
