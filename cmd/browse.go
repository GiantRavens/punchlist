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
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	browseMargin = config.DefaultBrowseMargin()
	helpStyle    = lipgloss.NewStyle().Faint(true)
	contentStyle = lipgloss.NewStyle()
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
	margin    int
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
		margin:    browseMargin,
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
		m.margin = effectiveBrowseMargin(msg.Width)
		m.textinput.Width = browseContentWidth(msg.Width, m.margin)
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeBrowse:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "left", "k", "K":
				if m.cursor > 0 {
					m.cursor--
				}
			case "right", "j", "J", " ":
				if m.cursor < len(m.tasks)-1 {
					m.cursor++
				}
			case "t":
				return applyStateChange(m, task.StateTodo)
			case "b":
				return applyStateChange(m, task.StateBegun)
			case "d":
				return applyStateChange(m, task.StateDone)
			case "c":
				return applyStateChange(m, task.StateConfirm)
			case "n":
				m.mode = modeNote
				m.textinput.Focus()
				m.textinput.SetValue("") // clear previous input
				return m, textinput.Blink
			case "x":
				return applyStateChange(m, task.StateNotDo)
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				return applyPriorityChange(m, msg.String())
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
					editorArgs = append(editorArgs, goyoArgs()...)
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
			case "b":
				newState = task.StateBegun
			case "d":
				newState = task.StateDone
			case "k":
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

			m, _ = applyStateChange(m, newState)
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

