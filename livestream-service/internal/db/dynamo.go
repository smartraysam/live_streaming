package db

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Store is a light repository abstraction; in-memory maps are used as fallback during local development.
type Store struct {
	StreamsTable string
	ChatTable    string
	TicketsTable string

	mu      sync.RWMutex
	streams map[string]map[string]interface{}
	chat    map[string][]map[string]interface{}
	tickets map[string]map[string]time.Time
}

func NewStore(streamsTable, chatTable, ticketsTable string) *Store {
	return &Store{
		StreamsTable: streamsTable,
		ChatTable:    chatTable,
		TicketsTable: ticketsTable,
		streams:      map[string]map[string]interface{}{},
		chat:         map[string][]map[string]interface{}{},
		tickets:      map[string]map[string]time.Time{},
	}
}

func (s *Store) PutStream(ctx context.Context, id string, item map[string]interface{}) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streams[id] = item
	return nil
}

func (s *Store) GetStream(ctx context.Context, id string) (map[string]interface{}, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.streams[id]
	if !ok {
		return nil, fmt.Errorf("stream not found")
	}
	return item, nil
}

func (s *Store) DeleteStream(ctx context.Context, id string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.streams, id)
	return nil
}

func (s *Store) ListStreams(ctx context.Context) ([]map[string]interface{}, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(s.streams))
	for _, v := range s.streams {
		out = append(out, v)
	}
	return out, nil
}

func (s *Store) PutChatMessage(ctx context.Context, streamID string, msg map[string]interface{}) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chat[streamID] = append(s.chat[streamID], msg)
	return nil
}

func (s *Store) ChatHistory(ctx context.Context, streamID string, limit int) ([]map[string]interface{}, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.chat[streamID]
	if len(msgs) <= limit {
		return append([]map[string]interface{}(nil), msgs...), nil
	}
	return append([]map[string]interface{}(nil), msgs[len(msgs)-limit:]...), nil
}

func (s *Store) GrantTicket(ctx context.Context, streamID, userID string, expiresAt time.Time) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tickets[streamID]; !ok {
		s.tickets[streamID] = map[string]time.Time{}
	}
	s.tickets[streamID][userID] = expiresAt
	return nil
}

func (s *Store) HasTicket(ctx context.Context, streamID, userID string) bool {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.tickets[streamID][userID]
	return ok && exp.After(time.Now())
}
