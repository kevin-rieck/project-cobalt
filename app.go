package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"opcua-studio/internal/opcua"
	"opcua-studio/internal/session"
)

const (
	eventVariableInspectionUpdated = "variable-inspection-updated"
	eventWatchlistUpdated          = "watchlist-updated"
	eventSessionTrendUpdated       = "session-trend-updated"
	eventDiagnosticLogAppended     = "diagnostic-log-appended"
)

// App is the Wails backend boundary for OPC UA Studio.
type App struct {
	ctx context.Context

	mu          sync.Mutex
	client      opcua.Client
	inspections *session.InspectionSet
	logs               []DiagnosticLogEntry
	connected          bool
	trendNotifyPending bool
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{client: opcua.NewClient(), inspections: session.NewInspectionSet()}
}

// startup is called when the app starts. The context is saved so we can emit runtime events.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.appendLog("info", "OPC UA Studio started")
}

type DiagnosticLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type ConnectionRequest struct {
	Endpoint       string         `json:"endpoint"`
	SecurityPolicy string         `json:"securityPolicy"`
	SecurityMode   string         `json:"securityMode"`
	AuthType       opcua.AuthType `json:"authType"`
	Username       string         `json:"username"`
	Password       string         `json:"password"`
}

type VariableNodeInspectionView struct {
	Node           opcua.AddressNode `json:"node"`
	Value          opcua.LiveValue   `json:"value"`
	Details        opcua.NodeDetails `json:"details"`
	Subscribing    bool              `json:"subscribing"`
	LoadingDetails bool              `json:"loadingDetails"`
	Stale          bool              `json:"stale"`
	OutOfRange     string            `json:"outOfRange"`
	UpdateCount    int               `json:"updateCount"`
	Watched        bool              `json:"watched"`
	Error          string            `json:"error"`
	DetailsError   string            `json:"detailsError"`
}

type WatchlistRowView struct {
	Node            opcua.AddressNode `json:"node"`
	Value           opcua.LiveValue   `json:"value"`
	DataType        string            `json:"dataType"`
	EngineeringUnit string            `json:"engineeringUnit"`
	Stale           bool              `json:"stale"`
	OutOfRange      string            `json:"outOfRange"`
	UpdateCount     int               `json:"updateCount"`
	Error           string            `json:"error"`
	DetailsError    string            `json:"detailsError"`
}

func (a *App) DiscoverEndpoints(endpoint string) ([]opcua.Endpoint, error) {
	a.appendLog("info", fmt.Sprintf("Discovering endpoints for %s", endpoint))
	endpoints, err := a.client.DiscoverEndpoints(a.ctx, endpoint)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Endpoint discovery failed: %v", err))
		return nil, err
	}
	a.appendLog("info", fmt.Sprintf("Discovered %d endpoints", len(endpoints)))
	return endpoints, nil
}

func (a *App) Connect(request ConnectionRequest) error {
	connectRequest := opcua.ConnectRequest{
		Endpoint:       request.Endpoint,
		SecurityPolicy: request.SecurityPolicy,
		SecurityMode:   request.SecurityMode,
		AuthType:       request.AuthType,
		Username:       request.Username,
		Password:       request.Password,
	}
	a.appendLog("info", fmt.Sprintf("Connecting to %s (%s / %s / %s)", request.Endpoint, request.SecurityPolicy, request.SecurityMode, request.AuthType))
	if err := a.client.Connect(a.ctx, connectRequest); err != nil {
		a.appendLog("error", fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	a.mu.Lock()
	a.connected = true
	a.inspections = session.NewInspectionSet()
	a.mu.Unlock()
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()
	a.appendLog("info", "Connected")
	return nil
}

func (a *App) Disconnect() error {
	a.appendLog("info", "Disconnecting")
	if err := a.client.Close(a.ctx); err != nil {
		a.appendLog("error", fmt.Sprintf("Disconnect failed: %v", err))
		return err
	}
	a.mu.Lock()
	a.client = opcua.NewClient()
	a.inspections = session.NewInspectionSet()
	a.connected = false
	a.mu.Unlock()
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()
	a.appendLog("info", "Disconnected")
	return nil
}

func (a *App) BrowseChildren(nodeID string) ([]opcua.AddressNode, error) {
	if nodeID == "" {
		nodeID = "i=85"
	}
	a.appendLog("info", fmt.Sprintf("Browsing children of %s", nodeID))
	children, err := a.client.BrowseChildren(a.ctx, nodeID)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Browse failed for %s: %v", nodeID, err))
		return nil, err
	}
	a.appendLog("info", fmt.Sprintf("Browsed %d children of %s", len(children), nodeID))
	return children, nil
}

func (a *App) InspectVariableNode(node opcua.AddressNode) error {
	if node.NodeClass != "Variable" {
		return a.ClearVariableNodeInspection()
	}
	a.appendLog("info", fmt.Sprintf("Inspecting Variable Node %s", node.NodeID))
	a.mu.Lock()
	requests := a.inspections.Select(node)
	view := a.currentInspectionLocked()
	a.mu.Unlock()
	a.emitInspection(view)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *App) ClearVariableNodeInspection() error {
	a.mu.Lock()
	requests := a.inspections.Unselect()
	a.mu.Unlock()
	a.emitInspection(nil)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *App) WatchVariableNode(node opcua.AddressNode) error {
	if node.NodeClass != "Variable" {
		return fmt.Errorf("only Variable Nodes can be added to the Watchlist")
	}
	a.appendLog("info", fmt.Sprintf("Adding Variable Node %s to Watchlist", node.NodeID))
	a.mu.Lock()
	requests := a.inspections.Watch(node)
	rows := a.watchlistLocked()
	view := a.currentInspectionLocked()
	a.mu.Unlock()
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *App) UnwatchVariableNode(nodeID string) error {
	a.appendLog("info", fmt.Sprintf("Removing Variable Node %s from Watchlist", nodeID))
	a.mu.Lock()
	requests := a.inspections.Unwatch(nodeID)
	rows := a.watchlistLocked()
	view := a.currentInspectionLocked()
	a.mu.Unlock()
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *App) GetWatchlist() []WatchlistRowView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.watchlistLocked()
}

