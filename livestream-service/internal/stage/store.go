package stage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Store persists Stage records.  The in-memory implementation is the default;
// a DynamoDB-backed one can be added without changing the Service.
type Store interface {
	Put(ctx context.Context, s *Stage) error
	Get(ctx context.Context, stageID string) (*Stage, error)
	Update(ctx context.Context, s *Stage) error
	Delete(ctx context.Context, stageID string) error
	ListByHost(ctx context.Context, hostID string) ([]*Stage, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// In-memory Store (default; no AWS dependency)
// ──────────────────────────────────────────────────────────────────────────────

type memStore struct {
	mu     sync.RWMutex
	stages map[string]*Stage // keyed by StageID
}

func NewMemStore() Store {
	return &memStore{stages: make(map[string]*Stage)}
}

func (m *memStore) Put(_ context.Context, s *Stage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *s
	m.stages[s.StageID] = &cp
	return nil
}

func (m *memStore) Get(_ context.Context, stageID string) (*Stage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.stages[stageID]
	if !ok {
		return nil, fmt.Errorf("stage_not_found")
	}
	cp := *s
	return &cp, nil
}

func (m *memStore) Update(_ context.Context, s *Stage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stages[s.StageID]; !ok {
		return fmt.Errorf("stage_not_found")
	}
	cp := *s
	m.stages[s.StageID] = &cp
	return nil
}

func (m *memStore) Delete(_ context.Context, stageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.stages[stageID]; !ok {
		return fmt.Errorf("stage_not_found")
	}
	delete(m.stages, stageID)
	return nil
}

func (m *memStore) ListByHost(_ context.Context, hostID string) ([]*Stage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Stage
	for _, s := range m.stages {
		if s.HostID == hostID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────────────────────────────────────

func ptrTime(t time.Time) *time.Time { return &t }
