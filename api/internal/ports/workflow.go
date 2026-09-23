package ports

import (
	"context"
	"time"
)

// WorkflowEvent contains routing and lifecycle metadata only, never answers, signatures or clinical text.
type WorkflowEvent struct {
	ID         string
	Collection string
	EntityID   string
	Before     map[string]string
	After      map[string]string
	CreatedAt  time.Time
}
type WorkflowEventRepository interface {
	Pending(context.Context, int) ([]WorkflowEvent, error)
	Complete(context.Context, string) error
	Failed(context.Context, string, string) error
}
