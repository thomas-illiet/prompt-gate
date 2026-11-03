package health

import (
	"context"
	"sync"
	"time"
)

type contextKey struct{}
type Progress struct {
	mu        sync.Mutex
	deadlines map[string]time.Time
	now       func() time.Time
}

func NewProgress() *Progress { return &Progress{deadlines: make(map[string]time.Time), now: time.Now} }
func WithProgress(ctx context.Context, p *Progress) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func Beat(ctx context.Context, name string, budget time.Duration) {
	if p, ok := ctx.Value(contextKey{}).(*Progress); ok {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.deadlines[name] = p.now().Add(budget)
	}
}

// Run observes real job execution rather than a separate timer that could keep
// reporting healthy while a worker is stuck. Idle periods have their own budget.
func Run(ctx context.Context, name string, interval time.Duration, run func()) {
	if ctx.Err() != nil {
		return
	}
	Beat(ctx, name, 10*time.Minute)
	defer Beat(ctx, name, interval+time.Minute)
	run()
}

func (p *Progress) Healthy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, deadline := range p.deadlines {
		if !p.now().Before(deadline) {
			return false
		}
	}
	return true
}
