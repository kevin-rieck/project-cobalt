package search

import (
	"context"
	"strings"
	"sync"

	"opcua-studio/internal/opcua"
)

// Browser is the browse-only OPC UA seam needed by Address Space Search.
type Browser interface {
	BrowseChildren(ctx context.Context, nodeID string) ([]opcua.AddressNode, error)
}

// Session owns Address Space Search state for one connected OPC UA Server.
type Session struct {
	mu sync.RWMutex

	browser         Browser
	ctx             context.Context
	cancel          context.CancelFunc
	index           *Service
	indexingActive  bool
	budgetExhausted bool
	stopped         bool
}

func NewSession(base context.Context, browser Browser) *Session {
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	index := NewService()
	index.AddNodes([]opcua.AddressNode{{NodeID: "i=85", DisplayName: "Objects", BrowseName: "Objects", NodeClass: "Object"}})
	return &Session{
		browser:        browser,
		ctx:            ctx,
		cancel:         cancel,
		index:          index,
		indexingActive: true,
	}
}

// Stop ends the connected-server search session and rejects further browsing.
func (s *Session) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.indexingActive = false
	s.cancel()
	s.mu.Unlock()
}

// BrowseChildren browses through the session and ingests returned metadata.
func (s *Session) BrowseChildren(nodeID string) ([]opcua.AddressNode, error) {
	if strings.TrimSpace(nodeID) == "" {
		nodeID = "i=85"
	}

	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		return nil, context.Canceled
	}
	ctx := s.ctx
	browser := s.browser
	s.mu.RUnlock()

	children, err := browser.BrowseChildren(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.index.AddNodes(children)
	return children, nil
}

// FinishShallowIndexing records the coverage state used in search status text.
func (s *Session) FinishShallowIndexing(budgetExhausted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.indexingActive = false
	s.budgetExhausted = budgetExhausted
}

// Search scores the session's indexed metadata and describes its current coverage.
func (s *Session) Search(query string) AddressSpaceSearchView {
	view := s.index.Search(query)
	view.Status = strings.ReplaceAll(view.Status, "browsed Address Space metadata", "browsed and shallow-indexed Address Space metadata")

	s.mu.RLock()
	indexingActive := s.indexingActive
	budgetExhausted := s.budgetExhausted
	s.mu.RUnlock()
	if indexingActive {
		view.Status += " Shallow Address Space Indexing is active; indexed coverage is still expanding."
	} else if budgetExhausted {
		view.Status += " Shallow Address Space Indexing reached its session budget; some Address Space areas may not be indexed."
	}
	return view
}
