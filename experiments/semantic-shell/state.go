package semanticshell

import (
	"errors"

	agenttui "github.com/spice-framework/spice-agent-tui"
)

type semanticState struct {
	initialized bool
	revision    uint64
	workspace   semanticWorkspace
	status      semanticStatus
	activity    []string
	history     []string
}

type semanticView struct {
	Revision      uint64            `json:"revision"`
	Workspace     semanticWorkspace `json:"workspace"`
	Status        semanticStatus    `json:"status"`
	Activity      []string          `json:"activity"`
	PromptHistory []string          `json:"prompt_history"`
}

type semanticWorkspace struct {
	Title    string            `json:"title"`
	Sections []semanticSection `json:"sections"`
}

type semanticSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type semanticStatus struct {
	Level   string   `json:"level"`
	Message string   `json:"message"`
	Hints   []string `json:"hints"`
}

func (state *semanticState) apply(update agenttui.SessionUpdate) (semanticView, error) {
	if err := update.Validate(); err != nil || update.Revision() <= state.revision {
		return semanticView{}, ErrSessionReceive
	}
	switch update.Kind() {
	case agenttui.SessionUpdateSnapshot:
		snapshot, present := update.Snapshot()
		if !present {
			return semanticView{}, ErrSessionReceive
		}
		state.applySnapshot(snapshot)
	case agenttui.SessionUpdateActivity:
		if !state.initialized {
			return semanticView{}, ErrSessionReceive
		}
		activity, present := update.Activity()
		if !present {
			return semanticView{}, ErrSessionReceive
		}
		state.activity = append(state.activity, activity.String())
		state.trimActivity()
	case agenttui.SessionUpdatePromptHistory:
		if !state.initialized {
			return semanticView{}, ErrSessionReceive
		}
		history, present := update.PromptHistory()
		if !present {
			return semanticView{}, ErrSessionReceive
		}
		state.history = textStrings(history)
	default:
		return semanticView{}, ErrSessionReceive
	}
	state.revision = update.Revision()
	return state.view(), nil
}

func (state *semanticState) applySnapshot(snapshot agenttui.SessionSnapshot) {
	workspace := snapshot.Workspace()
	sections := workspace.Sections()
	state.workspace = semanticWorkspace{Title: workspace.Title().String(), Sections: make([]semanticSection, len(sections))}
	for index, section := range sections {
		state.workspace.Sections[index] = semanticSection{
			Title: section.Title().String(), Body: section.Body().String(),
		}
	}
	status := snapshot.Status()
	state.status = semanticStatus{
		Level: string(status.Level()), Message: status.Message().String(), Hints: textStrings(status.Hints()),
	}
	state.activity = textStrings(snapshot.Activity())
	state.history = textStrings(snapshot.PromptHistory())
	state.initialized = true
}

func (state *semanticState) trimActivity() {
	for len(state.activity) > 0 {
		texts := make([]agenttui.Text, len(state.activity))
		for index, value := range state.activity {
			texts[index], _ = agenttui.NewText(value)
		}
		editor, _ := agenttui.NewEditor("")
		workspace, workspaceErr := publicWorkspace(state.workspace)
		status, statusErr := publicStatus(state.status)
		if workspaceErr == nil && statusErr == nil {
			if _, err := agenttui.NewViewData(workspace, status, editor, texts); err == nil {
				return
			}
		}
		state.activity = state.activity[1:]
	}
}

func publicWorkspace(value semanticWorkspace) (agenttui.WorkspaceState, error) {
	title, err := agenttui.NewText(value.Title)
	if err != nil {
		return agenttui.WorkspaceState{}, err
	}
	sections := make([]agenttui.Section, len(value.Sections))
	for index, current := range value.Sections {
		sectionTitle, titleErr := agenttui.NewText(current.Title)
		body, bodyErr := agenttui.NewText(current.Body)
		if titleErr != nil || bodyErr != nil {
			return agenttui.WorkspaceState{}, errors.New("semantic state is invalid")
		}
		sections[index], err = agenttui.NewSection(sectionTitle, body)
		if err != nil {
			return agenttui.WorkspaceState{}, err
		}
	}
	return agenttui.NewWorkspace(title, sections)
}

func publicStatus(value semanticStatus) (agenttui.StatusState, error) {
	message, err := agenttui.NewText(value.Message)
	if err != nil {
		return agenttui.StatusState{}, err
	}
	hints := make([]agenttui.Text, len(value.Hints))
	for index, current := range value.Hints {
		hints[index], err = agenttui.NewText(current)
		if err != nil {
			return agenttui.StatusState{}, err
		}
	}
	return agenttui.NewStatus(agenttui.StatusLevel(value.Level), message, hints)
}

func (state semanticState) view() semanticView {
	return semanticView{
		Revision: state.revision,
		Workspace: semanticWorkspace{
			Title: state.workspace.Title, Sections: append([]semanticSection(nil), state.workspace.Sections...),
		},
		Status: semanticStatus{
			Level: state.status.Level, Message: state.status.Message, Hints: append([]string(nil), state.status.Hints...),
		},
		Activity: append([]string(nil), state.activity...), PromptHistory: append([]string(nil), state.history...),
	}
}

func textStrings(values []agenttui.Text) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}
