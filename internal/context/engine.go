package context

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"fmt"

	"easy-cmd/internal/protocol"
)

type Provider interface {
	Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error)
}

type ProviderFunc func(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error)

func (f ProviderFunc) Run(ctx stdcontext.Context, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	return f(ctx, session, raw)
}

type Registry struct {
	providers map[string]Provider
}

// Register 把一个 provider 实现注册到指定名字下，模型之后会在
// JSON context_request 里用这个名字来调用它。
func (r *Registry) Register(name string, provider Provider) {
	if r.providers == nil {
		r.providers = make(map[string]Provider)
	}
	r.providers[name] = provider
}

// Run 按名字把一次上下文请求分发给对应的 provider。
func (r Registry) Run(ctx stdcontext.Context, name string, session protocol.SessionContext, raw json.RawMessage) (any, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return provider.Run(ctx, session, raw)
}

type Client interface {
	NextResponse(ctx stdcontext.Context, session protocol.SessionContext) ([]byte, error)
}

type Options struct {
	MaxRounds           int
	MaxRequestsPerRound int
}

// Activity 是面向 UI 的 provider 调用摘要，用来告诉用户模型在背后
// 额外请求了哪些上下文。
type Activity struct {
	Title  string
	Detail string
}

// RunResult 包含最终 assistant_turn，以及在补齐上下文过程中收集到的
// provider 活动记录。
type RunResult struct {
	Turn       protocol.AssistantTurnEnvelope
	Activities []Activity
}

type Engine struct {
	registry Registry
	client   Client
	options  Options
}

// NewEngine 为递归式上下文收集设置默认安全限制。
func NewEngine(registry Registry, client Client, options Options) *Engine {
	if options.MaxRounds <= 0 {
		options.MaxRounds = 3
	}
	if options.MaxRequestsPerRound <= 0 {
		options.MaxRequestsPerRound = 3
	}
	return &Engine{registry: registry, client: client, options: options}
}

// Run 负责驱动一次完整的 assistant 回合。模型在真正返回命令候选前，
// 可能会先多次请求额外上下文。
func (e *Engine) Run(ctx stdcontext.Context, session protocol.SessionContext) (RunResult, error) {
	rounds := 0
	result := RunResult{}
	for {
		// 模型每一轮要么直接结束并返回结果，要么继续请求只读上下文。
		// 这里会持续把新拿到的上下文回填进 session，直到收到最终结果，
		// 或触发预设的安全限制。
		raw, err := e.client.NextResponse(ctx, session)
		if err != nil {
			return RunResult{}, err
		}

		envelope, err := protocol.ParseEnvelope(raw)
		if err != nil {
			return RunResult{}, err
		}

		switch msg := envelope.(type) {
		case protocol.AssistantTurnEnvelope:
			result.Turn = msg
			return result, nil
		case protocol.ContextRequestEnvelope:
			// 这两个限制都是为了防止模型失控循环，或者单轮响应里发起过多
			// 上下文请求。
			if rounds >= e.options.MaxRounds {
				return RunResult{}, fmt.Errorf("maximum context rounds exceeded (%d)", e.options.MaxRounds)
			}
			if len(msg.Requests) > e.options.MaxRequestsPerRound {
				return RunResult{}, fmt.Errorf("context request count %d exceeds limit %d", len(msg.Requests), e.options.MaxRequestsPerRound)
			}

			results := make([]protocol.ContextResult, 0, len(msg.Requests))
			for _, request := range msg.Requests {
				data, err := e.registry.Run(ctx, request.Provider, session, request.Args)
				if err != nil {
					results = append(results, protocol.ContextResult{
						ID:       request.ID,
						Provider: request.Provider,
						OK:       false,
						Error:    err.Error(),
					})
					result.Activities = append(result.Activities, Activity{
						Title:  fmt.Sprintf("Failed %s", request.Provider),
						Detail: err.Error(),
					})
					continue
				}
				results = append(results, protocol.ContextResult{
					ID:       request.ID,
					Provider: request.Provider,
					OK:       true,
					Data:     data,
				})
				result.Activities = append(result.Activities, Activity{
					Title:  fmt.Sprintf("Ran %s", request.Provider),
					Detail: request.Reason,
				})
			}

			// 把请求和结果都保存在 session 里，下一轮模型调用就能知道哪些
			// 信息已经取过，从而避免重复请求。
			session.RequestHistory = append(session.RequestHistory, msg.Requests...)
			session.CollectedContext = append(session.CollectedContext, results...)
			rounds++
		default:
			return RunResult{}, errors.New("unexpected envelope type")
		}
	}
}
