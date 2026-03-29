package app

import (
	stdcontext "context"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	contextengine "easy-cmd/internal/context"
	"easy-cmd/internal/protocol"
)

func TestModelCtrlCCancelReturnsCancelOutput(t *testing.T) {
	model := New(Dependencies{})
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	typed := next.(Model)
	if typed.Output().Action != protocol.ActionCancel {
		t.Fatalf("expected cancel action, got %q", typed.Output().Action)
	}
}

func TestSubmittingFollowupMarksPreviousCommandGroupStale(t *testing.T) {
	loader := &stubLoader{
		results: []contextengine.RunResult{
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "先看当前目录。",
					Candidates: []protocol.CommandCandidate{
						{Command: "ls -la", Summary: "list", RiskLevel: protocol.RiskLow},
					},
				},
				Activities: []contextengine.Activity{
					{Title: "Ran filesystem.list", Detail: "path=. depth=1"},
				},
			},
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "缩小到日志文件。",
					Candidates: []protocol.CommandCandidate{
						{Command: "find . -name '*.log'", Summary: "logs", RiskLevel: protocol.RiskLow},
					},
				},
			},
		},
	}

	model := New(Dependencies{Loader: loader})
	model = submitPrompt(t, model, "先列目录")
	firstGroupID := model.activeCommandGroupID
	if firstGroupID == "" {
		t.Fatal("expected first command group to become active")
	}
	if len(model.transcript) < 4 {
		t.Fatalf("expected transcript to include user, activity, assistant, and command group entries, got %d", len(model.transcript))
	}

	model = submitPrompt(t, model, "只看日志")
	if model.activeCommandGroupID == firstGroupID {
		t.Fatal("expected a new command group to become active")
	}
	if !commandGroupByID(t, model, firstGroupID).Stale {
		t.Fatal("expected previous command group to be marked stale")
	}
	if commandGroupByID(t, model, model.activeCommandGroupID).Stale {
		t.Fatal("expected newest command group to stay active")
	}
}

func TestSelectingLatestCommandExecutesThatCommand(t *testing.T) {
	loader := &stubLoader{
		results: []contextengine.RunResult{
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "第一轮。",
					Candidates: []protocol.CommandCandidate{
						{Command: "ls -la", Summary: "list", RiskLevel: protocol.RiskLow},
					},
				},
			},
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "第二轮。",
					Candidates: []protocol.CommandCandidate{
						{Command: "find . -name '*.log'", Summary: "logs", RiskLevel: protocol.RiskLow},
					},
				},
			},
		},
	}

	model := New(Dependencies{Loader: loader})
	model = submitPrompt(t, model, "第一轮")
	model = submitPrompt(t, model, "第二轮")

	next, _ := model.Update(tea.KeyMsg{Runes: []rune{'1'}, Type: tea.KeyRunes})
	model = next.(Model)
	if model.Output().Action != protocol.ActionExecute {
		t.Fatalf("expected execute action, got %q", model.Output().Action)
	}
	if model.Output().SelectedCommand != "find . -name '*.log'" {
		t.Fatalf("unexpected selected command: %q", model.Output().SelectedCommand)
	}
}

func TestHighRiskCandidateRequiresInlineConfirmation(t *testing.T) {
	loader := &stubLoader{
		results: []contextengine.RunResult{
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "这个命令会删除文件。",
					Candidates: []protocol.CommandCandidate{
						{Command: "rm -rf build", Summary: "remove build", RiskLevel: protocol.RiskHigh, RequiresConfirmation: true},
					},
				},
			},
		},
	}

	model := New(Dependencies{Loader: loader})
	model = submitPrompt(t, model, "删除构建目录")

	next, _ := model.Update(tea.KeyMsg{Runes: []rune{'1'}, Type: tea.KeyRunes})
	model = next.(Model)
	if model.pendingConfirmation == nil {
		t.Fatal("expected pending confirmation for high-risk command")
	}
	if model.Output().Action != "" {
		t.Fatal("did not expect execution before confirmation")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if model.Output().Action != protocol.ActionExecute {
		t.Fatalf("expected execute action, got %q", model.Output().Action)
	}
}

