package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// optionsModel lets the user set the install scope and sanitization passes
// before reviewing. Changes persist to preferences.
type optionsModel struct {
	cursor int
}

// optionRows is the fixed set of toggles, in display order.
const (
	optScope = iota
	optStripInvisible
	optStripComments
	optPiRulesFile
	optRowCount
)

func (s *optionsModel) enter(app *App) tea.Cmd {
	if s.cursor >= s.rowCount(app) {
		s.cursor = s.rowCount(app) - 1
	}
	return nil
}
func (s *optionsModel) capturing() bool { return false }

func (s *optionsModel) update(app *App, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(k, app.keys.Up):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(k, app.keys.Down):
		if s.cursor < s.rowCount(app)-1 {
			s.cursor++
		}
	case key.Matches(k, app.keys.Confirm):
		return func() tea.Msg { return nav(stateConfirm) }
	case key.Matches(k, app.keys.Toggle), k.String() == "left", k.String() == "right", k.String() == "h", k.String() == "l":
		s.toggle(app, s.cursor)
	case key.Matches(k, app.keys.Back):
		return func() tea.Msg { return navBackMsg{} }
	}
	return nil
}

// toggle flips the option at row and persists the change.
func (s *optionsModel) toggle(app *App, row int) {
	switch row {
	case optScope:
		app.global = !app.global
		app.reresolveAgents()
	case optStripInvisible:
		app.security.StripInvisible = !app.security.StripInvisible
	case optStripComments:
		app.security.StripComments = !app.security.StripComments
	case optPiRulesFile:
		choices := []string{"", "AGENTS.md", "AGENTS.override.md", "APPEND_SYSTEM.md"}
		for i, choice := range choices {
			if choice == app.piRulesFile {
				app.piRulesFile = choices[(i+1)%len(choices)]
				break
			}
		}
		return
	}
	app.saveScopeSanitize()
}

func (s *optionsModel) view(app *App) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", app.th.subtitle.Render("Install options"))

	rows := []struct {
		label string
		value string
	}{
		{"Install scope", scopeValue(app)},
		{"Strip invisible characters", onOff(app.security.StripInvisible)},
		{"Strip comments (references)", onOff(app.security.StripComments)},
	}
	if hasPiRules(app) {
		rows = append(rows, struct {
			label string
			value string
		}{"Pi rules file", piRulesFileLabel(app)})
	}
	for i, r := range rows {
		cursor := "  "
		label := r.label
		if i == s.cursor {
			cursor = app.th.cursor.Render("▸ ")
			label = app.th.selected.Render(label)
		}
		fmt.Fprintf(&b, "%s%-32s %s\n", cursor, label, app.th.accent.Render(r.value))
	}

	if hasPiRules(app) {
		note := "Keep the recorded destination; new installs use APPEND_SYSTEM.md."
		switch app.piRulesFile {
		case "AGENTS.md":
			note = "Context instructions; global and ancestor context can also apply."
		case "AGENTS.override.md":
			note = "Context instructions replacing AGENTS.md/CLAUDE.md in the same directory."
		case "APPEND_SYSTEM.md":
			note = "Adds to the system prompt; the project file takes precedence over global."
		}
		fmt.Fprintf(&b, "\n%s\n", app.th.faint.Render(note))
	}
	b.WriteString("\n")
	b.WriteString(app.helpView(
		app.keys.Up, app.keys.Down,
		key.NewBinding(key.WithHelp("space", "toggle")),
		app.keys.Confirm, app.keys.Back,
	))
	return b.String()
}

func scopeValue(app *App) string {
	if app.global {
		return "global (~/)"
	}
	return "local (this project)"
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func hasPiRules(app *App) bool {
	pi := false
	for _, ag := range app.chosen {
		if ag.Def.Name == "pi" {
			pi = true
			break
		}
	}
	if !pi {
		return false
	}
	for _, sk := range app.selectedSkills() {
		if sk.Type() == "rule" && !sk.IsRuleFile() {
			return true
		}
	}
	return false
}

func (s *optionsModel) rowCount(app *App) int {
	if hasPiRules(app) {
		return optRowCount
	}
	return optRowCount - 1
}

func piRulesFileLabel(app *App) string {
	if app.piRulesFile == "" {
		return "recorded choice / APPEND_SYSTEM.md"
	}
	return app.piRulesFile
}
