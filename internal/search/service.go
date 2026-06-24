package search

import (
	"sort"
	"strings"
	"sync"

	"opcua-studio/internal/opcua"
)

type MatchKind string

const (
	MatchDisplayName MatchKind = "DisplayName"
	MatchBrowseName  MatchKind = "BrowseName"
	MatchNodeID      MatchKind = "NodeID"
	MatchNodeClass   MatchKind = "NodeClass"
)

type Source string

const (
	SourceAddressSpaceMetadata Source = "AddressSpaceMetadata"
)

type AddressSpaceSearchResult struct {
	Node      opcua.AddressNode `json:"node"`
	MatchKind string            `json:"matchKind"`
	MatchText string            `json:"matchText"`
	Source    string            `json:"source"`
	Score     int               `json:"score"`
}

type AddressSpaceSearchView struct {
	Query   string                     `json:"query"`
	Results []AddressSpaceSearchResult `json:"results"`
	Status  string                     `json:"status"`
}

type Service struct {
	mu    sync.RWMutex
	nodes map[string]opcua.AddressNode
	order []string
}

func NewService() *Service {
	return &Service{nodes: map[string]opcua.AddressNode{}}
}

func (s *Service) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = map[string]opcua.AddressNode{}
	s.order = nil
}

func (s *Service) AddNodes(nodes []opcua.AddressNode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nodes == nil {
		s.nodes = map[string]opcua.AddressNode{}
	}
	for _, node := range nodes {
		if strings.TrimSpace(node.NodeID) == "" {
			continue
		}
		if _, exists := s.nodes[node.NodeID]; exists {
			continue
		}
		s.nodes[node.NodeID] = node
		s.order = append(s.order, node.NodeID)
	}
}

func (s *Service) Search(query string) AddressSpaceSearchView {
	query = strings.TrimSpace(query)
	view := AddressSpaceSearchView{Query: query, Results: []AddressSpaceSearchResult{}, Status: "Search browsed Address Space metadata."}
	if query == "" {
		view.Status = "Enter a search term to search browsed Address Space metadata."
		return view
	}

	s.mu.RLock()
	results := make([]AddressSpaceSearchResult, 0, len(s.nodes))
	for _, nodeID := range s.order {
		node, ok := s.nodes[nodeID]
		if !ok {
			continue
		}
		if result, matched := matchNode(query, node); matched {
			results = append(results, result)
		}
	}
	s.mu.RUnlock()

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		left := strings.ToLower(results[i].Node.DisplayName)
		right := strings.ToLower(results[j].Node.DisplayName)
		if left != right {
			return left < right
		}
		return results[i].Node.NodeID < results[j].Node.NodeID
	})
	view.Results = results
	if len(results) == 0 {
		view.Status = "No matches in browsed Address Space metadata yet. Browse more nodes to expand Search."
	}
	return view
}

func matchNode(query string, node opcua.AddressNode) (AddressSpaceSearchResult, bool) {
	candidates := []struct {
		kind MatchKind
		text string
	}{
		{MatchDisplayName, node.DisplayName},
		{MatchBrowseName, node.BrowseName},
		{MatchNodeClass, node.NodeClass},
		{MatchNodeID, node.NodeID},
	}

	best := AddressSpaceSearchResult{}
	matched := false
	for _, candidate := range candidates {
		if score := scoreMatch(query, candidate.kind, candidate.text); score > 0 && (!matched || score > best.Score) {
			matched = true
			best = AddressSpaceSearchResult{
				Node:      node,
				MatchKind: string(candidate.kind),
				MatchText: candidate.text,
				Source:    string(SourceAddressSpaceMetadata),
				Score:     score,
			}
		}
	}
	return best, matched
}

func scoreMatch(query string, kind MatchKind, text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	q := strings.ToLower(query)
	t := strings.ToLower(text)

	switch kind {
	case MatchDisplayName, MatchBrowseName:
		if t == q {
			return 500
		}
		if strings.HasPrefix(t, q) || hasWordPrefix(t, q) {
			return 400
		}
		if strings.Contains(t, q) {
			return 300
		}
	case MatchNodeClass:
		if t == q {
			return 200
		}
	case MatchNodeID:
		if strings.Contains(t, q) {
			return 100
		}
	}
	return 0
}

func hasWordPrefix(text string, query string) bool {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-' || r == '.' || r == ':' || r == '/'
	})
	for _, field := range fields {
		if strings.HasPrefix(field, query) {
			return true
		}
	}
	return false
}