func TestEscClearsConfirmationAndThenReturnsToConversation(t *testing.T) {
	loader := &stubLoader{
		results: []contextengine.RunResult{
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "危险操作。",
					Candidates: []protocol.CommandCandidate{
						{Command: "rm -rf build", Summary: "remove build", RiskLevel: protocol.RiskHigh, RequiresConfirmation: true},
					},
				},
			},
		},
	}

	model := New(Dependencies{Loader: loader})
	model = submitPrompt(t, model, "删掉构建目录")

	next, _ := model.Update(tea.KeyMsg{Runes: []rune{'1'}, Type: tea.KeyRunes})
	model = next.(Model)
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.pendingConfirmation != nil {
		t.Fatal("expected first esc to clear confirmation")
	}
	if model.Output().Action != "" {
		t.Fatal("expected no final output after clearing confirmation")
	}

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = next.(Model)
	if model.Output().Action != "" {
		t.Fatalf("expected session to stay open, got %q", model.Output().Action)
	}
	if model.status != "告诉 easy-cmd 这次要改成什么。" {
		t.Fatalf("unexpected status after escape: %q", model.status)
	}
}

func TestCommandCardRendersCandidateListInsideSingleCard(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.transcript = []transcriptItem{
		{
			Kind: itemUserMessage,
			Text: "用户输入：展示当前路径",
		},
		{
			Kind: itemCommandGroup,
			CommandGroup: &commandGroup{
				ID: "group-1",
				Candidates: []protocol.CommandCandidate{
					{
						Command:   "pwd",
						Summary:   "显示当前工作目录",
						RiskLevel: protocol.RiskLow,
					},
					{
						Command:   "ls -la",
						Summary:   "列出当前目录下的文件和详细信息",
						RiskLevel: protocol.RiskLow,
					},
					{
						Command:   "find . -maxdepth 1",
						Summary:   "查看当前目录下一层内容",
						RiskLevel: protocol.RiskLow,
					},
				},
			},
		},
	}
	model.activeCommandGroupID = "group-1"

	rendered := model.renderTranscript()
	assertContains(t, rendered, "用户输入：展示当前路径")
	assertContains(t, rendered, "> 1. pwd")
	assertContains(t, rendered, "显示当前工作目录")
	assertContains(t, rendered, "2. ls -la")
	assertContains(t, rendered, "列出当前目录下的文件和详细信息")
	assertContains(t, rendered, "3. find . -maxdepth 1")
	assertContains(t, rendered, "查看当前目录下一层内容")
	assertNotContains(t, rendered, "Would you like to run the following command?")
}

func TestAssistantMessageIsNotRenderedInTranscript(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.transcript = []transcriptItem{
		{Kind: itemUserMessage, Text: "展示当前路径下的文件"},
		{Kind: itemAssistantText, Text: "我可以帮您列出当前路径下的文件。以下是几个常用的命令选项："},
		{
			Kind: itemCommandGroup,
			CommandGroup: &commandGroup{
				ID: "group-1",
				Candidates: []protocol.CommandCandidate{
					{Command: "ls", Summary: "列出当前目录下的文件和文件夹", RiskLevel: protocol.RiskLow},
				},
			},
		},
	}
	model.activeCommandGroupID = "group-1"

	rendered := model.renderTranscript()
	assertContains(t, rendered, "展示当前路径下的文件")
	assertContains(t, rendered, "1. ls")
	assertNotContains(t, rendered, "我可以帮您列出当前路径下的文件。以下是几个常用的命令选项：")
}

func TestPendingConfirmationExpandsInsideSelectedCandidate(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.transcript = []transcriptItem{
		{
			Kind: itemCommandGroup,
			CommandGroup: &commandGroup{
				ID: "group-1",
				Candidates: []protocol.CommandCandidate{
					{
						Command:              "rm -rf build",
						Summary:              "删除构建目录。",
						RiskLevel:            protocol.RiskHigh,
						RequiresConfirmation: true,
					},
				},
			},
		},
	}
	model.activeCommandGroupID = "group-1"
	model.pendingConfirmation = &pendingConfirmation{GroupID: "group-1", CandidateIndex: 0}

	rendered := model.renderTranscript()
	assertContains(t, rendered, "> 1. rm -rf build")
	assertContains(t, rendered, "删除构建目录。")
	assertContains(t, rendered, "按 Enter 确认执行，按 Esc 取消")
}

