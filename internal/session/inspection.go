package session

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"opcua-studio/internal/opcua"
)

// RequestKind describes side effects that Variable Node Inspection needs the caller to perform.
type RequestKind int

const (
	RequestSubscribeValue RequestKind = iota
	RequestReadDetails
	RequestWaitValue
	RequestCancelSubscription
)

const sessionTrendPointLimit = 500

// Request is a side-effect request emitted by InspectionSet.
type Request struct {
	Kind         RequestKind
	NodeID       string
	Updates      <-chan opcua.LiveValue
	Subscription opcua.ValueSubscription
}

// VariableNodeInspection is the Troubleshooting Session state for one Variable Node.
type VariableNodeInspection struct {
	Node           opcua.AddressNode
	Updates        <-chan opcua.LiveValue
	Subscription   opcua.ValueSubscription
	Value          opcua.LiveValue
	Details        opcua.NodeDetails
	Subscribing    bool
	LoadingDetails bool
	Watched        bool
	Err            error
	DetailsErr     error
	Stale          bool
	OutOfRange     string
	UpdateCount    int
}

// SessionTrendPoint is one Live Value update observed during a Troubleshooting Session.
type SessionTrendPoint struct {
	Value           string `json:"value"`
	Status          string `json:"status"`
	Timestamp       string `json:"timestamp"`
	SourceTimestamp string `json:"sourceTimestamp"`
	ServerTimestamp string `json:"serverTimestamp"`
	ReceivedAt      string `json:"receivedAt"`
}

// SessionTrendNode summarizes an Observed Variable Node available in Session Trend.
type SessionTrendNode struct {
	Node        opcua.AddressNode `json:"node"`
	LatestValue string            `json:"latestValue"`
	Status      string            `json:"status"`
	PointCount  int               `json:"pointCount"`
}

// SessionTrendView contains the Observed Variable Nodes and newest-first points for one node.
type SessionTrendView struct {
	Nodes  []SessionTrendNode  `json:"nodes"`
	Points []SessionTrendPoint `json:"points"`
}

// InspectionSet owns selected and watched Variable Node Inspections.
type InspectionSet struct {
	selectedNodeID string
	inspections    map[string]*VariableNodeInspection
	watchOrder     []string
	trendOrder     []string
	trendNodes     map[string]opcua.AddressNode
	trends         map[string][]SessionTrendPoint
}

func NewInspectionSet() *InspectionSet {
	return &InspectionSet{inspections: map[string]*VariableNodeInspection{}, trendNodes: map[string]opcua.AddressNode{}, trends: map[string][]SessionTrendPoint{}}
}

func (s *InspectionSet) Select(node opcua.AddressNode) []Request {
	if node.NodeClass != "Variable" {
		return s.Unselect()
	}

	previous := s.selectedNodeID
	s.selectedNodeID = node.NodeID
	inspection := s.ensure(node)
	requests := s.startRequests(inspection)
	if previous != "" && previous != node.NodeID {
		requests = append(requests, s.cleanupIfUnreferenced(previous)...)
	}
	return requests
}

func (s *InspectionSet) Unselect() []Request {
	previous := s.selectedNodeID
	s.selectedNodeID = ""
	if previous == "" {
		return nil
	}
	return s.cleanupIfUnreferenced(previous)
}

func (s *InspectionSet) Watch(node opcua.AddressNode) []Request {
	if node.NodeClass != "Variable" {
		return nil
	}
	inspection := s.ensure(node)
	if !inspection.Watched {
		inspection.Watched = true
		s.watchOrder = append(s.watchOrder, node.NodeID)
	}
	return s.startRequests(inspection)
}

func (s *InspectionSet) Unwatch(nodeID string) []Request {
	inspection, ok := s.inspections[nodeID]
	if !ok {
		return nil
	}
	inspection.Watched = false
	s.removeWatchOrder(nodeID)
	return s.cleanupIfUnreferenced(nodeID)
}

