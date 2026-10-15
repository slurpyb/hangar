package engine

import (
	"strings"

	"github.com/louisescher/hangar/internal/agents"
	"github.com/louisescher/hangar/internal/discover"
)

// Agent is the resolved-agent type surfaced to callers (CLI and TUI).
type Agent = agents.Agent

// SkillDoc is a skill's manifest split for display: the parsed skill metadata
// plus the markdown body (frontmatter removed), ready for glamour rendering.
type SkillDoc struct {
	Skill Skill
	Body  string
}

// ReadSkillDoc loads authored content and returns its body for preview. The content's
// AbsPath must still be on disk (true while its source's Discovered is open).
func (e *Engine) ReadSkillDoc(sk Skill) (SkillDoc, error) {
	data, err := discover.ReadContent(sk)
	if err != nil {
		return SkillDoc{}, err
	}
	if sk.Type() == "rule" {
		return SkillDoc{Skill: sk, Body: string(data)}, nil
	}
	fm, body := discover.SplitFrontmatter(data)
	sk.FrontmatterRaw = strings.TrimSpace(string(fm))
	return SkillDoc{Skill: sk, Body: string(body)}, nil
}

// DetectAgents returns the agents detected on this machine for the given scope.
func (e *Engine) DetectAgents(global bool) ([]agents.Agent, error) {
	return agents.Detect(global)
}

// ResolveAgents resolves agent names (and aliases) to concrete install targets.
func (e *Engine) ResolveAgents(names []string, global bool) ([]agents.Agent, error) {
	out, _, err := agents.Resolve(names, global)
	return out, err
}

// AgentDefs returns the full static agent matrix (for selection UIs).
func (e *Engine) AgentDefs() []agents.Def {
	return agents.Defs
}