func TestViewShowsInlineLoadingIndicatorAfterSubmittingPrompt(t *testing.T) {
	loader := &stubLoader{
		results: []contextengine.RunResult{
			{
				Turn: protocol.AssistantTurnEnvelope{
					Type:    "assistant_turn",
					Message: "这里是结果。",
					Candidates: []protocol.CommandCandidate{
						{Command: "ls", Summary: "列出文件", RiskLevel: protocol.RiskLow},
					},
				},
			},
		},
	}

	model := New(Dependencies{Loader: loader})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.input.SetValue("列出当前目录下的文件")

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected async load command")
	}

	view := model.View()
	assertContains(t, view, "列出当前目录下的文件")
	assertContains(t, view, "正在生成")
	assertNotContains(t, view, "Status:")
	assertNotContains(t, view, "1/2/3 choose command")

	model = applyCmdMessages(t, model, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(turnLoadedMsg)
		return ok
	})

	view = model.View()
	assertNotContains(t, view, "正在生成")
	assertNotContains(t, view, "Status:")
	assertNotContains(t, view, "1/2/3 choose command")
}

func TestViewShowsInlineErrorWithoutBottomStatus(t *testing.T) {
	loader := &stubLoader{err: stdcontext.DeadlineExceeded}
	model := New(Dependencies{Loader: loader})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.input.SetValue("列出当前目录下的文件")

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected async load command")
	}

	model = applyCmdMessages(t, model, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(loadErrMsg)
		return ok
	})

	view := model.View()
	assertContains(t, view, "请求失败")
	assertContains(t, view, stdcontext.DeadlineExceeded.Error())
	assertNotContains(t, view, "Status:")
	assertNotContains(t, view, "1/2/3 choose command")
}

func TestViewDoesNotRenderFollowUpLabel(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()

	view := model.View()
	assertNotContains(t, view, "Follow-up")
}

func TestEmptyTranscriptRendersEasyCmdWordmarkAndTwoLineGuide(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()

	rendered := model.renderTranscript()
	assertContains(t, rendered, "███████  █████  ███████")
	assertContains(t, rendered, "描述你想完成的操作")
	assertContains(t, rendered, "像平时说话一样输入，再逐步细化到合适的命令。")
	assertNotContains(t, rendered, "Describe what you want to do")
}

func TestEmptyTranscriptStacksOnNarrowWidth(t *testing.T) {
	model := New(Dependencies{})
	model.width = 70
	model.height = 40
	model.resizeViewport()

	rendered := model.renderTranscript()
	assertContains(t, rendered, "███████  █████  ███████")
	assertContains(t, rendered, "描述你想完成的操作")

	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		if lipgloss.Width(line) > model.viewport.Width {
			t.Fatalf("expected line width <= %d, got %d for %q", model.viewport.Width, lipgloss.Width(line), line)
		}
	}
}

func TestViewportContentUsesCompactSpacingBetweenUserMessageAndCommandGroup(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.transcript = []transcriptItem{
		{Kind: itemUserMessage, Text: "列出当前目录下的文件"},
		{
			Kind: itemCommandGroup,
			CommandGroup: &commandGroup{
				ID: "group-1",
				Candidates: []protocol.CommandCandidate{
					{Command: "ls", Summary: "列出当前目录", RiskLevel: protocol.RiskLow},
				},
			},
		},
	}
	model.activeCommandGroupID = "group-1"

	rendered := model.renderViewportContent()
	lines := strings.Split(rendered, "\n")
	userLine := -1
	commandLine := -1
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "列出当前目录下的文件" {
			userLine = idx
		}
		if trimmed == "> 1. ls" {
			commandLine = idx
			break
		}
	}
	if userLine == -1 || commandLine == -1 {
		t.Fatalf("expected user line and first command line in output, got:\n%s", rendered)
	}
	if commandLine-userLine < 2 {
		t.Fatalf("expected at least one blank line between user input and command list, got:\n%s", rendered)
	}
}

func TestRenderInlineStatusUsesPlainLoadingText(t *testing.T) {
	model := New(Dependencies{})
	model.mode = ModeLoading

	got := model.renderInlineStatus()
	assertContains(t, got, "  正在生成")
	assertNotContains(t, got, "╭")
	assertNotContains(t, got, "╰")
}

