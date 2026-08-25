package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"opcua-studio/internal/application"
	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
	"opcua-studio/internal/search"
	"opcua-studio/internal/session"
)

// App is the Wails delivery adapter for OPC UA Studio. Troubleshooting Session
// behavior lives in the platform-neutral application.Studio.
type App struct {
	studio *application.Studio
	ctx    context.Context
	emit   func(context.Context, application.Event)
}

// NewApp creates the desktop delivery adapter.
func NewApp() *App {
	return NewAppWithSavedConnectionStore(connections.DefaultStorePath())
}

func NewAppWithSavedConnectionStore(path string) *App {
	return newApp(application.NewStudioWithSavedConnectionStore(path), wailsEventEmitter)
}

func newApp(studio *application.Studio, emit func(context.Context, application.Event)) *App {
	app := &App{studio: studio, emit: emit}
	studio.SetEventSink(app.forwardEvent)
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.studio.Start(ctx)
}

func (a *App) forwardEvent(event application.Event) {
	a.emit(a.ctx, event)
}

func wailsEventEmitter(ctx context.Context, event application.Event) {
	if ctx != nil {
		runtime.EventsEmit(ctx, string(event.Type()), event.Payload())
	}
}

type DiagnosticLogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type SessionSafetyView struct {
	Connected    bool `json:"connected"`
	ReadOnlyMode bool `json:"readOnlyMode"`
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

type MethodNodeRequest struct {
	ObjectNodeID string `json:"objectNodeID"`
	MethodNodeID string `json:"methodNodeID"`
}

type MethodCallRequest struct {
	ObjectNodeID   string   `json:"objectNodeID"`
	MethodNodeID   string   `json:"methodNodeID"`
	InputArguments []string `json:"inputArguments"`
}

type VariableNodeWriteRequest struct {
	NodeID      string `json:"nodeID"`
	TargetValue string `json:"targetValue"`
}

type VariableNodeWriteResult struct {
	NodeID      string          `json:"nodeID"`
	TargetValue string          `json:"targetValue"`
	Status      string          `json:"status"`
	ReadBack    opcua.LiveValue `json:"readBack"`
	Warning     string          `json:"warning"`
}

func (a *App) GetSessionSafety() SessionSafetyView {
	return sessionSafetyView(a.studio.GetSessionSafety())
}

func (a *App) SetReadOnlyMode(enabled bool) error {
	return a.studio.SetReadOnlyMode(enabled)
}

func (a *App) DiscoverEndpoints(endpoint string) ([]opcua.Endpoint, error) {
	return a.studio.DiscoverEndpoints(endpoint)
}

func (a *App) GetSavedConnections() []connections.SavedConnection {
	return a.studio.GetSavedConnections()
}

func (a *App) SaveSavedConnection(request ConnectionRequest) (connections.SavedConnection, error) {
	return a.studio.SaveSavedConnection(applicationConnectionRequest(request))
}

func (a *App) DeleteSavedConnection(id string) (bool, error) {
	return a.studio.DeleteSavedConnection(id)
}

func (a *App) Connect(request ConnectionRequest) error {
	return a.studio.Connect(applicationConnectionRequest(request))
}

func (a *App) Disconnect() error {
	return a.studio.Disconnect()
}

func (a *App) BrowseChildren(nodeID string) ([]opcua.AddressNode, error) {
	return a.studio.BrowseChildren(nodeID)
}

func (a *App) SearchAddressSpace(query string) (search.AddressSpaceSearchView, error) {
	return a.studio.SearchAddressSpace(query)
}

func (a *App) GetMethodDetails(request MethodNodeRequest) (opcua.MethodDetails, error) {
	return a.studio.GetMethodDetails(application.MethodNodeRequest(request))
}

func (a *App) CallMethod(request MethodCallRequest) (opcua.MethodCallResult, error) {
	return a.studio.CallMethod(application.MethodCallRequest(request))
}

func (a *App) InspectVariableNode(node opcua.AddressNode) error {
	return a.studio.InspectVariableNode(node)
}

func (a *App) ClearVariableNodeInspection() error {
	return a.studio.ClearVariableNodeInspection()
}

func (a *App) WatchVariableNode(node opcua.AddressNode) error {
	return a.studio.WatchVariableNode(node)
}

func (a *App) UnwatchVariableNode(nodeID string) error {
	return a.studio.UnwatchVariableNode(nodeID)
}

func (a *App) GetWatchlist() []WatchlistRowView {
	rows := a.studio.GetWatchlist()
	views := make([]WatchlistRowView, len(rows))
	for i, row := range rows {
		views[i] = watchlistRowView(row)
	}
	return views
}

func (a *App) GetSessionTrend(focusedNodeID string) session.SessionTrendView {
	return a.studio.GetSessionTrend(focusedNodeID)
}

func (a *App) RefreshVariableNodeValue(nodeID string) error {
	return a.studio.RefreshVariableNodeValue(nodeID)
}

func (a *App) WriteVariableNodeValue(request VariableNodeWriteRequest) (VariableNodeWriteResult, error) {
	result, err := a.studio.WriteVariableNodeValue(application.VariableNodeWriteRequest(request))
	return variableNodeWriteResult(result), err
}

func (a *App) GetDiagnosticLogs() []DiagnosticLogEntry {
	logs := a.studio.GetDiagnosticLogs()
	entries := make([]DiagnosticLogEntry, len(logs))
	for i, entry := range logs {
		entries[i] = DiagnosticLogEntry(entry)
	}
	return entries
}

// PickClientCertificate retains Wails-native Client Certificate selection.
func (a *App) PickClientCertificate() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select OPC UA Client Certificate",
		Filters: []runtime.FileFilter{
			{DisplayName: "Certificate files (*.pem;*.crt;*.cer)", Pattern: "*.pem;*.crt;*.cer"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

// PickClientPrivateKey retains Wails-native Client Private Key selection.
func (a *App) PickClientPrivateKey() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select OPC UA Client Private Key",
		Filters: []runtime.FileFilter{
			{DisplayName: "Private key files (*.key;*.pem)", Pattern: "*.key;*.pem"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
}

func applicationConnectionRequest(request ConnectionRequest) application.ConnectionRequest {
	return application.ConnectionRequest(request)
}

func sessionSafetyView(view application.SessionSafetyView) SessionSafetyView {
	return SessionSafetyView(view)
}

func watchlistRowView(row application.WatchlistRowView) WatchlistRowView {
	return WatchlistRowView(row)
}

func variableNodeWriteResult(result application.VariableNodeWriteResult) VariableNodeWriteResult {
	return VariableNodeWriteResult(result)
}