func (s *InspectionSet) ApplySubscription(nodeID string, updates <-chan opcua.LiveValue, subscription opcua.ValueSubscription, err error) []Request {
	inspection, ok := s.inspections[nodeID]
	if !ok {
		if subscription != nil {
			return []Request{{Kind: RequestCancelSubscription, NodeID: nodeID, Subscription: subscription}}
		}
		return nil
	}
	inspection.Subscribing = false
	if err != nil {
		inspection.Err = err
		return nil
	}
	inspection.Updates = updates
	inspection.Subscription = subscription
	inspection.Err = nil
	inspection.Stale = false
	if updates == nil {
		return nil
	}
	return []Request{{Kind: RequestWaitValue, NodeID: nodeID, Updates: updates}}
}

func (s *InspectionSet) ApplyLiveValue(nodeID string, value opcua.LiveValue, err error) []Request {
	inspection, ok := s.inspections[nodeID]
	if !ok {
		return nil
	}
	if err != nil {
		inspection.Err = err
		inspection.Stale = true
		return nil
	}
	inspection.Value = value
	inspection.Err = nil
	inspection.Stale = false
	inspection.OutOfRange = outOfRangeText(value.Value, inspection.Details.EURange)
	inspection.UpdateCount++
	s.appendTrendPoint(inspection.Node, value)
	if inspection.Updates == nil {
		return nil
	}
	return []Request{{Kind: RequestWaitValue, NodeID: nodeID, Updates: inspection.Updates}}
}

func (s *InspectionSet) ApplyDetails(nodeID string, details opcua.NodeDetails, err error) {
	inspection, ok := s.inspections[nodeID]
	if !ok {
		return
	}
	inspection.LoadingDetails = false
	if err != nil {
		inspection.DetailsErr = err
		return
	}
	inspection.Details = details
	inspection.OutOfRange = outOfRangeText(inspection.Value.Value, details.EURange)
	inspection.DetailsErr = nil
}

func (s *InspectionSet) Selected() (VariableNodeInspection, bool) {
	if s.selectedNodeID == "" {
		return VariableNodeInspection{}, false
	}
	inspection, ok := s.inspections[s.selectedNodeID]
	if !ok {
		return VariableNodeInspection{}, false
	}
	return *inspection, true
}

func (s *InspectionSet) Watched() []VariableNodeInspection {
	watched := make([]VariableNodeInspection, 0, len(s.watchOrder))
	for _, nodeID := range s.watchOrder {
		inspection, ok := s.inspections[nodeID]
		if ok && inspection.Watched {
			watched = append(watched, *inspection)
		}
	}
	return watched
}

func (s *InspectionSet) IsWatched(nodeID string) bool {
	inspection, ok := s.inspections[nodeID]
	return ok && inspection.Watched
}

func (s *InspectionSet) Inspection(nodeID string) (VariableNodeInspection, bool) {
	inspection, ok := s.inspections[nodeID]
	if !ok {
		return VariableNodeInspection{}, false
	}
	return *inspection, true
}

func (s *InspectionSet) SessionTrend(focusedNodeID string) SessionTrendView {
	view := SessionTrendView{Nodes: make([]SessionTrendNode, 0, len(s.trendOrder))}
	for _, nodeID := range s.trendOrder {
		points := s.trends[nodeID]
		if len(points) == 0 {
			continue
		}
		node, ok := s.trendNodes[nodeID]
		if !ok {
			continue
		}
		latest := points[len(points)-1]
		view.Nodes = append(view.Nodes, SessionTrendNode{Node: node, LatestValue: latest.Value, Status: latest.Status, PointCount: len(points)})
	}
	if focusedNodeID == "" && len(view.Nodes) > 0 {
		focusedNodeID = view.Nodes[0].Node.NodeID
	}
	points := s.trends[focusedNodeID]
	view.Points = make([]SessionTrendPoint, 0, len(points))
	for i := len(points) - 1; i >= 0; i-- {
		view.Points = append(view.Points, points[i])
	}
	return view
}