func TestRenderInlineStatusAnimatesWithoutBorder(t *testing.T) {
	model := New(Dependencies{})
	model.mode = ModeLoading

	got0 := model.renderInlineStatus()
	model.loadingFrame = 3
	got1 := model.renderInlineStatus()

	if got0 == got1 {
		t.Fatalf("expected loading text to change across frames, got %q", got0)
	}
	assertContains(t, got1, "正在生成...")
	assertNotContains(t, got1, "╭")
	assertNotContains(t, got1, "╰")
}

func TestEmptyTranscriptUsesEnglishWhenConfigured(t *testing.T) {
	model := newModelWithLanguage(t, "en-US")
	model.width = 120
	model.height = 40
	model.resizeViewport()

	rendered := model.renderTranscript()
	assertContains(t, rendered, "Describe what you want to do")
	assertContains(t, rendered, "Type naturally. Refine until a command looks right.")
	assertNotContains(t, rendered, "描述你想完成的操作")
}

func TestPendingConfirmationUsesEnglishWhenConfigured(t *testing.T) {
	model := newModelWithLanguage(t, "en-US")
	model.width = 120
	model.height = 40
	model.resizeViewport()
	model.transcript = []transcriptItem{
		{
			Kind: itemCommandGroup,
			CommandGroup: &commandGroup{
				ID: "group-1",
				Candidates: []protocol.CommandCandidate{
					{
						Command:              "rm -rf build",
						Summary:              "Remove build output.",
						RiskLevel:            protocol.RiskHigh,
						RequiresConfirmation: true,
					},
				},
			},
		},
	}
	model.activeCommandGroupID = "group-1"
	model.pendingConfirmation = &pendingConfirmation{GroupID: "group-1", CandidateIndex: 0}

	rendered := model.renderTranscript()
	assertContains(t, rendered, "Press Enter to confirm, Esc to cancel")
	assertContains(t, rendered, "Choose a number, or move with ↑/↓ then press Enter")
	assertNotContains(t, rendered, "按 Enter 确认执行，按 Esc 取消")
}

func TestResizeViewportAccountsForComposerHeight(t *testing.T) {
	model := New(Dependencies{})
	model.width = 120
	model.height = 40
	model.resizeViewport()

	expected := 40 - model.composerHeight(model.width-4) - 3
	if model.viewport.Height != expected {
		t.Fatalf("expected viewport height %d, got %d", expected, model.viewport.Height)
	}
}

type stubLoader struct {
	results  []contextengine.RunResult
	err      error
	sessions []protocol.SessionContext
}

func (s *stubLoader) Load(_ stdcontext.Context, session protocol.SessionContext) (contextengine.RunResult, error) {
	s.sessions = append(s.sessions, session)
	if s.err != nil {
		return contextengine.RunResult{}, s.err
	}
	if len(s.results) == 0 {
		return contextengine.RunResult{}, nil
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func submitPrompt(t *testing.T, model Model, prompt string) Model {
	t.Helper()
	model.input.SetValue(prompt)
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected async load command")
	}
	return applyCmdMessages(t, model, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(turnLoadedMsg)
		return ok
	})
}

func applyCmdMessages(t *testing.T, model Model, cmd tea.Cmd, stop func(tea.Msg) bool) Model {
	t.Helper()
	for _, msg := range collectCmdMessages(cmd) {
		next, _ := model.Update(msg)
		model = next.(Model)
		if stop != nil && stop(msg) {
			break
		}
	}
	return model
}

func newModelWithLanguage(t *testing.T, language string) Model {
	t.Helper()

	deps := Dependencies{}
	value := reflect.ValueOf(&deps).Elem()
	field := value.FieldByName("Language")
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(language)
	}

	return New(deps)
}

func collectCmdMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, nested := range msg {
			out = append(out, collectCmdMessages(nested)...)
		}
		return out
	case nil:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

func commandGroupByID(t *testing.T, model Model, id string) commandGroup {
	t.Helper()
	for _, item := range model.transcript {
		if item.Kind == itemCommandGroup && item.CommandGroup != nil && item.CommandGroup.ID == id {
			return *item.CommandGroup
		}
	}
	t.Fatalf("command group %q not found", id)
	return commandGroup{}
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected rendered output to contain %q\nfull output:\n%s", want, text)
	}
}

func assertNotContains(t *testing.T, text string, want string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("expected rendered output not to contain %q\nfull output:\n%s", want, text)
	}
}
