package pathselect

import (
	"context"
	"fmt"
	"sync/atomic"
)

// StubHandle is a test/dev handle.
type StubHandle struct {
	name string
}

func (h *StubHandle) PathName() string { return h.name }

func (h *StubHandle) Close(ctx context.Context) error { return nil }

// StubPath is a controllable Path for unit tests (Phase A).
type StubPath struct {
	name      string
	probeErr  error
	openErr   error
	probeN    atomic.Int64
	openN     atomic.Int64
	failAfter int64 // if >0, probe fails after N successful probes
}

// NewStubPath returns a path that always succeeds unless errors are set.
func NewStubPath(name string) *StubPath {
	return &StubPath{name: name}
}

func (p *StubPath) Name() string { return p.name }

func (p *StubPath) SetProbeErr(err error) { p.probeErr = err }

func (p *StubPath) SetOpenErr(err error) { p.openErr = err }

// FailAfterNProbes makes Probe return error after n successful calls (for health tests).
func (p *StubPath) FailAfterNProbes(n int64) { p.failAfter = n }

func (p *StubPath) Probe(ctx context.Context) error {
	n := p.probeN.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.failAfter > 0 && n > p.failAfter {
		return fmt.Errorf("stub %s: probe forced fail after %d", p.name, p.failAfter)
	}
	return p.probeErr
}

func (p *StubPath) Open(ctx context.Context, req OpenRequest) (Handle, error) {
	p.openN.Add(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.openErr != nil {
		return nil, p.openErr
	}
	return &StubHandle{name: p.name}, nil
}

func (p *StubPath) Close(ctx context.Context) error { return nil }

func (p *StubPath) ProbeCount() int64 { return p.probeN.Load() }

func (p *StubPath) OpenCount() int64 { return p.openN.Load() }
