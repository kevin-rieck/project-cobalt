package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
	"opcua-studio/internal/search"
	"opcua-studio/internal/session"
)

const (
	eventVariableInspectionUpdated    = "variable-inspection-updated"
	eventWatchlistUpdated             = "watchlist-updated"
	eventSessionTrendUpdated          = "session-trend-updated"
	eventDiagnosticLogAppended        = "diagnostic-log-appended"
	defaultShallowIndexBrowseInterval = time.Second
	defaultShallowIndexBrowseBudget   = 250
)

// App is the Wails backend boundary for OPC UA Studio.
type App struct {
	ctx context.Context

	mu                         sync.Mutex
	client                     opcua.Client
	inspections                *session.InspectionSet
	addressSpaceSearch         *search.Service
	logs                       []DiagnosticLogEntry
	savedConnections           []connections.SavedConnection
	savedStore                 *connections.FileStore
	connected                  bool
	trendNotifyPending         bool
	shallowIndexCancel         context.CancelFunc
	shallowIndexPrioritize     chan []opcua.AddressNode
	shallowIndexBrowseInterval time.Duration
	shallowIndexBrowseBudget   int
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return NewAppWithSavedConnectionStore(connections.DefaultStorePath())
}

func NewAppWithSavedConnectionStore(path string) *App {
	return &App{client: opcua.NewClient(), inspections: session.NewInspectionSet(), addressSpaceSearch: search.NewService(), savedStore: connections.NewFileStore(path), savedConnections: []connections.SavedConnection{}, shallowIndexBrowseInterval: defaultShallowIndexBrowseInterval, shallowIndexBrowseBudget: defaultShallowIndexBrowseBudget}
}

// startup is called when the app starts. The context is saved so we can emit runtime events.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if saved, err := a.savedStore.Load(); err != nil {
		a.appendLog("error", fmt.Sprintf("Loading Saved Connections failed: %v", err))
	} else {
		a.mu.Lock()
		a.savedConnections = saved
		a.mu.Unlock()
	}
	a.appendLog("info", "OPC UA Studio started")
}

type DiagnosticLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type ConnectionRequest struct {
	ExistingName          string         `json:"existingName"`
	SavedConnectionID     string         `json:"savedConnectionID"`
	Name                  string         `json:"name"`
	Endpoint              string         `json:"endpoint"`
	SecurityPolicy        string         `json:"securityPolicy"`
	SecurityMode          string         `json:"securityMode"`
	AuthType              opcua.AuthType `json:"authType"`
	Username              string         `json:"username"`
	Password              string         `json:"password"`
	ClientCertificatePath string         `json:"clientCertificatePath"`
	ClientPrivateKeyPath  string         `json:"clientPrivateKeyPath"`
	ServerThumbprint      string         `json:"serverThumbprint"`
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

func objectsRootNode() opcua.AddressNode {
	return opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", BrowseName: "Objects", NodeClass: "Object"}
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

func (a *App) GetSavedConnections() []connections.SavedConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	saved := make([]connections.SavedConnection, len(a.savedConnections))
	copy(saved, a.savedConnections)
	return saved
}

func (a *App) SaveSavedConnection(request ConnectionRequest) (connections.SavedConnection, error) {
	saved, err := a.savedStore.Save(connections.SaveRequest{
		ExistingName:                request.ExistingName,
		Name:                        request.Name,
		Endpoint:                    request.Endpoint,
		SecurityPolicy:              request.SecurityPolicy,
		SecurityMode:                request.SecurityMode,
		AuthType:                    string(request.AuthType),
		Username:                    request.Username,
		Password:                    request.Password,
		ClientCertificatePath:       request.ClientCertificatePath,
		ClientPrivateKeyPath:        request.ClientPrivateKeyPath,
		ServerCertificateThumbprint: request.ServerThumbprint,
	}, time.Now())
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Saving Saved Connection failed: %v", err))
		return connections.SavedConnection{}, err
	}
	a.mu.Lock()
	a.savedConnections, err = a.savedStore.Load()
	if err != nil {
		a.mu.Unlock()
		a.appendLog("error", fmt.Sprintf("Reloading Saved Connections failed: %v", err))
		return connections.SavedConnection{}, err
	}
	a.mu.Unlock()
	a.appendLog("info", fmt.Sprintf("Saved Connection %q", saved.Name))
	return saved, nil
}

