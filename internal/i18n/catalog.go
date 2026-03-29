package i18n

import "fmt"

type Language string

const (
	LanguageChinese Language = "zh-CN"
	LanguageEnglish Language = "en-US"
	DefaultLanguage Language = LanguageChinese
)

const (
	KeyInputPlaceholder       = "input_placeholder"
	KeyStatusReady            = "status_ready"
	KeyStatusRequestFailed    = "status_request_failed"
	KeyStatusClearedConfirm   = "status_confirmation_cleared"
	KeyStatusRefineRequest    = "status_refine_request"
	KeyStatusAskingModel      = "status_asking_model"
	KeyStatusRenderedOptions  = "status_rendered_options"
	KeyStatusConfirmHighRisk  = "status_confirm_high_risk"
	KeyEmptyStateTitle        = "empty_state_title"
	KeyEmptyStateSubtitle     = "empty_state_subtitle"
	KeyInlineErrorPrefix      = "inline_error_prefix"
	KeyInlineConfirmSelection = "inline_confirm_selection"
	KeyCommandGroupExpired    = "command_group_expired"
	KeyCommandGroupFooter     = "command_group_footer"
)

type Catalog struct {
	language Language
}

var messages = map[Language]map[string]string{
	LanguageChinese: {
		KeyInputPlaceholder:       "继续补充你的要求，或按 Enter 发送...",
		KeyStatusReady:            "就绪",
		KeyStatusRequestFailed:    "请求失败",
		KeyStatusClearedConfirm:   "已清除确认状态",
		KeyStatusRefineRequest:    "告诉 easy-cmd 这次要改成什么。",
		KeyStatusAskingModel:      "正在请求模型",
		KeyStatusRenderedOptions:  "已渲染 %d 个候选命令",
		KeyStatusConfirmHighRisk:  "请确认高风险命令",
		KeyEmptyStateTitle:        "描述你想完成的操作",
		KeyEmptyStateSubtitle:     "像平时说话一样输入，再逐步细化到合适的命令。",
		KeyInlineErrorPrefix:      "请求失败",
		KeyInlineConfirmSelection: "按 Enter 确认执行，按 Esc 取消",
		KeyCommandGroupExpired:    "这组选项已过期，请参考最新一组候选命令。",
		KeyCommandGroupFooter:     "按数字键选择，或使用 ↑/↓ 移动后按 Enter 确认",
	},
	LanguageEnglish: {
		KeyInputPlaceholder:       "Add more detail, or press Enter to send...",
		KeyStatusReady:            "Ready",
		KeyStatusRequestFailed:    "Request failed",
		KeyStatusClearedConfirm:   "Confirmation cleared",
		KeyStatusRefineRequest:    "Tell easy-cmd what to do differently.",
		KeyStatusAskingModel:      "Asking model",
		KeyStatusRenderedOptions:  "Rendered %d options",
		KeyStatusConfirmHighRisk:  "Confirm high-risk command",
		KeyEmptyStateTitle:        "Describe what you want to do",
		KeyEmptyStateSubtitle:     "Type naturally. Refine until a command looks right.",
		KeyInlineErrorPrefix:      "Request failed",
		KeyInlineConfirmSelection: "Press Enter to confirm, Esc to cancel",
		KeyCommandGroupExpired:    "These options are stale. Refer to the latest command group.",
		KeyCommandGroupFooter:     "Choose a number, or move with ↑/↓ then press Enter",
	},
}

func NewCatalog(raw string) Catalog {
	return Catalog{language: Normalize(raw)}
}

func Normalize(raw string) Language {
	switch Language(raw) {
	case LanguageEnglish:
		return LanguageEnglish
	case LanguageChinese:
		fallthrough
	default:
		return DefaultLanguage
	}
}

func IsSupported(raw string) bool {
	switch Language(raw) {
	case LanguageChinese, LanguageEnglish:
		return true
	default:
		return false
	}
}

func (c Catalog) Text(key string) string {
	lang := c.language
	if lang == "" {
		lang = DefaultLanguage
	}
	if value, ok := messages[lang][key]; ok {
		return value
	}
	if value, ok := messages[DefaultLanguage][key]; ok {
		return value
	}
	return key
}

func (c Catalog) Sprintf(key string, args ...any) string {
	return fmt.Sprintf(c.Text(key), args...)
}

func (c Catalog) LoadingFrames() []string {
	if c.language == LanguageEnglish {
		return []string{
			"Generating",
			"Generating.",
			"Generating..",
			"Generating...",
		}
	}
	return []string{
		"正在生成",
		"正在生成.",
		"正在生成..",
		"正在生成...",
	}
}

func (c Catalog) JoinError(base string, err error) string {
	if err == nil {
		return base
	}
	if c.language == LanguageEnglish {
		return base + ": " + err.Error()
	}
	return base + "：" + err.Error()
}
