package onboarding

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"easy-cmd/internal/config"
	"easy-cmd/internal/i18n"
)

type Result struct {
	Config    config.Config
	Cancelled bool
}

type Model struct {
	inputs           []textinput.Model
	focusIndex       int
	errMsg           string
	lang             i18n.Language
	selected         string
	providerSelected string
	result           Result
}

func New(initial config.Config, reason string) Model {
	lang := i18n.LanguageChinese
	if i18n.IsSupported(initial.Language) {
		lang = i18n.Normalize(initial.Language)
	}

	baseURL := textinput.New()
	baseURL.Placeholder = placeholderFor(lang, "base_url")
	baseURL.Prompt = ""
	baseURL.SetValue(initial.BaseURL)
	baseURL.Focus()

	apiKey := textinput.New()
	apiKey.Placeholder = placeholderFor(lang, "api_key")
	apiKey.Prompt = ""
	apiKey.SetValue(initial.APIKey)
	apiKey.EchoMode = textinput.EchoPassword
	apiKey.EchoCharacter = '•'

	modelName := textinput.New()
	modelName.Placeholder = placeholderFor(lang, "model")
	modelName.Prompt = ""
	modelName.SetValue(initial.Model)

	providerSel := initial.Provider
	if providerSel == "" {
		providerSel = "openai"
	}

	return Model{
		inputs: []textinput.Model{
			baseURL,
			apiKey,
			modelName,
		},
		lang:             lang,
		selected:         initial.Language,
		providerSelected: providerSel,
	}
}

