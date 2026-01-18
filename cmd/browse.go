package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	browseMargin = config.DefaultBrowseMargin()
	docStyle     = lipgloss.NewStyle().Margin(0, browseMargin)
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	stateStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	tagStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type viewMode int

const (
	modeBrowse viewMode = iota
	modeNote
	modeState
)

type editorFinishedMsg struct {
	err error
}

type model struct {
	tasks     []*task.Task
	cursor    int
	width     int
	height    int
	mode      viewMode
	textinput textinput.Model
	err       error
}

func initialModel(tasks []*task.Task) model {

	ti := textinput.New()

	ti.Placeholder = "Your note..."

	ti.CharLimit = 256

	ti.Width = 80

	return model{
		tasks:     tasks,
		cursor:    0,
		mode:      modeBrowse,
		textinput: ti,
		err:       nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textinput.Width = msg.Width - (browseMargin * 2) - 4 // account for margin
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeBrowse:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "left", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "right", "j", " ":
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
				}
			case "n":
				m.mode = modeNote
				m.textinput.Focus()
				m.textinput.SetValue("") // clear previous input
				return m, textinput.Blink
			case "s":
				m.mode = modeState
				return m, nil
			case "e":
				if len(m.tasks) == 0 {
					return m, nil
				}
				currentTask := m.tasks[m.cursor]
				editorCommand, editorArgs := resolveEditorCommand()
				if shouldStartInsert(editorCommand) {
					editorArgs = append(editorArgs, "+startinsert")
				}
				if shouldStartGoyo(editorCommand) {
					editorArgs = append(editorArgs, "+Goyo")
				}
				editorArgs = append(editorArgs, currentTask.Path)
				cmdExec := exec.Command(editorCommand, editorArgs...)
				return m, tea.ExecProcess(cmdExec, func(err error) tea.Msg {
					return editorFinishedMsg{err: err}
				})
			}
		case modeNote:
			switch msg.String() {
			case "enter":
				noteText := m.textinput.Value()
				if noteText != "" {
					currentTask := m.tasks[m.cursor]
					addNote(currentTask, noteText)
					if err := currentTask.Write(currentTask.Path); err != nil {
						m.err = err
					}
				}
				m.mode = modeBrowse
				m.textinput.Blur()
				return m, nil
			case "esc":
				m.mode = modeBrowse
				m.textinput.Blur()
				return m, nil
			}
		case modeState:
			var newState task.State
			switch msg.String() {
			case "t":
				newState = task.StateTodo
			case "g":
				newState = task.StateBegun
			case "d":
				newState = task.StateDone
			case "b":
				newState = task.StateBlock
			case "c":
				newState = task.StateConfirm
			case "n":
				newState = task.StateNotDo
			case "esc", "q":
				m.mode = modeBrowse
				return m, nil
			default:
				return m, nil // no state change
			}

			currentTask := m.tasks[m.cursor]
			oldState := currentTask.State
			if oldState != newState {
				changeState(currentTask, newState)
				logMsg := fmt.Sprintf("State changed from %s to %s", oldState, newState)
				addLog(currentTask, logMsg)
				if err := currentTask.Write(currentTask.Path); err != nil {
					m.err = err
				}
			}
			m.mode = modeBrowse
			return m, nil
		}
	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil
	}

	m.textinput, cmd = m.textinput.Update(msg)
	return m, cmd
}

func addNote(t *task.Task, message string) {
	noteEntry := fmt.Sprintf("- %s: %s", time.Now().UTC().Format(time.RFC3339), message)
	preBody, logSection, afterLog, logFound := splitSection(t.Body, "## Log")
	if logFound {
		preBody = joinBlocks(preBody, afterLog)
	}

	beforeNotes, notesSection, afterNotes, notesFound := splitSection(preBody, "## Notes")
	if !notesFound {
		notesSection = "## Notes"
	}

	notesSection = appendEntry(notesSection, noteEntry)
	newPreBody := joinBlocks(beforeNotes, notesSection, afterNotes)
	if logFound {
		t.Body = joinBlocks(newPreBody, logSection)
	} else {
		t.Body = newPreBody
	}
	t.UpdatedAt = time.Now()
}

