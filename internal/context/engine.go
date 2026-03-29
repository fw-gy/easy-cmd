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

func (r *Registry) Register(name string, provider Provider) {
	if r.providers == nil {
		r.providers = make(map[string]Provider)
	}
	r.providers[name] = provider
}

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

type Activity struct {
	Title  string
	Detail string
}

type RunResult struct {
	Turn       protocol.AssistantTurnEnvelope
	Activities []Activity
}

type Engine struct {
	registry Registry
	client   Client
	options  Options
}

func NewEngine(registry Registry, client Client, options Options) *Engine {
	if options.MaxRounds <= 0 {
		options.MaxRounds = 3
	}
	if options.MaxRequestsPerRound <= 0 {
		options.MaxRequestsPerRound = 3
	}
	return &Engine{registry: registry, client: client, options: options}
}

func (e *Engine) Run(ctx stdcontext.Context, session protocol.SessionContext) (RunResult, error) {
	rounds := 0
	result := RunResult{}
	for {
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
					return RunResult{}, fmt.Errorf("provider %s: %w", request.Provider, err)
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

			session.RequestHistory = append(session.RequestHistory, msg.Requests...)
			session.CollectedContext = append(session.CollectedContext, results...)
			rounds++
		default:
			return RunResult{}, errors.New("unexpected envelope type")
		}
	}
}