// InitConfig 运行引导流程补全配置并写回默认路径。
func InitConfig(output io.Writer) (config.Config, error) {
	path := config.DefaultPath()
	initial := config.Config{}
	if rawCfg, rawErr := config.ReadRaw(path); rawErr == nil {
		initial = rawCfg
		if !i18n.IsSupported(initial.Language) {
			initial.Language = ""
		}
	}

	cfg, err := Run(initial, "", output)
	if err != nil {
		return config.Config{}, err
	}
	if err := config.Save(path, cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func Run(initial config.Config, reason string, output io.Writer) (config.Config, error) {
	program := tea.NewProgram(New(initial, reason), tea.WithAltScreen(), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return config.Config{}, err
	}

	typed, ok := finalModel.(Model)
	if !ok {
		return config.Config{}, fmt.Errorf("unexpected final onboarding model type %T", finalModel)
	}
	if typed.result.Cancelled {
		return config.Config{}, errors.New(messageFor(typed.lang, "cancelled"))
	}
	return typed.result.Config, nil
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.result = Result{Cancelled: true}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.focusIndex == len(m.inputs)+1 {
				next, err := m.submit()
				if err != nil {
					return next, nil
				}
				return next, tea.Quit
			}
			m.setFocus(m.focusIndex + 1)
			return m, nil
		case tea.KeyTab, tea.KeyDown:
			m.setFocus((m.focusIndex + 1) % (len(m.inputs) + 2))
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.setFocus((m.focusIndex - 1 + len(m.inputs) + 2) % (len(m.inputs) + 2))
			return m, nil
		case tea.KeyLeft:
			if m.focusIndex == len(m.inputs) {
				m.selectProvider(-1)
				return m, nil
			}
			if m.focusIndex == len(m.inputs)+1 {
				m.selectLanguage(-1)
				return m, nil
			}
		case tea.KeyRight:
			if m.focusIndex == len(m.inputs) {
				m.selectProvider(1)
				return m, nil
			}
			if m.focusIndex == len(m.inputs)+1 {
				m.selectLanguage(1)
				return m, nil
			}
		case tea.KeyRunes:
			if m.focusIndex == len(m.inputs) {
				switch strings.ToLower(string(msg.Runes)) {
				case "o":
					m.providerSelected = "openai"
					return m, nil
				case "a":
					m.providerSelected = "anthropic"
					return m, nil
				case "g":
					m.providerSelected = "gemini"
					return m, nil
				}
			}
			if m.focusIndex == len(m.inputs)+1 {
				switch strings.ToLower(string(msg.Runes)) {
				case "z":
					m.selected = string(i18n.LanguageChinese)
					m.lang = i18n.LanguageChinese
					return m, nil
				case "e":
					m.selected = string(i18n.LanguageEnglish)
					m.lang = i18n.LanguageEnglish
					return m, nil
				}
			}
		case tea.KeySpace:
			if m.focusIndex == len(m.inputs) {
				m.selectProvider(1)
				return m, nil
			}
			if m.focusIndex == len(m.inputs)+1 {
				m.selectLanguage(1)
				return m, nil
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEF3FF"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#B7C0D6"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB4B4")).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA3B5"))
	focusLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9FE5FF")).
		Bold(true)
	lines := []string{
		titleStyle.Render(messageFor(m.lang, "title")),
		hintStyle.Render(messageFor(m.lang, "subtitle")),
	}
	if strings.TrimSpace(m.errMsg) != "" {
		lines = append(lines, "", errorStyle.Render(m.errMsg))
	}

	labels := []string{
		messageFor(m.lang, "base_url"),
		messageFor(m.lang, "api_key"),
		messageFor(m.lang, "model"),
	}
	lines = append(lines, "")
	for i, input := range m.inputs {
		currentLabelStyle := labelStyle
		renderedInput := input.View()
		if i == m.focusIndex {
			currentLabelStyle = focusLabelStyle
		}
		lines = append(lines, currentLabelStyle.Render(labels[i]))
		lines = append(lines, renderedInput)
		lines = append(lines, "")
	}
	currentProviderStyle := labelStyle
	if m.focusIndex == len(m.inputs) {
		currentProviderStyle = focusLabelStyle
	}
	lines = append(lines, currentProviderStyle.Render(messageFor(m.lang, "provider")))
	lines = append(lines, m.renderProviderOptions())
	lines = append(lines, "")

	currentLabelStyle := labelStyle
	if m.focusIndex == len(m.inputs)+1 {
		currentLabelStyle = focusLabelStyle
	}
	lines = append(lines, currentLabelStyle.Render(messageFor(m.lang, "language")))
	lines = append(lines, m.renderLanguageOptions())
	lines = append(lines, "")
	lines = append(lines, hintStyle.Render(messageFor(m.lang, "footer")))

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8EAF1")).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func (m Model) submit() (Model, error) {
	cfg := config.Config{
		BaseURL:  strings.TrimSpace(m.inputs[0].Value()),
		APIKey:   strings.TrimSpace(m.inputs[1].Value()),
		Model:    strings.TrimSpace(m.inputs[2].Value()),
		Language: strings.TrimSpace(m.selected),
		Provider: strings.TrimSpace(m.providerSelected),
	}

	missing := make([]string, 0, 4)
	if strings.TrimSpace(cfg.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		missing = append(missing, "api_key")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		missing = append(missing, "model")
	}
	if strings.TrimSpace(cfg.Language) == "" {
		missing = append(missing, "language")
	}
	if len(missing) > 0 {
		m.errMsg = validationMessage(m.lang, missing)
		return m, errors.New(m.errMsg)
	}
	if !i18n.IsSupported(cfg.Language) {
		m.errMsg = fmt.Sprintf(messageFor(m.lang, "invalid_language"), cfg.Language)
		return m, errors.New(m.errMsg)
	}

	m.errMsg = ""
	m.result = Result{Config: cfg}
	return m, nil
}

func (m *Model) setFocus(index int) {
	m.focusIndex = index
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func placeholderFor(lang i18n.Language, field string) string {
	switch field {
	case "base_url":
		if lang == i18n.LanguageEnglish {
			return "Enter API endpoint"
		}
		return "输入接口地址"
	case "api_key":
		if lang == i18n.LanguageEnglish {
			return "Paste your API key"
		}
		return "填入你的 API Key"
	case "model":
		if lang == i18n.LanguageEnglish {
			return "Enter model name"
		}
		return "输入模型名称"
	default:
		return ""
	}
}

func validationMessage(lang i18n.Language, missing []string) string {
	if lang == i18n.LanguageEnglish {
		return "Required fields: " + strings.Join(missing, ", ")
	}
	return "必填项不能为空：" + strings.Join(missing, "、")
}

func (m *Model) selectLanguage(delta int) {
	options := []string{string(i18n.LanguageChinese), string(i18n.LanguageEnglish)}
	if len(options) == 0 {
		return
	}
	index := -1
	for i, option := range options {
		if option == m.selected {
			index = i
			break
		}
	}
	if index == -1 {
		if delta < 0 {
			index = len(options) - 1
		} else {
			index = 0
		}
	} else {
		index = (index + delta + len(options)) % len(options)
	}
	m.selected = options[index]
	m.lang = i18n.Normalize(m.selected)
}

func (m Model) renderProviderOptions() string {
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("#D5DAE8"))
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1F202B")).
		Background(lipgloss.Color("#9FE5FF")).
		Bold(true).
		Padding(0, 1)

	options := []struct{ value, label string }{
		{"openai", "OpenAI"},
		{"anthropic", "Anthropic"},
		{"gemini", "Gemini"},
	}

	var parts []string
	for _, opt := range options {
		if m.providerSelected == opt.value {
			parts = append(parts, selected.Render(opt.label))
		} else {
			parts = append(parts, normal.Render(opt.label))
		}
	}
	return strings.Join(parts, "  ")
}

func (m *Model) selectProvider(delta int) {
	options := []string{"openai", "anthropic", "gemini"}
	index := -1
	for i, opt := range options {
		if opt == m.providerSelected {
			index = i
			break
		}
	}
	if index == -1 {
		index = 0
	} else {
		index = (index + delta + len(options)) % len(options)
	}
	m.providerSelected = options[index]
}

func (m Model) renderLanguageOptions() string {
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("#D5DAE8"))
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1F202B")).
		Background(lipgloss.Color("#9FE5FF")).
		Bold(true).
		Padding(0, 1)

	zh := normal.Render("中文")
	en := normal.Render("English")
	if m.selected == string(i18n.LanguageChinese) {
		zh = selected.Render("中文")
	}
	if m.selected == string(i18n.LanguageEnglish) {
		en = selected.Render("English")
	}
	return zh + "  " + en
}

