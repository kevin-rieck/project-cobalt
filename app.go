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
	eventDiagnosticLogAppended     = "diagnostic-log-appended"
)

// App is the Wails backend boundary for OPC UA Studio.
type App struct {
	ctx context.Context

	mu          sync.Mutex
	client      opcua.Client
	inspections *session.InspectionSet
	logs        []DiagnosticLogEntry
	connected   bool
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
	Error          string            `json:"error"`
	DetailsError   string            `json:"detailsError"`
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
	a.mu.Unlock()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Subscribe failed for %s: %v", nodeID, err))
	}
	a.emitInspection(view)
	a.executeInspectionRequests(requests)
	if err != nil || updates == nil {
		return
	}
	for value := range updates {
		a.mu.Lock()
		requests := a.inspections.ApplyLiveValue(nodeID, value, nil)
		view := a.currentInspectionLocked()
		a.mu.Unlock()
		a.emitInspection(view)
		a.executeInspectionRequests(requests)
	}
}

func (a *App) readNodeDetails(nodeID string) {
	details, err := a.client.ReadNodeDetails(a.ctx, nodeID)
	a.mu.Lock()
	a.inspections.ApplyDetails(nodeID, details, err)
	view := a.currentInspectionLocked()
	a.mu.Unlock()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Read details failed for %s: %v", nodeID, err))
	}
	a.emitInspection(view)
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
	}
	if inspection.Err != nil {
		view.Error = inspection.Err.Error()
	}
	if inspection.DetailsErr != nil {
		view.DetailsError = inspection.DetailsErr.Error()
	}
	return view
}

func (a *App) emitInspection(view *VariableNodeInspectionView) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventVariableInspectionUpdated, view)
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
