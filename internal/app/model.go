package app

import (
	stdcontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/i18n"
	"easy-cmd/internal/protocol"
)

type Mode string

const (
	ModeCompose Mode = "compose"
	ModeLoading Mode = "loading"
	ModeError   Mode = "error"
)

type Loader interface {
	Load(ctx stdcontext.Context, session protocol.SessionContext) (contextengine.RunResult, error)
}

type Dependencies struct {
	Loader       Loader
	BaseSession  protocol.SessionContext
	InitialQuery string
	Language     string
}

type transcriptItemKind string

const (
	itemUserMessage    transcriptItemKind = "user_message"
	itemAssistantText  transcriptItemKind = "assistant_message"
	itemContextActvity transcriptItemKind = "context_activity"
	itemCommandGroup   transcriptItemKind = "command_group"
)

type transcriptItem struct {
	Kind         transcriptItemKind
	Text         string
	Activity     *contextengine.Activity
	CommandGroup *commandGroup
}

type commandGroup struct {
	ID         string
	Candidates []protocol.CommandCandidate
	Stale      bool
}

type pendingConfirmation struct {
	GroupID        string
	CandidateIndex int
}

type turnLoadedMsg struct {
	result contextengine.RunResult
}

type loadErrMsg struct {
	err error
}

type loadingTickMsg struct{}

type Model struct {
	input                textinput.Model
	viewport             viewport.Model
	mode                 Mode
	transcript           []transcriptItem
	activeCommandGroupID string
	selectedCandidate    int
	pendingConfirmation  *pendingConfirmation
	status               string
	output               protocol.AppOutput
	lastError            error
	catalog              i18n.Catalog
	deps                 Dependencies
	session              protocol.SessionContext
	groupSequence        int
	loadingFrame         int
	width                int
	height               int
}

func New(deps Dependencies) Model {
	catalog := i18n.NewCatalog(deps.Language)
	input := textinput.New()
	input.Placeholder = catalog.Text(i18n.KeyInputPlaceholder)
	input.Focus()
	input.SetValue(deps.InitialQuery)
	input.Prompt = "› "

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return Model{
		input:    input,
		viewport: vp,
		mode:     ModeCompose,
		status:   catalog.Text(i18n.KeyStatusReady),
		catalog:  catalog,
		deps:     deps,
		session:  deps.BaseSession,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		return m, nil
	case turnLoadedMsg:
		m.applyTurn(msg.result)
		return m, nil
	case loadErrMsg:
		m.mode = ModeError
		m.lastError = msg.err
		m.status = m.catalog.Text(i18n.KeyStatusRequestFailed)
		m.syncViewport()
		return m, nil
	case loadingTickMsg:
		if m.mode != ModeLoading {
			return m, nil
		}
		m.loadingFrame = (m.loadingFrame + 1) % 4
		m.syncViewport()
		return m, loadingTickCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m Model) View() string {
	background := lipgloss.NewStyle().Background(lipgloss.Color("#1F202B")).Foreground(lipgloss.Color("#E8EAF1"))
	frame := lipgloss.NewStyle().Padding(1, 2)
	composerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3C4158")).
		Background(lipgloss.Color("#262A38")).
		Padding(1, 1)
	transcript := m.viewport.View()
	composer := composerStyle.Width(max(40, m.width-6)).Render(m.input.View())

	content := lipgloss.JoinVertical(lipgloss.Left, transcript, "", composer)
	return background.Render(frame.Render(content))
}

func (m Model) Output() protocol.AppOutput {
	return m.output
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.output = protocol.AppOutput{Action: protocol.ActionCancel}
		return m, tea.Quit
	case tea.KeyEsc:
		if m.pendingConfirmation != nil {
			m.pendingConfirmation = nil
			m.status = m.catalog.Text(i18n.KeyStatusClearedConfirm)
			m.syncViewport()
			return m, nil
		}
		if m.activeCommandGroup() != nil {
			m.status = m.catalog.Text(i18n.KeyStatusRefineRequest)
			m.syncViewport()
			return m, nil
		}
		m.output = protocol.AppOutput{Action: protocol.ActionCancel}
		return m, tea.Quit
	case tea.KeyUp:
		m.moveSelection(-1)
		return m, nil
	case tea.KeyDown:
		m.moveSelection(1)
		return m, nil
	case tea.KeyPgUp:
		m.viewport.LineUp(6)
		return m, nil
	case tea.KeyPgDown:
		m.viewport.LineDown(6)
		return m, nil
	case tea.KeyEnter:
		return m.handleEnter()
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && m.input.Value() == "" {
			switch msg.Runes[0] {
			case '1':
				return m.selectCandidate(0)
			case '2':
				return m.selectCandidate(1)
			case '3':
				return m.selectCandidate(2)
			case 'y', 'Y':
				return m.selectCandidate(m.selectedCandidate)
			}
		}
	}

	if m.mode == ModeLoading {
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.pendingConfirmation != nil && strings.TrimSpace(m.input.Value()) == "" {
		return m.confirmPendingSelection()
	}

	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return m.selectCandidate(m.selectedCandidate)
	}

	m.appendUserMessage(query)
	m.session.UserQuery = query
	m.session.Conversation = append(m.session.Conversation, protocol.ConversationMessage{
		Role:    "user",
		Content: query,
	})
	m.input.SetValue("")
	m.mode = ModeLoading
	m.loadingFrame = 0
	m.status = m.catalog.Text(i18n.KeyStatusAskingModel)
	m.lastError = nil
	m.syncViewport()
	return m, tea.Batch(loadTurnCmd(m.deps.Loader, m.session), loadingTickCmd())
}

