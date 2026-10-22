package usage

import (
	"sync"
	"time"
)

// ScanProfile contains aggregate timings only: no source paths, identifiers or
// transcript content. Nested stages overlap and must not be added as wall time.
type ScanProfile struct {
	mu     sync.Mutex
	stages map[string]ScanStage
}

type ScanStage struct {
	Calls   int64   `json:"calls"`
	TotalMS float64 `json:"total_ms"`
	MaxMS   float64 `json:"max_ms"`
}

func (p *ScanProfile) start(stage string) func() {
	if p == nil {
		return func() {}
	}
	started := time.Now()
	return func() {
		elapsed := float64(time.Since(started).Nanoseconds()) / 1e6
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.stages == nil {
			p.stages = map[string]ScanStage{}
		}
		value := p.stages[stage]
		value.Calls++
		value.TotalMS += elapsed
		value.MaxMS = max(value.MaxMS, elapsed)
		p.stages[stage] = value
	}
}

func (p *ScanProfile) Report() map[string]ScanStage {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]ScanStage, len(p.stages))
	for key, value := range p.stages {
		out[key] = value
	}
	return out
}