func applyStateChange(m model, newState task.State) (model, tea.Cmd) {
	if len(m.tasks) == 0 {
		return m, nil
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
	return m, nil
}

func applyPriorityChange(m model, key string) (model, tea.Cmd) {
	if len(m.tasks) == 0 {
		return m, nil
	}
	newPriority, ok := parsePriorityKey(key)
	if !ok {
		return m, nil
	}
	currentTask := m.tasks[m.cursor]
	oldPriority := currentTask.Priority
	if oldPriority != newPriority {
		currentTask.Priority = newPriority
		currentTask.UpdatedAt = time.Now()
		logMsg := fmt.Sprintf("Priority changed from %d to %d", oldPriority, newPriority)
		addLog(currentTask, logMsg)
		if err := currentTask.Write(currentTask.Path); err != nil {
			m.err = err
		}
	}
	return m, nil
}

func parsePriorityKey(key string) (int, bool) {
	if key == "0" {
		return 10, true
	}
	value, err := strconv.Atoi(key)
	if err != nil {
		return 0, false
	}
	if value < 0 || value > 9 {
		return 0, false
	}
	return value, true
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

func browseSummaryLine(t *task.Task, idWidth, contentWidth, titleMaxLen int) string {
	if contentWidth <= 0 {
		contentWidth = 1
	}
	idStr := fmt.Sprintf("%*d", idWidth, t.ID)
	if contentWidth <= idWidth+1 {
		idStr = fmt.Sprintf("%d", t.ID)
	}
	stateStr := string(t.State)

	suffixParts := []string{}
	if t.Priority > 0 {
		suffixParts = append(suffixParts, fmt.Sprintf("pri:%d", t.Priority))
	}
	if t.Due != nil {
		suffixParts = append(suffixParts, fmt.Sprintf("due:%s", formatDueDate(t.Due)))
	}
	if len(t.Tags) > 0 {
		suffixParts = append(suffixParts, fmt.Sprintf("{%s}", strings.Join(t.Tags, ",")))
	}

	base := strings.Join([]string{idStr, stateStr}, " ")
	suffix := strings.Join(suffixParts, " ")

	available := contentWidth - len(base)
	if suffix != "" {
		available -= 1 + len(suffix)
	}
	if available < 0 {
		available = 0
	}
	if titleMaxLen > 0 && titleMaxLen < available {
		available = titleMaxLen
	}

	title := ""
	if available > 0 {
		title = truncateWithEllipsis(t.Title, available)
	}

	parts := []string{base}
	if title != "" {
		parts = append(parts, title)
	}
	if suffix != "" {
		parts = append(parts, suffix)
	}
	line := strings.Join(parts, " ")
	if contentWidth > 0 && len(line) > contentWidth {
		line = truncateWithEllipsis(line, contentWidth)
	}
	if strings.TrimSpace(line) == "" {
		line = fmt.Sprintf("%d", t.ID)
	}
	return line
}

func (m model) View() string {
	if len(m.tasks) == 0 {
		return applyLeftMargin("No tasks to display.", m.margin)
	}
	if m.width == 0 {
		return "..."
	}

	var mainContent string
	var help string

	currentTask := m.tasks[m.cursor]
	contentWidth := browseContentWidth(m.width, m.margin)
	mainContent = renderBrowseContent(currentTask, contentWidth)
	debugEnabled := browseDebugEnabled()

	switch m.mode {
	case modeNote:
		help = "enter: save note  esc: cancel\n" + m.textinput.View()
	case modeState:
		help = "t: todo  b: begun  d: done  k: block  c: confirm  n: notdo  esc: cancel"
	default: // modeBrowse
		help = "←/K: prev  →/J/space: next\n" +
			"d:done  b:begun  t:todo  c:confirm  x:notdo  n:note  e:edit  1-0:pri  q:quit"
	}
	if debugEnabled {
		titleLen := len([]rune(currentTask.Title))
		help = fmt.Sprintf(
			"%s\ndebug w=%d h=%d m=%d c=%d title=%d main=%q",
			help,
			m.width,
			m.height,
			m.margin,
			contentWidth,
			titleLen,
			mainContent,
		)
	}

	contentHeight := lipgloss.Height(mainContent)
	helpHeight := lipgloss.Height(help)
	topPad := browseTopPadding(m.height, contentHeight, helpHeight)
	spacerHeight := m.height - topPad - contentHeight - helpHeight
	if spacerHeight < 0 {
		spacerHeight = 0
	}
	spacer := strings.Repeat("\n", spacerHeight)
	if spacer == "" {
		spacer = "\n"
	}
	topSpacer := strings.Repeat("\n", topPad)

	if browsePlainEnabled() {
		return topSpacer + mainContent + spacer + help
	}
	view := contentStyle.Render(mainContent) + spacer + helpStyle.Render(help)
	if topSpacer != "" {
		view = topSpacer + view
	}
	return applyLeftMargin(view, m.margin)
}

func newBrowseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "browse [state]",
		Short:             "Browse tasks in an interactive viewer",
		Long:              "Browse tasks one by one in an interactive full-screen viewer. Keys: \u2190/\u2192 to move, n to add a note, s to set state, e to edit, q to quit.",
		ValidArgsFunction: stateArgCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			if browseSmokeEnabled() {
				fmt.Println("browse smoke ok")
				return nil
			}
			browseMargin = loadBrowseMargin()

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
			if browseNoTuiEnabled() {
				if len(allTasks) == 0 {
					fmt.Println("No tasks found.")
					return nil
				}
				contentWidth := 80
				fmt.Println(renderBrowseContent(allTasks[0], contentWidth))
				return nil
			}
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

func effectiveBrowseMargin(width int) int {
	if width <= 0 {
		return browseMargin
	}
	maxMargin := (width - 5) / 2
	if maxMargin < 0 {
		maxMargin = 0
	}
	if browseMargin > maxMargin {
		return maxMargin
	}
	return browseMargin
}

func browseContentWidth(width, margin int) int {
	contentWidth := width - (margin * 2) - 4
	if contentWidth < 1 {
		return 1
	}
	return contentWidth
}

func browseDebugEnabled() bool {
	return strings.TrimSpace(os.Getenv("PIN_BROWSE_DEBUG")) != ""
}

func browsePlainEnabled() bool {
	return strings.TrimSpace(os.Getenv("PIN_BROWSE_PLAIN")) != ""
}

func browseNoWrapEnabled() bool {
	return strings.TrimSpace(os.Getenv("PIN_BROWSE_NO_WRAP")) != ""
}

func browseNoTuiEnabled() bool {
	return strings.TrimSpace(os.Getenv("PIN_BROWSE_NO_TUI")) != ""
}

func browseSmokeEnabled() bool {
	return strings.TrimSpace(os.Getenv("PIN_BROWSE_SMOKE")) != ""
}

func browseTopPadding(height, contentHeight, helpHeight int) int {
	available := height - contentHeight - helpHeight
	if available <= 0 {
		return 0
	}
	return 1
}

func applyLeftMargin(text string, margin int) string {
	if margin <= 0 {
		return text
	}
	padding := strings.Repeat(" ", margin)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = padding + line
	}
	return strings.Join(lines, "\n")
}

func renderBrowseContent(t *task.Task, width int) string {
	age := formatAge(t.CreatedAt)
	metaLine := fmt.Sprintf("%d %s (%s)", t.ID, t.State, age)
	if len(t.Tags) > 0 {
		metaLine = fmt.Sprintf("%s {%s}", metaLine, strings.Join(t.Tags, ","))
	}
	titleLine := lipgloss.NewStyle().Bold(true).Render(t.Title)

	notes := sectionEntries(t.Body, "## Notes")
	logs := sectionEntries(t.Body, "## Log")
	details := truncateBrowseBlock(browseBodyDetails(t.Title, t.Body))
	notes = truncateBrowseEntries(notes)
	logs = truncateBrowseEntries(logs)

	lines := []string{metaLine, "", titleLine}
	if details != "" {
		lines = append(lines, "", details)
	}
	if len(notes) > 0 {
		lines = append(lines, "", "NOTES", strings.Join(notes, "\n"))
	}
	if len(logs) > 0 {
		lines = append(lines, "", "STATUS LOG", strings.Join(logs, "\n"))
	}
	block := strings.Join(lines, "\n")
	if width <= 0 {
		return truncateBrowseBlock(block)
	}
	if browsePlainEnabled() || browseNoWrapEnabled() || len(block) > browseWrapLimit() {
		return truncateBrowseBlock(block)
	}
	return wrapBrowseText(truncateBrowseBlock(block), width)
}

func stripTitleFromBody(title, body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#") {
		trimmed := strings.TrimSpace(strings.TrimLeft(first, "#"))
		if strings.EqualFold(trimmed, strings.TrimSpace(title)) {
			return strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
	}
	return body
}

func wrapBrowseText(text string, width int) string {
	if width <= 0 {
		return text
	}
	style := lipgloss.NewStyle().Width(width)
	return style.Render(text)
}

func browseWrapLimit() int {
	return 20000
}

func browseMaxContentLen() int {
	return 12000
}

func truncateBrowseBlock(text string) string {
	limit := browseMaxContentLen()
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "\n… (truncated)"
}

func truncateBrowseEntries(entries []string) []string {
	if len(entries) == 0 {
		return entries
	}
	limit := browseMaxContentLen()
	total := 0
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if total+len(entry) > limit {
			out = append(out, "… (truncated)")
			break
		}
		out = append(out, entry)
		total += len(entry) + 1
	}
	return out
}

