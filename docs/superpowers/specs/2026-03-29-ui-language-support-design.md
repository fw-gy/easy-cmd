# UI Language Support Design

## Summary

Add configurable UI language support so users can choose the display language in `config.json`. The first release supports `zh-CN` and `en-US`, while keeping the implementation structured for future languages.

## Goals

- Add a `language` field to the config file.
- Default to Chinese when `language` is omitted.
- Render user-facing UI and CLI strings in the configured language.
- Centralize translated copy so additional languages can be added without rewriting app logic.

## Non-Goals

- Translating provider names, protocol fields, or other internal identifiers.
- Translating model-generated free-form text returned by the AI provider.
- Loading translations from external files in this release.

## Design

### Config

Extend `internal/config.Config` with a `Language` field using the JSON key `language`.

Rules:

- Missing `language` defaults to `zh-CN`.
- Accepted values are `zh-CN` and `en-US`.
- Any other value returns a validation error during config load.

This keeps the selection explicit and predictable while reserving a stable config surface for future languages.

### Translation Catalog

Add a small internal i18n package that defines:

- a `Language` type
- supported language constants
- a `Catalog` struct or equivalent lookup object
- one centralized map of message keys to translated strings

The app will request translated strings by key instead of hard-coding English or Chinese text inside UI logic. Unknown keys should fall back to the key itself or a safe default so failures stay visible during development.

### Wiring

`cmd/easy-cmd/main.go` will build the catalog from the loaded config and pass it into `internal/app`.

`internal/app/model.go` will use the catalog for:

- input placeholder
- empty-state title and subtitle
- loading and error badges
- status line text
- inline confirmation prompt
- command-group footer and stale-group hint

`cmd/easy-cmd/main.go` will also localize direct CLI-facing errors that are shown before the TUI starts, including subcommand usage and unsupported shell messages where practical.

### Testing

Add tests for:

- config defaulting to `zh-CN`
- config rejecting unsupported language values
- app rendering English empty-state and command-card copy when `en-US` is selected
- app preserving existing Chinese copy when `zh-CN` is selected

## Risks

- If strings remain embedded in the UI code, the feature becomes inconsistent. The implementation should move all user-facing copy behind the catalog in the touched paths.
- AI responses remain untranslated by design. This can produce mixed-language sessions, which is acceptable for the first release because the request is scoped to displayed chrome rather than model output rewriting.
