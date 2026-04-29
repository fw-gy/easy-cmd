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

	"easy-cmd/internal/core"
	"easy-cmd/internal/i18n"
	"easy-cmd/internal/protocol"
)

type Mode string

const (
	ModeCompose Mode = "compose"
	ModeLoading Mode = "loading"
	ModeError   Mode = "error"
)

// Dependencies 通过注入方式传入，这样 Bubble Tea model 可以专注于
// UI 状态本身，也更容易在测试里替换真实网络依赖。
type Dependencies struct {
	Runner       core.Runner
	BaseRequest  core.Request
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
	Activity     *core.Activity
	CommandGroup *commandGroup
}

type commandGroup struct {
	ID         string
	Candidates []core.Candidate
	Stale      bool
}

type pendingConfirmation struct {
	GroupID        string
	CandidateIndex int
}

type turnLoadedMsg struct {
	result core.Result
}

type loadErrMsg struct {
	err error
}

type loadingTickMsg struct{}

// Model 是完整的 TUI 状态机。它负责维护输入框、已渲染的对话记录、
// 当前激活的命令候选，以及本轮要发给逻辑层的一次性请求参数。
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
	groupSequence        int
	loadingFrame         int
	width                int
	height               int
}

// New 创建初始输入界面，此时对话记录为空，只保留稳定的基础请求参数。
// 当 InitialQuery 非空时，会跳过输入等待阶段，直接进入加载状态。
func New(deps Dependencies) Model {
	catalog := i18n.NewCatalog(deps.Language)
	input := textinput.New()
	input.Placeholder = catalog.Text(i18n.KeyInputPlaceholder)
	input.Focus()
	input.Prompt = "› "

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	m := Model{
		input:    input,
		viewport: vp,
		mode:     ModeCompose,
		status:   catalog.Text(i18n.KeyStatusReady),
		catalog:  catalog,
		deps:     deps,
	}

	if query := strings.TrimSpace(deps.InitialQuery); query != "" {
		m.transcript = append(m.transcript, transcriptItem{
			Kind: itemUserMessage,
			Text: query,
		})
		m.mode = ModeLoading
		m.status = catalog.Text(i18n.KeyStatusAskingModel)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	if query := strings.TrimSpace(m.deps.InitialQuery); query != "" {
		request := m.deps.BaseRequest
		request.Query = query
		return tea.Batch(textinput.Blink, loadTurnCmd(m.deps.Runner, request), loadingTickCmd())
	}
	return textinput.Blink
}

// Update 是 Bubble Tea 的核心 reducer：窗口变化、按键输入、异步加载
// 结果都会从这里流过，并产出下一份 model 状态。
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

// View 负责渲染当前对话内容，以及底部的输入框区域。
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

// handleKey 把键盘输入映射成状态变化，并把全局快捷键和输入框自身的
// 编辑行为区分开来。
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.output = protocol.AppOutput{Action: protocol.ActionCancel}
		return m, tea.Quit
	case tea.KeyEsc:
		if m.pendingConfirmation != nil {
			// 按 ESC 时先取消额外确认状态，而不是直接执行更激进的动作，
			// 比如退出程序。
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
		// 请求还在处理中时，忽略普通输入，避免 UI 状态与后台正在执行的
		// 一次性逻辑请求发生偏离。
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleEnter 在不同状态下有三种含义：
// 1. 确认一个待确认的高风险命令
// 2. 当输入框为空时，执行当前选中的候选命令
// 3. 把新的自然语言请求发送给模型
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.pendingConfirmation != nil && strings.TrimSpace(m.input.Value()) == "" {
		return m.confirmPendingSelection()
	}

	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return m.selectCandidate(m.selectedCandidate)
	}

	m.appendUserMessage(query)
	m.input.SetValue("")
	m.mode = ModeLoading
	m.loadingFrame = 0
	m.status = m.catalog.Text(i18n.KeyStatusAskingModel)
	m.lastError = nil
	m.syncViewport()
	// 并发启动模型请求和加载动画，让 UI 在后台等待时仍然保持响应。
	request := m.deps.BaseRequest
	request.Query = query
	return m, tea.Batch(loadTurnCmd(m.deps.Runner, request), loadingTickCmd())
}

func (m *Model) appendUserMessage(query string) {
	m.transcript = append(m.transcript, transcriptItem{
		Kind: itemUserMessage,
		Text: query,
	})
	m.syncViewport()
}

func (m *Model) applyTurn(result core.Result) {
	// 新一轮 assistant 结果会使上一组命令失效。这样 UI 仍然能保留旧建议，
	// 但不会再允许用户执行它们。
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
		Text: result.Message,
	})

	groupID := m.nextGroupID()
	group := commandGroup{
		ID:         groupID,
		Candidates: result.Candidates,
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

// selectCandidate 会根据候选命令的本地风险等级，决定是直接结束，
// 还是先进入待确认状态。
func (m Model) selectCandidate(index int) (tea.Model, tea.Cmd) {
	group := m.activeCommandGroup()
	if group == nil || group.Stale || index < 0 || index >= len(group.Candidates) {
		return m, nil
	}

	m.selectedCandidate = index
	candidate := group.Candidates[index]
	if candidate.RequiresConfirmation {
		// 高风险命令要求在空输入框上再按一次 Enter，避免用户本来只是想
		// 输入下一句请求，却意外确认执行了命令。
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

// confirmPendingSelection 只有在原先那组命令仍然处于激活状态时才会执行，
// 从而避免新结果到来后，还去确认旧选择。
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

// moveSelection 会把键盘选择限制在当前激活的候选列表范围内，
// 而不是循环跳转。
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

// resizeViewport 会先计算输入框渲染后还剩多少垂直空间，再用它来调整
// 可滚动对话区域的尺寸。
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
	// viewport 总是跟随到最新内容，因为这个 UI 更偏向聊天流体验，
	// 而不是强调手动恢复滚动位置。
	m.viewport.SetContent(m.renderViewportContent())
	m.viewport.GotoBottom()
}

// renderViewportContent 会把对话内容和临时状态行拼成一个整体字符串，
// 交给 Bubble Tea 的 viewport 组件显示。
func (m Model) renderViewportContent() string {
	blocks := []string{m.renderTranscript()}
	if inlineStatus := m.renderInlineStatus(); inlineStatus != "" {
		blocks = append(blocks, inlineStatus)
	}
	return strings.Join(blocks, "\n\n")
}

// renderTranscript 会遍历标准化后的 transcript item，并把每一类内容
// 分发给对应的小渲染函数。
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

// renderEmptyState 用来显示用户还没发送任何请求时的初始欢迎界面。
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

// renderInlineStatus 只负责显示临时状态，这些内容应该出现在对话下方，
// 而不是作为永久对话项写进 transcript。
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

func (m Model) renderActivity(activity core.Activity) string {
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

// renderCommandGroupCard 会把 assistant 返回的候选命令渲染成一张卡片。
// 当旧卡片失效后，会在视觉上被弱化显示。
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

func loadTurnCmd(runner core.Runner, request core.Request) tea.Cmd {
	return func() tea.Msg {
		if runner == nil {
			return loadErrMsg{err: fmt.Errorf("runner is not configured")}
		}
		result, err := runner.Run(stdcontext.Background(), request)
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