func (m *Model) appendUserMessage(query string) {
	m.transcript = append(m.transcript, transcriptItem{
		Kind: itemUserMessage,
		Text: query,
	})
	m.syncViewport()
}

func (m *Model) applyTurn(result contextengine.RunResult) {
	m.markActiveGroupStale()
	for _, activity := range result.Activities {
		activityCopy := activity
		m.transcript = append(m.transcript, transcriptItem{
			Kind:     itemContextActvity,
			Activity: &activityCopy,
		})
	}

	m.transcript = append(m.transcript, transcriptItem{
		Kind: itemAssistantText,
		Text: result.Turn.Message,
	})
	m.session.Conversation = append(m.session.Conversation, protocol.ConversationMessage{
		Role:    "assistant",
		Content: result.Turn.Message,
	})

	groupID := m.nextGroupID()
	group := commandGroup{
		ID:         groupID,
		Candidates: result.Turn.Candidates,
	}
	m.transcript = append(m.transcript, transcriptItem{
		Kind:         itemCommandGroup,
		CommandGroup: &group,
	})

	m.activeCommandGroupID = groupID
	m.selectedCandidate = 0
	m.pendingConfirmation = nil
	m.mode = ModeCompose
	m.loadingFrame = 0
	m.status = m.catalog.Sprintf(i18n.KeyStatusRenderedOptions, len(group.Candidates))
	m.syncViewport()
}

func (m *Model) markActiveGroupStale() {
	if m.activeCommandGroupID == "" {
		return
	}
	for idx := range m.transcript {
		item := &m.transcript[idx]
		if item.Kind == itemCommandGroup && item.CommandGroup != nil && item.CommandGroup.ID == m.activeCommandGroupID {
			item.CommandGroup.Stale = true
			break
		}
	}
	m.activeCommandGroupID = ""
	m.pendingConfirmation = nil
}

func (m Model) selectCandidate(index int) (tea.Model, tea.Cmd) {
	group := m.activeCommandGroup()
	if group == nil || group.Stale || index < 0 || index >= len(group.Candidates) {
		return m, nil
	}

	m.selectedCandidate = index
	candidate := group.Candidates[index]
	if candidate.RequiresConfirmation {
		m.pendingConfirmation = &pendingConfirmation{
			GroupID:        group.ID,
			CandidateIndex: index,
		}
		m.status = m.catalog.Text(i18n.KeyStatusConfirmHighRisk)
		m.syncViewport()
		return m, nil
	}

	return m.finishExecution(candidate.Command), tea.Quit
}

func (m Model) confirmPendingSelection() (tea.Model, tea.Cmd) {
	group := m.activeCommandGroup()
	if group == nil || m.pendingConfirmation == nil {
		return m, nil
	}
	index := m.pendingConfirmation.CandidateIndex
	if index < 0 || index >= len(group.Candidates) {
		return m, nil
	}

	return m.finishExecution(group.Candidates[index].Command), tea.Quit
}