func formatFriendlyTimestamp(ts time.Time) string {
	abs := ts.Local().Format("Jan 2, 2006 · 3:04pm")
	rel := formatRelativeTime(ts)
	if rel == "" {
		return abs
	}
	return fmt.Sprintf("%s (%s)", abs, rel)
}

func formatRelativeTime(ts time.Time) string {
	now := time.Now()
	diff := now.Sub(ts)
	if diff < 0 {
		diff = -diff
	}

	const minute = time.Minute
	const hour = time.Hour
	const day = 24 * hour
	const week = 7 * day
	const month = 30 * day
	const year = 365 * day

	switch {
	case diff < minute:
		return "just now"
	case diff < hour:
		return pluralizeDuration(int(diff/minute), "minute")
	case diff < day:
		return pluralizeDuration(int(diff/hour), "hour")
	case diff < week:
		return pluralizeDuration(int(diff/day), "day")
	case diff < month:
		return pluralizeDuration(int(diff/week), "week")
	case diff < year:
		return pluralizeDuration(int(diff/month), "month")
	default:
		return pluralizeDuration(int(diff/year), "year")
	}
}

func pluralizeDuration(value int, unit string) string {
	if value == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", value, unit)
}

func sectionEntries(body, heading string) []string {
	_, section, _, found := splitSection(body, heading)
	if !found {
		return nil
	}
	lines := strings.Split(section, "\n")
	entries := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(strings.TrimSpace(line), "##") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		}
		entries = append(entries, formatEntryTimestamp(trimmed))
	}
	return entries
}

func browseBodyDetails(title, body string) string {
	body = strings.TrimSpace(stripTitleFromBody(title, body))
	if body == "" {
		return ""
	}
	beforeNotes, _, afterNotes, notesFound := splitSection(body, "## Notes")
	cleaned := body
	if notesFound {
		cleaned = joinBlocks(beforeNotes, afterNotes)
	}
	beforeLog, _, afterLog, logFound := splitSection(cleaned, "## Log")
	if logFound {
		cleaned = joinBlocks(beforeLog, afterLog)
	}
	return strings.TrimSpace(cleaned)
}

func formatEntryTimestamp(line string) string {
	rest := strings.TrimSpace(line)
	colonIdx := strings.Index(rest, ": ")
	if colonIdx == -1 {
		return line
	}
	rawTs := strings.TrimSpace(rest[:colonIdx])
	parsed, err := time.Parse(time.RFC3339, rawTs)
	if err != nil {
		return line
	}
	chrono := parsed.Local().Format("2006-0102-1504")
	return fmt.Sprintf("%s%s", chrono, rest[colonIdx:])
}

func formatAge(created time.Time) string {
	now := time.Now()
	diff := now.Sub(created)
	if diff < 0 {
		diff = -diff
	}

	days := int(diff / (24 * time.Hour))
	diff -= time.Duration(days) * 24 * time.Hour
	hours := int(diff / time.Hour)
	diff -= time.Duration(hours) * time.Hour
	minutes := int(diff / time.Minute)
	diff -= time.Duration(minutes) * time.Minute
	seconds := int(diff / time.Second)

	switch {
	case days > 0:
		return fmt.Sprintf("%s, %s old", pluralizeAge(days, "day"), pluralizeAge(hours, "hour"))
	case hours > 0:
		return fmt.Sprintf("%s, %s old", pluralizeAge(hours, "hour"), pluralizeAge(minutes, "minute"))
	case minutes > 0:
		return fmt.Sprintf("%s, %s old", pluralizeAge(minutes, "minute"), pluralizeAge(seconds, "second"))
	default:
		if seconds < 1 {
			return "just now"
		}
		return fmt.Sprintf("%s old", pluralizeAge(seconds, "second"))
	}
}

func pluralizeAge(value int, unit string) string {
	if value == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}
