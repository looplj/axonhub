package decorator

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
)

func RequestDecorator(name string, decorator func(ctx context.Context, request *llm.Request) (*llm.Request, error)) Decorator {
	return &funcDecorator{
		name:             name,
		requestDecorator: decorator,
	}
}

func ResponseDecorator(name string, decorator func(ctx context.Context, response *llm.Response) (*llm.Response, error)) Decorator {
	return &funcDecorator{
		name:              name,
		responseDecorator: decorator,
	}
}

func FuncDecorator(
	name string,
	requestDecorator func(ctx context.Context, request *llm.Request) (*llm.Request, error),
	responseDecorator func(ctx context.Context, response *llm.Response) (*llm.Response, error),
) Decorator {
	return &funcDecorator{
		name:              name,
		requestDecorator:  requestDecorator,
		responseDecorator: responseDecorator,
	}
}

type funcDecorator struct {
	name              string
	requestDecorator  func(ctx context.Context, request *llm.Request) (*llm.Request, error)
	responseDecorator func(ctx context.Context, response *llm.Response) (*llm.Response, error)
}

func (d *funcDecorator) Name() string {
	return d.name
}

func (d *funcDecorator) DecorateRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if d.requestDecorator == nil {
		return request, nil
	}

	return d.requestDecorator(ctx, request)
}

func (d *funcDecorator) DecorateResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	if d.responseDecorator == nil {
		return response, nil
	}

	return d.responseDecorator(ctx, response)
}