func (m *Model) moveSelection(delta int) {
	group := m.activeCommandGroup()
	if group == nil || len(group.Candidates) == 0 {
		return
	}
	m.selectedCandidate += delta
	if m.selectedCandidate < 0 {
		m.selectedCandidate = 0
	}
	if m.selectedCandidate >= len(group.Candidates) {
		m.selectedCandidate = len(group.Candidates) - 1
	}
	m.syncViewport()
}

func (m *Model) resizeViewport() {
	transcriptWidth := m.width - 4
	if transcriptWidth < 40 {
		transcriptWidth = 40
	}
	transcriptHeight := m.height - m.composerHeight(transcriptWidth) - 3
	if transcriptHeight < 3 {
		transcriptHeight = 3
	}
	m.viewport.Width = transcriptWidth
	m.viewport.Height = transcriptHeight
	m.syncViewport()
}

func (m *Model) syncViewport() {
	if m.viewport.Width == 0 {
		return
	}
	m.viewport.SetContent(m.renderViewportContent())
	m.viewport.GotoBottom()
}

func (m Model) renderViewportContent() string {
	blocks := []string{m.renderTranscript()}
	if inlineStatus := m.renderInlineStatus(); inlineStatus != "" {
		blocks = append(blocks, inlineStatus)
	}
	return strings.Join(blocks, "\n\n")
}

func (m Model) renderTranscript() string {
	if len(m.transcript) == 0 {
		return m.renderEmptyState()
	}

	var blocks []string
	for _, item := range m.transcript {
		switch item.Kind {
		case itemUserMessage:
			blocks = append(blocks, m.renderUserMessage(item.Text))
		case itemContextActvity:
			blocks = append(blocks, m.renderActivity(*item.Activity))
		case itemCommandGroup:
			blocks = append(blocks, m.renderCommandGroup(*item.CommandGroup))
		}
	}
	return strings.Join(blocks, "\n\n")
}

func (m Model) renderEmptyState() string {
	wordmarkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EEF3FF")).
		Bold(true)
	wordmark := strings.Join([]string{
		"███████  █████  ███████ ██    ██        ██████ ███    ███ ██████ ",
		"██      ██   ██ ██       ██  ██        ██      ████  ████ ██   ██",
		"█████   ███████ ███████   ████         ██      ██ ████ ██ ██   ██",
		"██      ██   ██      ██    ██          ██      ██  ██  ██ ██   ██",
		"███████ ██   ██ ███████    ██           ██████ ██      ██ ██████ ",
	}, "\n")

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F3F5FB")).
		Bold(true).
		Render(m.catalog.Text(i18n.KeyEmptyStateTitle))
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A9B2C7")).
		MaxWidth(max(24, m.viewport.Width-4)).
		Render(m.catalog.Text(i18n.KeyEmptyStateSubtitle))
	copyBlock := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)

	renderedWordmark := wordmarkStyle.Render(wordmark)
	wordmarkWidth := lipgloss.Width(renderedWordmark)
	copyWidth := lipgloss.Width(copyBlock)

	if wordmarkWidth+3+copyWidth <= m.viewport.Width {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderedWordmark,
			"   ",
			copyBlock,
		)
	}

	stackedCopy := lipgloss.NewStyle().
		MarginTop(1).
		Render(copyBlock)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		renderedWordmark,
		stackedCopy,
	)
}

func (m Model) renderInlineStatus() string {
	switch m.mode {
	case ModeLoading:
		frames := m.catalog.LoadingFrames()
		return m.renderInlineStatusBadge(frames[m.loadingFrame%len(frames)], lipgloss.Color("#B9C6D9"))
	case ModeError:
		message := m.catalog.Text(i18n.KeyInlineErrorPrefix)
		message = m.catalog.JoinError(message, m.lastError)
		return m.renderInlineStatusBadge(message, lipgloss.Color("#FFB4B4"))
	default:
		return ""
	}
}

func (m Model) renderInlineStatusBadge(text string, foreground lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(foreground).
		Faint(true).
		PaddingLeft(2).
		Render(text)
}