func addLog(t *task.Task, message string) {
	logEntry := fmt.Sprintf("- %s: %s", time.Now().UTC().Format(time.RFC3339), message)
	preBody, logSection, afterLog, logFound := splitSection(t.Body, "## Log")
	if !logFound {
		logSection = "## Log"
	}
	logSection = appendEntry(logSection, logEntry)
	newBody := joinBlocks(preBody, afterLog, logSection)
	t.Body = newBody
	t.UpdatedAt = time.Now()
}

func changeState(t *task.Task, newState task.State) {
	t.State = newState
	t.UpdatedAt = time.Now()
	if newState == task.StateBegun {
		now := time.Now()
		t.StartedAt = &now
	} else if newState == task.StateDone {
		now := time.Now()
		t.CompletedAt = &now
	}
}

func formatDisplayTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02-1504")
}

func (m model) View() string {
	if len(m.tasks) == 0 {
		return docStyle.Render("No tasks to display.")
	}
	if m.width == 0 {
		return "..."
	}

	var mainContent string
	var help string

	currentTask := m.tasks[m.cursor]
	contentWidth := m.width - (browseMargin * 2) - 4
	bodyStyle := lipgloss.NewStyle().Width(contentWidth)
	var content strings.Builder

	// 1. Filename (base name)
	content.WriteString(filepath.Base(currentTask.Path) + "\n")

	// 2. Tags
	if len(currentTask.Tags) > 0 {
		formattedTags := make([]string, len(currentTask.Tags))
		for i, tag := range currentTask.Tags {
			formattedTags[i] = "#" + tag
		}
		content.WriteString(tagStyle.Render(strings.Join(formattedTags, ", ")) + "\n")
	}

	// 3. Blank line
	content.WriteString("\n")

	// 4. # short title (currentTask.Title)
	content.WriteString(titleStyle.Render(currentTask.Title) + "\n")

	// 5. Blank line
	content.WriteString("\n")

	// 6. STATUS (currentTask.State)
	content.WriteString(stateStyle.Render(string(currentTask.State)) + "\n")

	// 7. Blank line (between status and content)
	content.WriteString("\n")

	// Body rendering with sections
	notesBody, logSection, _, logFound := splitSection(currentTask.Body, "## Log")
	mainNotes, notesSection, _, notesFound := splitSection(notesBody, "## Notes")
	mainNotes = strings.TrimSpace(mainNotes)

	if mainNotes != "" {
		content.WriteString(bodyStyle.Render(mainNotes))
		content.WriteString("\n") // Ensure space after main notes if present
	}
	if notesFound {
		var notesContent strings.Builder
		notesContent.WriteString("\n## Notes\n") // Removed extra newline for better spacing
		for _, line := range strings.Split(notesSection, "\n") {
			if strings.HasPrefix(line, "- ") {
				bullet := "- "
				parts := strings.SplitN(line[len(bullet):], ": ", 2)
				var renderedNote string
				if len(parts) == 2 {
					timestamp, message := parts[0], parts[1]
					formattedTs := formatDisplayTimestamp(timestamp)
					fullText := formattedTs + ": " + message
					noteStyle := lipgloss.NewStyle().Width(contentWidth - len(bullet))
					renderedNote = lipgloss.JoinHorizontal(lipgloss.Top, bullet, noteStyle.Render(fullText))
				} else {
					noteStyle := lipgloss.NewStyle().Width(contentWidth - len(bullet))
					renderedNote = lipgloss.JoinHorizontal(lipgloss.Top, bullet, noteStyle.Render(line[len(bullet):]))
				}
				notesContent.WriteString(renderedNote + "\n")
			}
		}
		content.WriteString(notesContent.String())
	}
	if logFound {
		var logContent strings.Builder
		logContent.WriteString("\n## Log\n") // Removed extra newline for better spacing
		for _, line := range strings.Split(logSection, "\n") {
			if strings.HasPrefix(line, "- ") {
				bullet := "- "
				parts := strings.SplitN(line[len(bullet):], ": ", 2)
				var renderedLog string
				if len(parts) == 2 {
					timestamp, message := parts[0], parts[1]
					formattedTs := formatDisplayTimestamp(timestamp)
					fullText := formattedTs + ": " + message
					logStyle := lipgloss.NewStyle().Width(contentWidth - len(bullet))
					renderedLog = lipgloss.JoinHorizontal(lipgloss.Top, bullet, logStyle.Render(fullText))
				} else {
					logStyle := lipgloss.NewStyle().Width(contentWidth - len(bullet))
					renderedLog = lipgloss.JoinHorizontal(lipgloss.Top, bullet, logStyle.Render(line[len(bullet):]))
				}
				logContent.WriteString(renderedLog + "\n")
			}
		}
		content.WriteString(logContent.String())
	}
	mainContent = content.String()

	switch m.mode {
	case modeNote:
		help = "enter: save note  esc: cancel\n" + m.textinput.View()
	case modeState:
		help = "t: todo  g: begun  d: done  b: block  c: confirm  n: notdo  esc: cancel"
	default: // modeBrowse
		help = "←/k: prev  →/j/space: next  n: new note  s: set state  e: edit  q: quit"
	}

	contentHeight := lipgloss.Height(mainContent)
	helpHeight := lipgloss.Height(help)
	spacerHeight := m.height - contentHeight - helpHeight - 0
	if spacerHeight < 0 {
		spacerHeight = 0
	}
	spacer := strings.Repeat("\n", spacerHeight)

	return docStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		mainContent,
		spacer,
		helpStyle.Render(help),
	))
}

func newBrowseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "browse [state]",
		Short:             "Browse tasks in an interactive viewer",
		Long:              "Browse tasks one by one in an interactive full-screen viewer. Keys: \u2190/\u2192 to move, n to add a note, s to set state, e to edit, q to quit.",
		ValidArgsFunction: stateArgCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			browseMargin = loadBrowseMargin()
			docStyle = lipgloss.NewStyle().Margin(0, browseMargin)

			tasksPath, err := tasksDir()
			if err != nil {
				if printNotPunchlistError(err) {
					return nil
				}
				return fmt.Errorf("error locating tasks: %w", err)
			}

			if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
				fmt.Println("No tasks found.")
				return nil
			}

			var filterState task.State
			if len(args) > 0 {
				if parsed, ok := task.ParseState(args[0]); ok {
					filterState = parsed
				} else {
					filterState = task.State(args[0])
				}
			}

			var allTasks []*task.Task
			err = filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
					t, err := task.Parse(path)
					if err != nil {
						return nil
					}
					if filterState != "" && t.State != filterState {
						return nil
					}
					allTasks = append(allTasks, t)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("error listing tasks: %w", err)
			}

			stateOrder := []string{"BEGUN", "TODO", "NOTDO", "DONE"}
			orderIndex := buildStateOrderIndex(stateOrder)
			sort.Slice(allTasks, func(i, j int) bool {
				ai := orderIndex[stateOrderKey(allTasks[i].State)]
				aj := orderIndex[stateOrderKey(allTasks[j].State)]
				if ai == aj {
					return allTasks[i].ID < allTasks[j].ID
				}
				return ai < aj
			})
			p := tea.NewProgram(initialModel(allTasks), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("error running browse program: %w", err)
			}
			return nil
		},
	}
	return cmd
}

func loadBrowseMargin() int {
	cfg, err := config.LoadConfig()
	if err != nil || cfg.BrowseMargin <= 0 {
		return config.DefaultBrowseMargin()
	}
	return cfg.BrowseMargin
}