func (a *App) GetSessionTrend(focusedNodeID string) session.SessionTrendView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inspections.SessionTrend(focusedNodeID)
}

func (a *App) GetDiagnosticLogs() []DiagnosticLogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	logs := make([]DiagnosticLogEntry, len(a.logs))
	copy(logs, a.logs)
	return logs
}

func (a *App) executeInspectionRequests(requests []session.Request) {
	for _, request := range requests {
		req := request
		switch req.Kind {
		case session.RequestSubscribeValue:
			go a.subscribeValue(req.NodeID)
		case session.RequestReadDetails:
			go a.readNodeDetails(req.NodeID)
		case session.RequestCancelSubscription:
			if req.Subscription != nil {
				_ = req.Subscription.Cancel(a.ctx)
			}
		}
	}
}

func (a *App) subscribeValue(nodeID string) {
	updates, subscription, err := a.client.SubscribeValue(a.ctx, nodeID)
	a.mu.Lock()
	requests := a.inspections.ApplySubscription(nodeID, updates, subscription, err)
	view := a.currentInspectionLocked()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Subscribe failed for %s: %v", nodeID, err))
	}
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	a.executeInspectionRequests(requests)
	if err != nil || updates == nil {
		return
	}
	for value := range updates {
		a.mu.Lock()
		requests := a.inspections.ApplyLiveValue(nodeID, value, nil)
		view := a.currentInspectionLocked()
		rows := a.watchlistLocked()
		a.mu.Unlock()
		a.emitInspection(view)
		a.emitWatchlistRows(rows)
		a.scheduleSessionTrendUpdate()
		a.executeInspectionRequests(requests)
	}
}

func (a *App) readNodeDetails(nodeID string) {
	details, err := a.client.ReadNodeDetails(a.ctx, nodeID)
	a.mu.Lock()
	a.inspections.ApplyDetails(nodeID, details, err)
	view := a.currentInspectionLocked()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Read details failed for %s: %v", nodeID, err))
	}
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
}

func (a *App) currentInspectionLocked() *VariableNodeInspectionView {
	inspection, ok := a.inspections.Selected()
	if !ok {
		return nil
	}
	view := inspectionView(inspection)
	return &view
}

func inspectionView(inspection session.VariableNodeInspection) VariableNodeInspectionView {
	view := VariableNodeInspectionView{
		Node:           inspection.Node,
		Value:          inspection.Value,
		Details:        inspection.Details,
		Subscribing:    inspection.Subscribing,
		LoadingDetails: inspection.LoadingDetails,
		Stale:          inspection.Stale,
		OutOfRange:     inspection.OutOfRange,
		UpdateCount:    inspection.UpdateCount,
		Watched:        inspection.Watched,
	}
	if inspection.Err != nil {
		view.Error = inspection.Err.Error()
	}
	if inspection.DetailsErr != nil {
		view.DetailsError = inspection.DetailsErr.Error()
	}
	return view
}

func watchlistRowView(inspection session.VariableNodeInspection) WatchlistRowView {
	row := WatchlistRowView{
		Node:            inspection.Node,
		Value:           inspection.Value,
		DataType:        inspection.Details.DataType,
		EngineeringUnit: inspection.Details.EngineeringUnit,
		Stale:           inspection.Stale,
		OutOfRange:      inspection.OutOfRange,
		UpdateCount:     inspection.UpdateCount,
	}
	if inspection.Err != nil {
		row.Error = inspection.Err.Error()
	}
	if inspection.DetailsErr != nil {
		row.DetailsError = inspection.DetailsErr.Error()
	}
	return row
}

func (a *App) watchlistLocked() []WatchlistRowView {
	watched := a.inspections.Watched()
	rows := make([]WatchlistRowView, 0, len(watched))
	for _, inspection := range watched {
		rows = append(rows, watchlistRowView(inspection))
	}
	return rows
}

func (a *App) emitInspection(view *VariableNodeInspectionView) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventVariableInspectionUpdated, view)
	}
}

func (a *App) emitWatchlist() {
	a.mu.Lock()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	a.emitWatchlistRows(rows)
}

func (a *App) emitWatchlistRows(rows []WatchlistRowView) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventWatchlistUpdated, rows)
	}
}

func (a *App) scheduleSessionTrendUpdate() {
	a.mu.Lock()
	if a.trendNotifyPending {
		a.mu.Unlock()
		return
	}
	a.trendNotifyPending = true
	a.mu.Unlock()
	time.AfterFunc(250*time.Millisecond, func() {
		a.mu.Lock()
		a.trendNotifyPending = false
		a.mu.Unlock()
		a.emitSessionTrendUpdated()
	})
}

func (a *App) emitSessionTrendUpdated() {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventSessionTrendUpdated)
	}
}

func (a *App) appendLog(level, message string) {
	entry := DiagnosticLogEntry{Timestamp: time.Now().Format(time.RFC3339), Level: level, Message: message}
	a.mu.Lock()
	a.logs = append(a.logs, entry)
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventDiagnosticLogAppended, entry)
	}
}