func (m Model) composerHeight(width int) int {
	composerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3C4158")).
		Background(lipgloss.Color("#262A38")).
		Padding(1, 1)
	return lipgloss.Height(composerStyle.Width(max(40, width-2)).Render(m.input.View()))
}

func (m Model) renderUserMessage(text string) string {
	outer := lipgloss.NewStyle().
		Background(lipgloss.Color("#323448")).
		Foreground(lipgloss.Color("#F2F4FA")).
		Bold(true).
		Padding(0, 2).
		Width(max(40, m.viewport.Width-4))
	return outer.Render(text)
}

func (m Model) renderActivity(activity contextengine.Activity) string {
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9BE38F")).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA3B5"))
	lines := []string{
		titleStyle.Render("• " + activity.Title),
		detailStyle.Render("  " + activity.Detail),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCommandGroup(group commandGroup) string {
	return m.renderCommandGroupCard(group)
}

func (m Model) renderCommandGroupCard(group commandGroup) string {
	background := lipgloss.Color("#38384A")
	foreground := lipgloss.Color("#E7EAF3")
	if group.Stale {
		background = lipgloss.Color("#2A2C39")
		foreground = lipgloss.Color("#8A90A6")
	}

	card := lipgloss.NewStyle().
		Background(background).
		Foreground(foreground).
		Padding(0, 2).
		Width(max(40, m.viewport.Width-8))

	commandStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAF0FF")).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D5DAE8"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9FE5FF")).Bold(true)
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#959CB2"))
	confirmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB4B4")).Bold(true)
	expiredStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0B8"))

	lines := make([]string, 0, len(group.Candidates)*3+2)
	for idx, candidate := range group.Candidates {
		prefix := "  "
		style := commandStyle
		if group.ID == m.activeCommandGroupID && idx == m.selectedCandidate && !group.Stale {
			prefix = "> "
			style = selectedStyle
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%d. %s", prefix, idx+1, candidate.Command)))
		lines = append(lines, detailStyle.Render("   "+candidate.Summary))
		if m.pendingConfirmation != nil &&
			m.pendingConfirmation.GroupID == group.ID &&
			m.pendingConfirmation.CandidateIndex == idx {
			lines = append(lines, confirmStyle.Render("   "+m.catalog.Text(i18n.KeyInlineConfirmSelection)))
		}
		if idx < len(group.Candidates)-1 {
			lines = append(lines, "")
		}
	}
	if group.Stale {
		lines = append(lines, "", expiredStyle.Render(m.catalog.Text(i18n.KeyCommandGroupExpired)))
	} else {
		lines = append(lines, "", footerStyle.Render(m.catalog.Text(i18n.KeyCommandGroupFooter)))
	}
	return card.Render(strings.Join(lines, "\n"))
}

func (m Model) activeCommandGroup() *commandGroup {
	for idx := range m.transcript {
		item := &m.transcript[idx]
		if item.Kind == itemCommandGroup && item.CommandGroup != nil && item.CommandGroup.ID == m.activeCommandGroupID {
			return item.CommandGroup
		}
	}
	return nil
}

func (m *Model) nextGroupID() string {
	m.groupSequence++
	return fmt.Sprintf("group-%d", m.groupSequence)
}

func (m Model) riskColor(level protocol.RiskLevel) lipgloss.Color {
	switch level {
	case protocol.RiskHigh:
		return lipgloss.Color("#FF7C7C")
	case protocol.RiskMedium:
		return lipgloss.Color("#FFD580")
	default:
		return lipgloss.Color("#8EE59A")
	}
}

func (m Model) finishExecution(command string) Model {
	m.output = protocol.AppOutput{
		Action:          protocol.ActionExecute,
		SelectedCommand: command,
	}
	return m
}

func loadTurnCmd(loader Loader, session protocol.SessionContext) tea.Cmd {
	return func() tea.Msg {
		if loader == nil {
			return loadErrMsg{err: fmt.Errorf("loader is not configured")}
		}
		result, err := loader.Load(stdcontext.Background(), session)
		if err != nil {
			return loadErrMsg{err: err}
		}
		return turnLoadedMsg{result: result}
	}
}

func loadingTickCmd() tea.Cmd {
	return tea.Tick(180*time.Millisecond, func(time.Time) tea.Msg {
		return loadingTickMsg{}
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
