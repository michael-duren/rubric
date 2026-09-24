package initialize

import (
	"context"

	"github.com/michael-duren/go-skills/internal/plan"
	"github.com/michael-duren/go-skills/internal/write"
)

// Apply writes a prepared plan for req's target once every conflict has a decision.
func Apply(ctx context.Context, req Request, p plan.Plan) (write.Result, error) {
	if err := ctx.Err(); err != nil {
		return write.Result{}, err
	}
	if len(plan.Conflicts(p)) > 0 {
		return write.Result{}, &ConflictError{Plan: p}
	}
	return write.Apply(ctx, target(req), p)
}

func target(req Request) string {
	if req.Target == "" {
		return "."
	}
	return req.Target
}