func (s *InspectionSet) ensure(node opcua.AddressNode) *VariableNodeInspection {
	if s.inspections == nil {
		s.inspections = map[string]*VariableNodeInspection{}
	}
	inspection, ok := s.inspections[node.NodeID]
	if !ok {
		inspection = &VariableNodeInspection{Node: node}
		s.inspections[node.NodeID] = inspection
		return inspection
	}
	inspection.Node = node
	return inspection
}

func (s *InspectionSet) startRequests(inspection *VariableNodeInspection) []Request {
	var requests []Request
	if inspection.Subscription == nil && !inspection.Subscribing && inspection.Err == nil {
		inspection.Subscribing = true
		requests = append(requests, Request{Kind: RequestSubscribeValue, NodeID: inspection.Node.NodeID})
	}
	if inspection.Details.NodeID == "" && !inspection.LoadingDetails && inspection.DetailsErr == nil {
		inspection.LoadingDetails = true
		requests = append(requests, Request{Kind: RequestReadDetails, NodeID: inspection.Node.NodeID})
	}
	return requests
}

func (s *InspectionSet) appendTrendPoint(node opcua.AddressNode, value opcua.LiveValue) {
	if s.trends == nil {
		s.trends = map[string][]SessionTrendPoint{}
	}
	if s.trendNodes == nil {
		s.trendNodes = map[string]opcua.AddressNode{}
	}
	s.trendNodes[node.NodeID] = node
	if _, ok := s.trends[node.NodeID]; !ok {
		s.trendOrder = append(s.trendOrder, node.NodeID)
	}
	receivedAt := time.Now().Format(time.RFC3339Nano)
	point := SessionTrendPoint{
		Value:           value.Value,
		Status:          value.Status,
		Timestamp:       displayTimestamp(value, receivedAt),
		SourceTimestamp: formatTimestamp(value.SourceTimestamp),
		ServerTimestamp: formatTimestamp(value.ServerTimestamp),
		ReceivedAt:      receivedAt,
	}
	s.trends[node.NodeID] = append(s.trends[node.NodeID], point)
	if len(s.trends[node.NodeID]) > sessionTrendPointLimit {
		s.trends[node.NodeID] = s.trends[node.NodeID][len(s.trends[node.NodeID])-sessionTrendPointLimit:]
	}
}

func displayTimestamp(value opcua.LiveValue, receivedAt string) string {
	if !value.SourceTimestamp.IsZero() {
		return formatTimestamp(value.SourceTimestamp)
	}
	if !value.ServerTimestamp.IsZero() {
		return formatTimestamp(value.ServerTimestamp)
	}
	return receivedAt
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func outOfRangeText(value string, valueRange *opcua.ValueRange) string {
	if valueRange == nil {
		return ""
	}
	numeric, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return ""
	}
	if numeric < valueRange.Low {
		return fmt.Sprintf("%g is below %g", numeric, valueRange.Low)
	}
	if numeric > valueRange.High {
		return fmt.Sprintf("%g is above %g", numeric, valueRange.High)
	}
	return ""
}

func (s *InspectionSet) removeWatchOrder(nodeID string) {
	for i, watchedNodeID := range s.watchOrder {
		if watchedNodeID == nodeID {
			s.watchOrder = append(s.watchOrder[:i], s.watchOrder[i+1:]...)
			return
		}
	}
}

func (s *InspectionSet) cleanupIfUnreferenced(nodeID string) []Request {
	inspection, ok := s.inspections[nodeID]
	if !ok || inspection.Watched || s.selectedNodeID == nodeID {
		return nil
	}
	delete(s.inspections, nodeID)
	if inspection.Subscription == nil {
		return nil
	}
	return []Request{{Kind: RequestCancelSubscription, NodeID: nodeID, Subscription: inspection.Subscription}}
}