func (a *App) DeleteSavedConnection(id string) (bool, error) {
	deleted, err := a.savedStore.Delete(id)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Deleting Saved Connection failed: %v", err))
		return false, err
	}
	if !deleted {
		return false, nil
	}
	saved, err := a.savedStore.Load()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Reloading Saved Connections failed: %v", err))
		return true, err
	}
	a.mu.Lock()
	a.savedConnections = saved
	a.mu.Unlock()
	a.appendLog("info", "Deleted Saved Connection")
	return true, nil
}

func (a *App) Connect(request ConnectionRequest) error {
	if request.AuthType == opcua.AuthUsername && strings.TrimSpace(request.Password) == "" {
		err := fmt.Errorf("username authentication requires password entry at connect time")
		a.appendLog("error", fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	connectRequest := opcua.ConnectRequest{
		Endpoint:              request.Endpoint,
		SecurityPolicy:        request.SecurityPolicy,
		SecurityMode:          request.SecurityMode,
		AuthType:              request.AuthType,
		Username:              request.Username,
		Password:              request.Password,
		ClientCertificatePath: request.ClientCertificatePath,
		ClientPrivateKeyPath:  request.ClientPrivateKeyPath,
	}
	a.appendLog("info", fmt.Sprintf("Connecting to %s (%s / %s / %s)", request.Endpoint, request.SecurityPolicy, request.SecurityMode, request.AuthType))
	if request.ServerThumbprint != "" {
		a.appendLog("info", fmt.Sprintf("Selected server certificate thumbprint: %s", request.ServerThumbprint))
	}
	if err := a.client.Connect(a.ctx, connectRequest); err != nil {
		a.appendLog("error", fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	if request.SavedConnectionID != "" || request.Name != "" {
		if _, ok, err := a.savedStore.MarkConnected(request.SavedConnectionID, request.Name, time.Now()); err != nil {
			a.appendLog("error", fmt.Sprintf("Updating Saved Connection last connected time failed: %v", err))
		} else if ok {
			if saved, err := a.savedStore.Load(); err != nil {
				a.appendLog("error", fmt.Sprintf("Reloading Saved Connections failed: %v", err))
			} else {
				a.mu.Lock()
				a.savedConnections = saved
				a.mu.Unlock()
			}
		}
	}
	a.mu.Lock()
	a.cancelShallowAddressSpaceIndexingLocked()
	a.connected = true
	a.inspections = session.NewInspectionSet()
	a.addressSpaceSearch.Reset()
	a.addressSpaceSearch.AddNodes([]opcua.AddressNode{objectsRootNode()})
	a.startShallowAddressSpaceIndexingLocked()
	a.mu.Unlock()
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()
	a.appendLog("info", "Connected")
	return nil
}

func (a *App) PickClientCertificate() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select OPC UA Client Certificate",
		Filters: []runtime.FileFilter{
			{DisplayName: "Certificate files (*.pem;*.crt;*.cer)", Pattern: "*.pem;*.crt;*.cer"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

func (a *App) PickClientPrivateKey() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select OPC UA Client Private Key",
		Filters: []runtime.FileFilter{
			{DisplayName: "Private key files (*.key;*.pem)", Pattern: "*.key;*.pem"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

func (a *App) Disconnect() error {
	a.appendLog("info", "Disconnecting")
	if err := a.client.Close(a.ctx); err != nil {
		a.appendLog("error", fmt.Sprintf("Disconnect failed: %v", err))
		return err
	}
	a.mu.Lock()
	a.cancelShallowAddressSpaceIndexingLocked()
	a.client = opcua.NewClient()
	a.inspections = session.NewInspectionSet()
	a.addressSpaceSearch.Reset()
	a.connected = false
	a.mu.Unlock()
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()
	a.appendLog("info", "Disconnected")
	return nil
}

func (a *App) startShallowAddressSpaceIndexingLocked() {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	prioritize := make(chan []opcua.AddressNode, 128)
	a.shallowIndexCancel = cancel
	a.shallowIndexPrioritize = prioritize
	client := a.client
	interval := a.shallowIndexBrowseInterval
	if interval <= 0 {
		interval = defaultShallowIndexBrowseInterval
	}
	budget := a.shallowIndexBrowseBudget
	if budget <= 0 {
		budget = defaultShallowIndexBrowseBudget
	}
	go a.runShallowAddressSpaceIndexing(ctx, client, interval, budget, prioritize)
}

func (a *App) cancelShallowAddressSpaceIndexingLocked() {
	if a.shallowIndexCancel != nil {
		a.shallowIndexCancel()
		a.shallowIndexCancel = nil
		a.shallowIndexPrioritize = nil
	}
}

func (a *App) runShallowAddressSpaceIndexing(ctx context.Context, client opcua.Client, interval time.Duration, browseBudget int, prioritize chan []opcua.AddressNode) {
	defer a.finishShallowAddressSpaceIndexing(prioritize)
	priorityQueue := []string{}
	backgroundQueue := []string{objectsRootNode().NodeID}
	seen := map[string]bool{objectsRootNode().NodeID: true}
	firstBrowse := true
	browseCount := 0

	for {
		drainShallowIndexPriorityRequests(prioritize, &priorityQueue, seen)
		if len(priorityQueue) == 0 && len(backgroundQueue) == 0 {
			select {
			case <-ctx.Done():
				return
			case nodes := <-prioritize:
				enqueueShallowIndexParentNodes(nodes, &priorityQueue, seen)
				continue
			}
		}

		if !firstBrowse {
			timer := time.NewTimer(interval)
			waiting := true
			for waiting {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case nodes := <-prioritize:
					enqueueShallowIndexParentNodes(nodes, &priorityQueue, seen)
				case <-timer.C:
					waiting = false
				}
			}
		}
		firstBrowse = false

		fromPriority := len(priorityQueue) > 0
		nodeID := ""
		if fromPriority {
			nodeID = priorityQueue[0]
			priorityQueue = priorityQueue[1:]
		} else {
			nodeID = backgroundQueue[0]
			backgroundQueue = backgroundQueue[1:]
		}
		if browseCount >= browseBudget {
			return
		}
		browseCount++
		children, err := client.BrowseChildren(ctx, nodeID)
		if err != nil {
			if ctx.Err() == nil {
				a.appendLog("error", fmt.Sprintf("Shallow Address Space Indexing browse failed for %s: %v", nodeID, err))
			}
			continue
		}
		if ctx.Err() != nil {
			return
		}
		a.addressSpaceSearch.AddNodes(children)
		if fromPriority {
			enqueueShallowIndexParentNodes(children, &priorityQueue, seen)
		} else {
			enqueueShallowIndexParentNodes(children, &backgroundQueue, seen)
		}
		if browseCount >= browseBudget {
			return
		}
	}
}

func (a *App) finishShallowAddressSpaceIndexing(prioritize chan []opcua.AddressNode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shallowIndexPrioritize == prioritize {
		a.shallowIndexCancel = nil
		a.shallowIndexPrioritize = nil
	}
}

func drainShallowIndexPriorityRequests(prioritize <-chan []opcua.AddressNode, priorityQueue *[]string, seen map[string]bool) {
	for {
		select {
		case nodes := <-prioritize:
			enqueueShallowIndexParentNodes(nodes, priorityQueue, seen)
		default:
			return
		}
	}
}

func enqueueShallowIndexParentNodes(nodes []opcua.AddressNode, queue *[]string, seen map[string]bool) {
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" || seen[nodeID] || !isShallowIndexParentNode(node) {
			continue
		}
		seen[nodeID] = true
		*queue = append(*queue, nodeID)
	}
}

func isShallowIndexParentNode(node opcua.AddressNode) bool {
	switch node.NodeClass {
	case "Object", "View":
		return true
	default:
		return false
	}
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
	a.addressSpaceSearch.AddNodes(children)
	a.prioritizeShallowAddressSpaceIndexing(children)
	a.appendLog("info", fmt.Sprintf("Browsed %d children of %s", len(children), nodeID))
	return children, nil
}

func (a *App) prioritizeShallowAddressSpaceIndexing(nodes []opcua.AddressNode) {
	a.mu.Lock()
	prioritize := a.shallowIndexPrioritize
	a.mu.Unlock()
	if prioritize == nil {
		return
	}
	select {
	case prioritize <- nodes:
	default:
		a.appendLog("error", "Shallow Address Space Indexing priority queue is full")
	}
}

func (a *App) SearchAddressSpace(query string) (search.AddressSpaceSearchView, error) {
	a.mu.Lock()
	connected := a.connected
	a.mu.Unlock()
	if !connected {
		return search.AddressSpaceSearchView{Query: query, Results: []search.AddressSpaceSearchResult{}, Status: "Connect to an OPC UA Server to search browsed Address Space metadata."}, nil
	}
	return a.addressSpaceSearch.Search(query), nil
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