func messageFor(lang i18n.Language, key string) string {
	if lang == i18n.LanguageEnglish {
		switch key {
		case "title":
			return "Configure easy-cmd"
		case "subtitle":
			return "Set up your runtime config. Press Enter to move forward, Esc to cancel."
		case "base_url":
			return "Base URL"
		case "api_key":
			return "API Key"
		case "model":
			return "Model"
		case "language":
			return "Language"
		case "provider":
			return "Provider"
		case "footer":
			return "Use ←/→ to choose a language. Enter saves on the language row. Tab or ↑/↓ moves focus."
		case "invalid_language":
			return "Unsupported language: %s"
		case "cancelled":
			return "configuration cancelled"
		}
	}

	switch key {
	case "title":
		return "配置 easy-cmd"
	case "subtitle":
		return "请完成运行配置。Enter 前进，最后一项再次按 Enter 保存，Esc 取消。"
	case "base_url":
		return "Base URL"
	case "api_key":
		return "API Key"
	case "model":
		return "Model"
	case "language":
		return "Language"
	case "provider":
		return "Provider"
	case "footer":
		return "在语言这一项用 ←/→ 选择。定位到语言后按 Enter 保存。Tab 或 ↑/↓ 可切换焦点。"
	case "invalid_language":
		return "不支持的语言：%s"
	case "cancelled":
		return "configuration cancelled"
	default:
		return key
	}
}
