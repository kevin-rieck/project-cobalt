package search

import (
	"context"
	"strings"
	"sync"
	"time"

	"opcua-studio/internal/opcua"
)

const (
	defaultBrowseInterval = time.Second
	defaultBrowseBudget   = 250
	objectsRootNodeID     = "i=85"
)

// Browser is the browse-only OPC UA seam needed by Address Space Search.
type Browser interface {
	BrowseChildren(ctx context.Context, nodeID string) ([]opcua.AddressNode, error)
}

// SessionOptions configures Shallow Address Space Indexing for one connected session.
// Zero values use conservative production defaults.
type SessionOptions struct {
	BrowseInterval    time.Duration
	BrowseBudget      int
	ReportBrowseError func(nodeID string, err error)
}

// Session owns browsing, indexing, and Address Space Search state for one connected OPC UA Server.
type Session struct {
	mu sync.RWMutex

	browser         Browser
	ctx             context.Context
	cancel          context.CancelFunc
	index           *Service
	prioritize      chan []opcua.AddressNode
	browseInterval  time.Duration
	browseBudget    int
	reportError     func(string, error)
	indexingActive  bool
	budgetExhausted bool
	stopped         bool
}

// NewSession starts a per-connection Address Space Search session.
func NewSession(base context.Context, browser Browser, configured ...SessionOptions) *Session {
	if base == nil {
		base = context.Background()
	}
	options := SessionOptions{}
	if len(configured) > 0 {
		options = configured[0]
	}
	if options.BrowseInterval <= 0 {
		options.BrowseInterval = defaultBrowseInterval
	}
	if options.BrowseBudget <= 0 {
		options.BrowseBudget = defaultBrowseBudget
	}

	ctx, cancel := context.WithCancel(base)
	index := NewService()
	index.AddNodes([]opcua.AddressNode{{NodeID: objectsRootNodeID, DisplayName: "Objects", BrowseName: "Objects", NodeClass: "Object"}})
	session := &Session{
		browser:        browser,
		ctx:            ctx,
		cancel:         cancel,
		index:          index,
		prioritize:     make(chan []opcua.AddressNode, 128),
		browseInterval: options.BrowseInterval,
		browseBudget:   options.BrowseBudget,
		reportError:    options.ReportBrowseError,
		indexingActive: true,
	}
	go session.runShallowIndexing()
	return session
}

// Stop ends the connected-server search session. It is safe to call more than once.
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

// BrowseChildren explicitly browses through the session, ingests returned metadata,
// and prioritizes discovered parent nodes for shallow indexing.
func (s *Session) BrowseChildren(nodeID string) ([]opcua.AddressNode, error) {
	children, err := s.browseAndIndex(nodeID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	indexingActive := s.indexingActive && !s.stopped
	s.mu.RUnlock()
	if indexingActive {
		select {
		case s.prioritize <- children:
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		default:
		}
	}
	return children, nil
}

func (s *Session) browseAndIndex(nodeID string) ([]opcua.AddressNode, error) {
	if strings.TrimSpace(nodeID) == "" {
		nodeID = objectsRootNodeID
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

func (s *Session) runShallowIndexing() {
	budgetExhausted := false
	defer func() {
		s.mu.Lock()
		if !s.stopped {
			s.indexingActive = false
			s.budgetExhausted = budgetExhausted
		}
		s.mu.Unlock()
	}()

	priorityQueue := []string{}
	backgroundQueue := []string{objectsRootNodeID}
	seen := map[string]bool{objectsRootNodeID: true}
	firstBrowse := true
	browseCount := 0

	for {
		drainPriorityRequests(s.prioritize, &priorityQueue, seen)
		if len(priorityQueue) == 0 && len(backgroundQueue) == 0 {
			select {
			case <-s.ctx.Done():
				return
			case nodes := <-s.prioritize:
				enqueueParentNodes(nodes, &priorityQueue, seen)
				continue
			}
		}

		if !firstBrowse {
			timer := time.NewTimer(s.browseInterval)
			waiting := true
			for waiting {
				select {
				case <-s.ctx.Done():
					timer.Stop()
					return
				case nodes := <-s.prioritize:
					enqueueParentNodes(nodes, &priorityQueue, seen)
				case <-timer.C:
					waiting = false
				}
			}
		}
		firstBrowse = false
		drainPriorityRequests(s.prioritize, &priorityQueue, seen)

		if browseCount >= s.browseBudget {
			budgetExhausted = true
			return
		}
		fromPriority := len(priorityQueue) > 0
		var nodeID string
		if fromPriority {
			nodeID = priorityQueue[0]
			priorityQueue = priorityQueue[1:]
		} else {
			nodeID = backgroundQueue[0]
			backgroundQueue = backgroundQueue[1:]
		}

		browseCount++
		children, err := s.browseAndIndex(nodeID)
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			if s.reportError != nil {
				s.reportError(nodeID, err)
			}
		} else if fromPriority {
			enqueueParentNodes(children, &priorityQueue, seen)
		} else {
			enqueueParentNodes(children, &backgroundQueue, seen)
		}

		if browseCount >= s.browseBudget && (len(priorityQueue) > 0 || len(backgroundQueue) > 0) {
			budgetExhausted = true
			return
		}
	}
}

func drainPriorityRequests(prioritize <-chan []opcua.AddressNode, queue *[]string, seen map[string]bool) {
	for {
		select {
		case nodes := <-prioritize:
			enqueueParentNodes(nodes, queue, seen)
		default:
			return
		}
	}
}

func enqueueParentNodes(nodes []opcua.AddressNode, queue *[]string, seen map[string]bool) {
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" || seen[nodeID] || !isParentNode(node) {
			continue
		}
		seen[nodeID] = true
		*queue = append(*queue, nodeID)
	}
}

func isParentNode(node opcua.AddressNode) bool {
	return node.NodeClass == "Object" || node.NodeClass == "View"
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
