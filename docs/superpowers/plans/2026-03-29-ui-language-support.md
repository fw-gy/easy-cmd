# UI Language Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add configurable Chinese and English UI language support selected through `config.json`.

**Architecture:** Introduce a small internal i18n catalog, validate the configured language in `internal/config`, and inject the catalog into the Bubble Tea model so all user-facing chrome strings come from one place. Keep the catalog in code for this release to minimize moving parts while leaving a clear extension point for new languages.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, standard `testing` package

---

### File Map

- Create: `internal/i18n/catalog.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/model.go`
- Modify: `internal/app/model_test.go`
- Modify: `cmd/easy-cmd/main.go`
- Modify: `examples/config.json`
- Modify: `README.md`

### Task 1: Add Config Tests First

**Files:**
- Modify: `internal/config/config_test.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test for default language**

```go
func TestLoadAppliesDefaultLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret"}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Language != "zh-CN" {
		t.Fatalf("expected default language zh-CN, got %q", cfg.Language)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadAppliesDefaultLanguage`
Expected: FAIL because `Config` has no `Language` field or the field is empty.

- [ ] **Step 3: Write the failing test for unsupported language**

```go
func TestLoadRejectsUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"base_url":"https://api.example.com/v1","api_key":"secret","language":"fr-FR"}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}
```

- [ ] **Step 4: Run test to verify it fails for the right reason**

Run: `go test ./internal/config -run TestLoadRejectsUnsupportedLanguage`
Expected: FAIL because the config loader currently accepts any extra JSON field.

### Task 2: Add App Rendering Tests First

**Files:**
- Modify: `internal/app/model_test.go`
- Test: `internal/app/model_test.go`

- [ ] **Step 1: Write the failing test for English empty-state copy**

```go
func TestEmptyStateUsesEnglishCatalog(t *testing.T) {
	model := New(Dependencies{Catalog: i18n.NewCatalog(i18n.LanguageEnglish)})
	model.width = 120
	model.height = 40
	model.resizeViewport()

	rendered := model.renderTranscript()
	assertContains(t, rendered, "Describe what you want to do")
	assertContains(t, rendered, "Type naturally. Refine until a command looks right.")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestEmptyStateUsesEnglishCatalog`
Expected: FAIL because dependencies do not yet accept a catalog and the model always uses embedded copy.

- [ ] **Step 3: Write the failing test for English command-card copy**

```go
func TestPendingConfirmationUsesEnglishCatalog(t *testing.T) {
	model := New(Dependencies{Catalog: i18n.NewCatalog(i18n.LanguageEnglish)})
	// build a group with a pending confirmation
	assertContains(t, rendered, "Press Enter to confirm, Esc to cancel")
	assertContains(t, rendered, "Choose a number, or move with ↑/↓ then press Enter")
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/app -run 'TestEmptyStateUsesEnglishCatalog|TestPendingConfirmationUsesEnglishCatalog'`
Expected: FAIL because English rendering is not implemented.

### Task 3: Implement Minimal Language Support

**Files:**
- Create: `internal/i18n/catalog.go`
- Modify: `internal/config/config.go`
- Modify: `internal/app/model.go`
- Modify: `cmd/easy-cmd/main.go`

- [ ] **Step 1: Add supported language definitions and catalog lookup**

```go
type Language string

const (
	LanguageChinese Language = "zh-CN"
	LanguageEnglish Language = "en-US"
)

type Catalog struct {
	language Language
}

func NewCatalog(language Language) Catalog { ... }
func (c Catalog) Text(key string) string { ... }
```

- [ ] **Step 2: Implement config validation and defaulting**

```go
type Config struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Language string `json:"language"`
}
```

Logic:

- default empty `Language` to `zh-CN`
- reject anything outside `zh-CN` and `en-US`

- [ ] **Step 3: Inject the catalog into the app model**

```go
type Dependencies struct {
	Loader       Loader
	BaseSession  protocol.SessionContext
	InitialQuery string
	Catalog      i18n.Catalog
}
```

- [ ] **Step 4: Replace embedded UI copy with catalog lookups**

Use catalog keys for placeholder, statuses, empty state, confirmation copy, footer copy, and stale-copy.

- [ ] **Step 5: Build the catalog in main**

```go
catalog := i18n.NewCatalog(i18n.Language(cfg.Language))
model := app.New(app.Dependencies{
	Loader: loader{engine: engine},
	Catalog: catalog,
	...
})
```

### Task 4: Verify Green and Update Docs

**Files:**
- Modify: `examples/config.json`
- Modify: `README.md`

- [ ] **Step 1: Run targeted tests**

Run: `go test ./internal/config ./internal/app ./cmd/easy-cmd`
Expected: PASS

- [ ] **Step 2: Update user-facing docs**

Add `"language": "zh-CN"` to `examples/config.json` and document accepted values plus default behavior in `README.md`.

- [ ] **Step 3: Run full test suite**

Run: `make test`
Expected: PASS

### Self-Review

- Spec coverage: config selection, centralized catalog, UI wiring, and tests are each represented by a dedicated task.
- Placeholder scan: no `TODO` or deferred implementation markers remain.
- Type consistency: `Language`, `Catalog`, and `Dependencies.Catalog` use the same names across tasks.
