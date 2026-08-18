package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
	"opcua-studio/internal/search"
	"opcua-studio/internal/session"
)

const (
	defaultDisconnectCloseTimeout = 2 * time.Second
	defaultOperationTimeout       = 15 * time.Second
	defaultMethodCallTimeout      = 30 * time.Second
	maxWatchlistNodes             = 100
)

// OperationTimeouts bounds each OPC UA operation. A zero field uses the
// conservative production default for that operation.
type OperationTimeouts struct {
	Discovery  time.Duration
	Connect    time.Duration
	Read       time.Duration
	Browse     time.Duration
	Write      time.Duration
	MethodCall time.Duration
	Disconnect time.Duration
}

func (t OperationTimeouts) withDefaults() OperationTimeouts {
	if t.Discovery <= 0 {
		t.Discovery = defaultOperationTimeout
	}
	if t.Connect <= 0 {
		t.Connect = defaultOperationTimeout
	}
	if t.Read <= 0 {
		t.Read = defaultOperationTimeout
	}
	if t.Browse <= 0 {
		t.Browse = defaultOperationTimeout
	}
	if t.Write <= 0 {
		t.Write = defaultOperationTimeout
	}
	if t.MethodCall <= 0 {
		t.MethodCall = defaultMethodCallTimeout
	}
	if t.Disconnect <= 0 {
		t.Disconnect = defaultDisconnectCloseTimeout
	}
	return t
}

// SavedConnectionStore is the persistence boundary required by Studio.
type SavedConnectionStore interface {
	Load() ([]connections.SavedConnection, error)
	Save(connections.SaveRequest, time.Time) (connections.SavedConnection, error)
	MarkConnected(id, name string, now time.Time) (connections.SavedConnection, bool, error)
	Delete(id string) (bool, error)
}

// Options supplies platform concerns to a Studio. Nil dependencies use the
// production adapters.
type Options struct {
	ClientFactory        func() opcua.Client
	SavedConnectionStore SavedConnectionStore
	Now                  func() time.Time
	WithTimeout          func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	AfterFunc            func(time.Duration, func()) *time.Timer
	Logger               func(DiagnosticLogEntry)
	EventSink            EventSink
	OperationTimeouts    OperationTimeouts
	MaxConcurrentReads   int
	NewSearchSession     func(context.Context, search.Browser) *search.Session
}

// ActiveConnection is the immutable, non-secret identity captured at connect
// time. Editing or deleting a Saved Connection never changes it.
type ActiveConnection struct {
	SavedConnectionID     string
	Name                  string
	Endpoint              string
	SecurityPolicy        string
	SecurityMode          string
	AuthType              opcua.AuthType
	Username              string
	ClientCertificatePath string
	ClientPrivateKeyPath  string
	ServerThumbprint      string
}

// UnknownMutationOutcomeError reports a mutation whose completion cannot be
// proven after its operation deadline elapsed.
type UnknownMutationOutcomeError struct {
	Operation string
	Err       error
}

func (e *UnknownMutationOutcomeError) Error() string {
	return fmt.Sprintf("%s outcome is unknown; inspect OPC UA Server state before retrying: %v", e.Operation, e.Err)
}

func (e *UnknownMutationOutcomeError) Unwrap() error { return e.Err }

// ErrUnknownMutationInFlight prevents state changes while an uncooperative OPC
// UA client may still be executing a mutation that already timed out.
var ErrUnknownMutationInFlight = errors.New("cannot change the Troubleshooting Session while a mutation outcome is unknown")

// EventType identifies an application state change for a delivery adapter.
type EventType string

const (
	EventVariableInspectionUpdated EventType = "variable-inspection-updated"
	EventWatchlistUpdated          EventType = "watchlist-updated"
	EventSessionTrendUpdated       EventType = "session-trend-updated"
	EventDiagnosticLogAppended     EventType = "diagnostic-log-appended"
	EventSessionSafetyUpdated      EventType = "session-safety-updated"
)

// Event is a sealed, typed application state change delivered to an adapter.
// Every implementation binds an event kind to exactly one payload type.
type Event interface {
	Type() EventType
	Payload() any
	isApplicationEvent()
}

// VariableInspectionUpdated carries the selected Variable Node Inspection.
type VariableInspectionUpdated struct{ View *VariableNodeInspectionView }

func (VariableInspectionUpdated) Type() EventType     { return EventVariableInspectionUpdated }
func (e VariableInspectionUpdated) Payload() any      { return e.View }
func (VariableInspectionUpdated) isApplicationEvent() {}

// WatchlistUpdated carries the current Watchlist rows.
type WatchlistUpdated struct{ Rows []WatchlistRowView }

func (WatchlistUpdated) Type() EventType     { return EventWatchlistUpdated }
func (e WatchlistUpdated) Payload() any      { return e.Rows }
func (WatchlistUpdated) isApplicationEvent() {}

// SessionTrendUpdated announces that Session Trend must be refreshed.
type SessionTrendUpdated struct{}

func (SessionTrendUpdated) Type() EventType     { return EventSessionTrendUpdated }
func (SessionTrendUpdated) Payload() any        { return nil }
func (SessionTrendUpdated) isApplicationEvent() {}

// DiagnosticLogAppended carries one sanitized diagnostic record.
type DiagnosticLogAppended struct{ Entry DiagnosticLogEntry }

func (DiagnosticLogAppended) Type() EventType     { return EventDiagnosticLogAppended }
func (e DiagnosticLogAppended) Payload() any      { return e.Entry }
func (DiagnosticLogAppended) isApplicationEvent() {}

// SessionSafetyUpdated carries the current connection and Read-Only state.
type SessionSafetyUpdated struct{ View SessionSafetyView }

func (SessionSafetyUpdated) Type() EventType     { return EventSessionSafetyUpdated }
func (e SessionSafetyUpdated) Payload() any      { return e.View }
func (SessionSafetyUpdated) isApplicationEvent() {}

// EventSink receives application state changes. It must not retain mutable state.
type EventSink func(Event)

// Studio owns the platform-neutral Troubleshooting Session application state.
type Studio struct {
	ctx context.Context

	mu                           sync.Mutex
	coordinator                  sync.RWMutex
	client                       opcua.Client
	newClient                    func() opcua.Client
	inspections                  *session.InspectionSet
	addressSpaceSearchSession    *search.Session
	newAddressSpaceSearchSession func(context.Context, search.Browser) *search.Session
	logs                         []DiagnosticLogEntry
	savedConnections             []connections.SavedConnection
	savedStore                   SavedConnectionStore
	activeConnection             *ActiveConnection
	mutationOutcomeUncertain     bool
	mutationExecutionPending     bool
	connected                    bool
	readOnlyMode                 bool
	trendNotifyPending           bool
	timeouts                     OperationTimeouts
	safeReadSemaphore            chan struct{}
	now                          func() time.Time
	withTimeout                  func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	afterFunc                    func(time.Duration, func()) *time.Timer
	logger                       func(DiagnosticLogEntry)
	eventSink                    EventSink
}

// NewStudio creates a platform-neutral Troubleshooting Session application.
func NewStudio(configured ...Options) *Studio {
	options := Options{}
	if len(configured) > 0 {
		options = configured[0]
	}
	clientFactory := options.ClientFactory
	if clientFactory == nil {
		clientFactory = opcua.NewClient
	}
	store := options.SavedConnectionStore
	if store == nil {
		store = connections.NewFileStore(connections.DefaultStorePath())
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	withTimeout := options.WithTimeout
	if withTimeout == nil {
		withTimeout = context.WithTimeout
	}
	afterFunc := options.AfterFunc
	if afterFunc == nil {
		afterFunc = time.AfterFunc
	}
	maxConcurrentReads := options.MaxConcurrentReads
	if maxConcurrentReads <= 0 {
		maxConcurrentReads = 4
	}
	return &Studio{
		client:                       clientFactory(),
		newClient:                    clientFactory,
		inspections:                  session.NewInspectionSet(now),
		savedStore:                   store,
		savedConnections:             []connections.SavedConnection{},
		readOnlyMode:                 true,
		timeouts:                     options.OperationTimeouts.withDefaults(),
		safeReadSemaphore:            make(chan struct{}, maxConcurrentReads),
		now:                          now,
		withTimeout:                  withTimeout,
		afterFunc:                    afterFunc,
		logger:                       options.Logger,
		eventSink:                    options.EventSink,
		newAddressSpaceSearchSession: options.NewSearchSession,
	}
}

func NewStudioWithSavedConnectionStore(path string) *Studio {
	return NewStudio(Options{SavedConnectionStore: connections.NewFileStore(path)})
}

// SetEventSink supplies the delivery adapter's event publisher.
func (a *Studio) SetEventSink(sink EventSink) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.eventSink = sink
}

// Start loads persisted Saved Connections and establishes the operation context.
func (a *Studio) Start(ctx context.Context) {
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

func (a *Studio) operationContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	a.mu.Lock()
	base := a.ctx
	a.mu.Unlock()
	if base == nil {
		base = context.Background()
	}
	return a.withTimeout(base, timeout)
}

type operationResult[T any] struct {
	value T
	err   error
}

func startOperation[T any](work func() (T, error)) <-chan operationResult[T] {
	done := make(chan operationResult[T], 1)
	go func() {
		value, err := work()
		done <- operationResult[T]{value: value, err: err}
	}()
	return done
}

func awaitOperation[T any](ctx context.Context, done <-chan operationResult[T]) (T, error) {
	var zero T
	select {
	case result := <-done:
		return result.value, result.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func runOperation[T any](ctx context.Context, work func() (T, error)) (T, error) {
	return awaitOperation(ctx, startOperation(work))
}

func withClientRead[T any](a *Studio, timeout time.Duration, work func(context.Context, opcua.Client) (T, error), onLateCompletion func(T)) (T, error) {
	var zero T
	ctx, cancel := a.operationContext(timeout)
	defer cancel()

	a.coordinator.RLock()
	select {
	case a.safeReadSemaphore <- struct{}{}:
	case <-ctx.Done():
		a.coordinator.RUnlock()
		return zero, ctx.Err()
	}

	a.mu.Lock()
	client := a.client
	a.mu.Unlock()
	done := make(chan operationResult[T], 1)
	go func() {
		value, err := work(ctx, client)
		done <- operationResult[T]{value: value, err: err}
	}()
	select {
	case result := <-done:
		<-a.safeReadSemaphore
		a.coordinator.RUnlock()
		return result.value, result.err
	case <-ctx.Done():
		a.coordinator.RUnlock()
		go func() {
			result := <-done
			if onLateCompletion != nil {
				onLateCompletion(result.value)
			}
			<-a.safeReadSemaphore
		}()
		return zero, ctx.Err()
	}
}

type deadlineBrowser struct {
	studio *Studio
	client opcua.Client
}

func (b deadlineBrowser) BrowseChildren(_ context.Context, nodeID string) ([]opcua.AddressNode, error) {
	return withClientRead(b.studio, b.studio.timeouts.Browse, func(ctx context.Context, _ opcua.Client) ([]opcua.AddressNode, error) {
		return b.client.BrowseChildren(ctx, nodeID)
	}, nil)
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

// Snapshot is the authoritative, adapter-neutral view of the current
// Troubleshooting Session.
type Snapshot struct {
	Safety                   SessionSafetyView
	MutationOutcomeUncertain bool
	MutationExecutionPending bool
	ActiveConnection         *ActiveConnection
	SavedConnections         []connections.SavedConnection
	Inspection               *VariableNodeInspectionView
	Watchlist                []WatchlistRowView
	SessionTrend             session.SessionTrendView
	DiagnosticLogs           []DiagnosticLogEntry
}

func (a *Studio) GetSessionSafety() SessionSafetyView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionSafetyLocked()
}

// Snapshot returns one consistent view of application state for delivery
// adapters that need to resynchronize after reconnecting.
func (a *Studio) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	snapshot := Snapshot{
		Safety:                   a.sessionSafetyLocked(),
		MutationOutcomeUncertain: a.mutationOutcomeUncertain,
		MutationExecutionPending: a.mutationExecutionPending,
		SavedConnections:         append([]connections.SavedConnection(nil), a.savedConnections...),
		Watchlist:                a.watchlistLocked(),
		SessionTrend:             a.inspections.SessionTrend(""),
		DiagnosticLogs:           append([]DiagnosticLogEntry(nil), a.logs...),
	}
	if a.activeConnection != nil {
		connection := *a.activeConnection
		snapshot.ActiveConnection = &connection
	}
	if inspection, ok := a.inspections.Selected(); ok {
		view := inspectionView(inspection)
		snapshot.Inspection = &view
	}
	return snapshot
}

func (a *Studio) SetReadOnlyMode(enabled bool) error {
	a.coordinator.Lock()
	a.mu.Lock()
	if a.mutationExecutionPending {
		a.mu.Unlock()
		a.coordinator.Unlock()
		return ErrUnknownMutationInFlight
	}
	if !enabled && !a.connected {
		a.mu.Unlock()
		a.coordinator.Unlock()
		return fmt.Errorf("Read-Only Mode can be disabled only for a connected session")
	}
	if !enabled && a.mutationOutcomeUncertain {
		// Deliberately re-enabling writes is the Automation Engineer's explicit
		// acknowledgement that server state was inspected after the unknown outcome.
		a.mutationOutcomeUncertain = false
	}
	wasReadOnly := a.readOnlyMode
	a.readOnlyMode = enabled
	view := a.sessionSafetyLocked()
	a.mu.Unlock()
	a.coordinator.Unlock()
	if enabled && !wasReadOnly {
		a.appendLog("info", "Read-Only Mode enabled")
	}
	a.emitSessionSafetyUpdated(view)
	return nil
}

func (a *Studio) DiscoverEndpoints(endpoint string) ([]opcua.Endpoint, error) {
	a.appendLog("info", fmt.Sprintf("Discovering endpoints for %s", endpoint))
	endpoints, err := withClientRead(a, a.timeouts.Discovery, func(ctx context.Context, client opcua.Client) ([]opcua.Endpoint, error) {
		return client.DiscoverEndpoints(ctx, endpoint)
	}, nil)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Endpoint discovery failed: %v", err))
		return nil, err
	}
	a.appendLog("info", fmt.Sprintf("Discovered %d endpoints", len(endpoints)))
	return endpoints, nil
}

func (a *Studio) GetSavedConnections() []connections.SavedConnection {
	a.mu.Lock()
	defer a.mu.Unlock()
	saved := make([]connections.SavedConnection, len(a.savedConnections))
	copy(saved, a.savedConnections)
	return saved
}

func (a *Studio) SaveSavedConnection(request ConnectionRequest) (connections.SavedConnection, error) {
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
	}, a.now())
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

func (a *Studio) DeleteSavedConnection(id string) (bool, error) {
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

func (a *Studio) Connect(request ConnectionRequest) error {
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

	a.coordinator.Lock()
	a.mu.Lock()
	if a.mutationOutcomeUncertain {
		a.mu.Unlock()
		a.coordinator.Unlock()
		return ErrUnknownMutationInFlight
	}
	previousClient := a.client
	clientToConnect := previousClient
	reconnecting := a.connected
	if reconnecting {
		clientToConnect = a.newClient()
	}
	a.mu.Unlock()

	ctx, cancel := a.operationContext(a.timeouts.Connect)
	done := startOperation(func() (struct{}, error) {
		return struct{}{}, clientToConnect.Connect(ctx, connectRequest)
	})
	var err error
	select {
	case completed := <-done:
		err = completed.err
	case <-ctx.Done():
		cancel()
		go func() {
			<-done
			_ = a.closeClient(clientToConnect)
		}()
		if !reconnecting {
			a.mu.Lock()
			a.client = a.newClient()
			a.mu.Unlock()
		}
		a.coordinator.Unlock()
		a.appendLog("error", fmt.Sprintf("Connection failed: %v", ctx.Err()))
		return ctx.Err()
	}
	cancel()
	if err != nil {
		go func() { _ = a.closeClient(clientToConnect) }()
		if !reconnecting {
			a.mu.Lock()
			a.client = a.newClient()
			a.mu.Unlock()
		}
		a.coordinator.Unlock()
		a.appendLog("error", fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	if request.SavedConnectionID != "" || request.Name != "" {
		if _, ok, err := a.savedStore.MarkConnected(request.SavedConnectionID, request.Name, a.now()); err != nil {
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
	a.stopAddressSpaceSearchSessionLocked()
	a.client = clientToConnect
	a.connected = true
	a.readOnlyMode = true
	a.inspections = session.NewInspectionSet(a.now)
	a.activeConnection = &ActiveConnection{
		SavedConnectionID: request.SavedConnectionID, Name: request.Name, Endpoint: request.Endpoint,
		SecurityPolicy: request.SecurityPolicy, SecurityMode: request.SecurityMode, AuthType: request.AuthType,
		Username: request.Username, ClientCertificatePath: request.ClientCertificatePath,
		ClientPrivateKeyPath: request.ClientPrivateKeyPath, ServerThumbprint: request.ServerThumbprint,
	}
	a.addressSpaceSearchSession = a.startAddressSpaceSearchSessionLocked()
	safety := a.sessionSafetyLocked()
	a.mu.Unlock()
	a.coordinator.Unlock()

	if reconnecting {
		if closeErr := a.closeClient(previousClient); closeErr != nil {
			a.appendLog("error", fmt.Sprintf("Closing previous OPC UA client after reconnect failed: %v", closeErr))
		}
	}
	a.emitSessionSafetyUpdated(safety)
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()
	a.appendLog("info", "Connected")
	return nil
}

// ActiveConnection returns the immutable non-secret connection snapshot for the
// current Troubleshooting Session.
func (a *Studio) ActiveConnection() (ActiveConnection, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeConnection == nil {
		return ActiveConnection{}, false
	}
	return *a.activeConnection, true
}

func (a *Studio) Disconnect() error {
	a.appendLog("info", "Disconnecting")

	a.coordinator.Lock()
	a.mu.Lock()
	if a.mutationExecutionPending {
		a.mu.Unlock()
		a.coordinator.Unlock()
		return ErrUnknownMutationInFlight
	}
	a.stopAddressSpaceSearchSessionLocked()
	clientToClose := a.client
	a.client = a.newClient()
	a.inspections = session.NewInspectionSet(a.now)
	a.activeConnection = nil
	a.mutationOutcomeUncertain = false
	a.connected = false
	a.readOnlyMode = true
	safety := a.sessionSafetyLocked()
	a.mu.Unlock()
	closeErr := a.closeClient(clientToClose)
	a.coordinator.Unlock()

	a.emitSessionSafetyUpdated(safety)
	a.emitInspection(nil)
	a.emitWatchlist()
	a.emitSessionTrendUpdated()

	if closeErr != nil {
		a.appendLog("error", fmt.Sprintf("Disconnect failed: %v", closeErr))
	}
	a.appendLog("info", "Disconnected")
	return nil
}

func (a *Studio) closeClient(client opcua.Client) error {
	closeCtx, cancelClose := a.operationContext(a.timeouts.Disconnect)
	defer cancelClose()

	closeDone := make(chan error, 1)
	go func() { closeDone <- client.Close(closeCtx) }()
	select {
	case err := <-closeDone:
		return err
	case <-closeCtx.Done():
		return fmt.Errorf("timed out closing OPC UA client after %s", a.timeouts.Disconnect)
	}
}

func (a *Studio) startAddressSpaceSearchSessionLocked() *search.Session {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	browser := deadlineBrowser{studio: a, client: a.client}
	var searchSession *search.Session
	if a.newAddressSpaceSearchSession != nil {
		searchSession = a.newAddressSpaceSearchSession(base, browser)
	} else {
		searchSession = search.NewSession(base, browser)
	}
	go a.consumeAddressSpaceSearchEvents(searchSession)
	return searchSession
}

func (a *Studio) consumeAddressSpaceSearchEvents(searchSession *search.Session) {
	for event := range searchSession.Events() {
		a.mu.Lock()
		isCurrentSession := a.addressSpaceSearchSession == searchSession
		a.mu.Unlock()
		if !isCurrentSession {
			continue
		}

		switch event := event.(type) {
		case search.BackgroundBrowseFailed:
			a.appendLog("error", fmt.Sprintf("Shallow Address Space Indexing browse failed for %s: %v", event.NodeID, event.Err))
		case search.PriorityQueueOverflow:
			a.appendLog("error", fmt.Sprintf("Shallow Address Space Indexing priority queue is full; %d parent nodes were not prioritized", event.DroppedParentCount))
		case search.IndexingBudgetExhausted:
			a.appendLog("info", fmt.Sprintf("Shallow Address Space Indexing reached its session budget after %d browse requests", event.BrowseCount))
		}
	}
}

func (a *Studio) stopAddressSpaceSearchSessionLocked() {
	if a.addressSpaceSearchSession != nil {
		a.addressSpaceSearchSession.Stop()
		a.addressSpaceSearchSession = nil
	}
}

func (a *Studio) BrowseChildren(nodeID string) ([]opcua.AddressNode, error) {
	if nodeID == "" {
		nodeID = "i=85"
	}
	a.appendLog("info", fmt.Sprintf("Browsing children of %s", nodeID))
	a.mu.Lock()
	searchSession := a.addressSpaceSearchSession
	a.mu.Unlock()
	if searchSession == nil {
		err := fmt.Errorf("connect to an OPC UA Server to browse Address Space children")
		a.appendLog("error", fmt.Sprintf("Browse failed for %s: %v", nodeID, err))
		return nil, err
	}
	children, err := searchSession.BrowseChildren(nodeID)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Browse failed for %s: %v", nodeID, err))
		return nil, err
	}
	a.appendLog("info", fmt.Sprintf("Browsed %d children of %s", len(children), nodeID))
	return children, nil
}

func (a *Studio) SearchAddressSpace(query string) (search.AddressSpaceSearchView, error) {
	a.mu.Lock()
	connected := a.connected
	searchSession := a.addressSpaceSearchSession
	a.mu.Unlock()
	if !connected || searchSession == nil {
		return search.AddressSpaceSearchView{Query: query, Results: []search.AddressSpaceSearchResult{}, Status: "Connect to an OPC UA Server to search browsed Address Space metadata."}, nil
	}
	return searchSession.Search(query), nil
}

func (a *Studio) GetMethodDetails(request MethodNodeRequest) (opcua.MethodDetails, error) {
	if strings.TrimSpace(request.ObjectNodeID) == "" || strings.TrimSpace(request.MethodNodeID) == "" {
		return opcua.MethodDetails{}, fmt.Errorf("Method inspection requires Object and Method Node IDs")
	}

	a.appendLog("info", fmt.Sprintf("Reading Method details for Object Node %s Method Node %s", request.ObjectNodeID, request.MethodNodeID))
	details, err := withClientRead(a, a.timeouts.Read, func(ctx context.Context, client opcua.Client) (opcua.MethodDetails, error) {
		a.mu.Lock()
		connected := a.connected
		a.mu.Unlock()
		if !connected {
			return opcua.MethodDetails{}, fmt.Errorf("Method inspection requires a connected session")
		}
		return client.ReadMethodDetails(ctx, request.ObjectNodeID, request.MethodNodeID)
	}, nil)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Reading Method details failed for Object Node %s Method Node %s: %v", request.ObjectNodeID, request.MethodNodeID, err))
		return opcua.MethodDetails{}, err
	}
	return details, nil
}

func (a *Studio) CallMethod(request MethodCallRequest) (opcua.MethodCallResult, error) {
	objectNodeID := strings.TrimSpace(request.ObjectNodeID)
	methodNodeID := strings.TrimSpace(request.MethodNodeID)
	if objectNodeID == "" || methodNodeID == "" {
		return opcua.MethodCallResult{}, fmt.Errorf("Method call requires Object and Method Node IDs")
	}

	a.appendLog("info", fmt.Sprintf("Method call attempted for Object Node %s Method Node %s", objectNodeID, methodNodeID))
	a.coordinator.Lock()
	defer a.coordinator.Unlock()
	ctx, cancel := a.operationContext(a.timeouts.MethodCall)
	defer cancel()

	a.mu.Lock()
	if a.mutationOutcomeUncertain {
		a.mu.Unlock()
		return opcua.MethodCallResult{}, ErrUnknownMutationInFlight
	}
	connected := a.connected
	readOnly := a.readOnlyMode
	client := a.client
	a.mu.Unlock()
	if !connected {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method call requires a connected session"))
	}
	if readOnly {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method call is blocked while Read-Only Mode is active"))
	}

	details, err := runOperation(ctx, func() (opcua.MethodDetails, error) {
		return client.ReadMethodDetails(ctx, objectNodeID, methodNodeID)
	})
	if err != nil {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("re-read Method metadata: %w", err))
	}
	if !details.Executable {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method is not executable"))
	}
	if !details.UserExecutable {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method is not executable for the current session"))
	}
	if len(request.InputArguments) != len(details.InputArguments) {
		return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method requires exactly %d input arguments, got %d", len(details.InputArguments), len(request.InputArguments)))
	}

	inputs := make([]opcua.ScalarValue, len(details.InputArguments))
	for i, argument := range details.InputArguments {
		if argument.ValueRank != "Scalar" {
			return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method input argument %d (%s) has unsupported non-scalar ValueRank %q", i+1, argument.Name, argument.ValueRank))
		}
		if !argument.Supported {
			return a.methodCallFailure(objectNodeID, methodNodeID, fmt.Errorf("Method input argument %d (%s) has unsupported DataType %q", i+1, argument.Name, argument.DataType))
		}
		inputs[i], err = opcua.ParseScalarValue(argument.DataType, request.InputArguments[i])
		if err != nil {
			parseErr := fmt.Errorf("parse Method input argument %d (%s): %w", i+1, argument.Name, err)
			a.appendLog("error", fmt.Sprintf("Method call failed for Object Node %s Method Node %s: input argument %d failed validation", objectNodeID, methodNodeID, i+1))
			return opcua.MethodCallResult{}, parseErr
		}
	}

	done := startOperation(func() (opcua.MethodCallResult, error) {
		return client.CallMethod(ctx, objectNodeID, methodNodeID, inputs)
	})
	var result opcua.MethodCallResult
	select {
	case completed := <-done:
		result, err = completed.value, completed.err
	case <-ctx.Done():
		a.appendLog("error", fmt.Sprintf("Method call failed for Object Node %s Method Node %s: transport or session failure", objectNodeID, methodNodeID))
		return opcua.MethodCallResult{}, pendingUnknownMutationOutcome(a, client, "Method Call", ctx.Err(), done)
	}
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Method call failed for Object Node %s Method Node %s: transport or session failure", objectNodeID, methodNodeID))
		return opcua.MethodCallResult{}, a.unknownMutationOutcome(client, "Method Call", err)
	}
	a.appendLog("info", fmt.Sprintf("Method call completed for Object Node %s Method Node %s StatusCode=%s", objectNodeID, methodNodeID, result.StatusCode))
	return result, nil
}

func (a *Studio) methodCallFailure(objectNodeID, methodNodeID string, err error) (opcua.MethodCallResult, error) {
	a.appendLog("error", fmt.Sprintf("Method call failed for Object Node %s Method Node %s: %v", objectNodeID, methodNodeID, err))
	return opcua.MethodCallResult{}, err
}

func pendingUnknownMutationOutcome[T any](a *Studio, client opcua.Client, operation string, err error, done <-chan operationResult[T]) error {
	a.mu.Lock()
	a.mutationOutcomeUncertain = true
	a.mutationExecutionPending = true
	a.readOnlyMode = true
	safety := a.sessionSafetyLocked()
	a.mu.Unlock()
	a.emitSessionSafetyUpdated(safety)
	go func() {
		<-done
		a.mu.Lock()
		if a.client != client {
			a.mu.Unlock()
			return
		}
		a.mutationExecutionPending = false
		// The outcome remains uncertain after completion; only a deliberate
		// re-enable of writes (or explicit disconnect) acknowledges it.
		a.mu.Unlock()
	}()
	return &UnknownMutationOutcomeError{Operation: operation, Err: err}
}

func (a *Studio) unknownMutationOutcome(client opcua.Client, operation string, err error) error {
	a.mu.Lock()
	if a.client != client {
		a.mu.Unlock()
		return &UnknownMutationOutcomeError{Operation: operation, Err: err}
	}
	a.mutationOutcomeUncertain = true
	a.readOnlyMode = true
	safety := a.sessionSafetyLocked()
	a.mu.Unlock()
	a.emitSessionSafetyUpdated(safety)
	return &UnknownMutationOutcomeError{Operation: operation, Err: err}
}

func (a *Studio) InspectVariableNode(node opcua.AddressNode) error {
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

func (a *Studio) ClearVariableNodeInspection() error {
	a.mu.Lock()
	requests := a.inspections.Unselect()
	a.mu.Unlock()
	a.emitInspection(nil)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *Studio) WatchVariableNode(node opcua.AddressNode) error {
	if node.NodeClass != "Variable" {
		return fmt.Errorf("only Variable Nodes can be added to the Watchlist")
	}
	a.appendLog("info", fmt.Sprintf("Adding Variable Node %s to Watchlist", node.NodeID))
	a.mu.Lock()
	if !a.inspections.IsWatched(node.NodeID) && len(a.inspections.Watched()) >= maxWatchlistNodes {
		a.mu.Unlock()
		return fmt.Errorf("Watchlist supports at most %d Variable Nodes", maxWatchlistNodes)
	}
	requests := a.inspections.Watch(node)
	rows := a.watchlistLocked()
	view := a.currentInspectionLocked()
	a.mu.Unlock()
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	a.executeInspectionRequests(requests)
	return nil
}

func (a *Studio) UnwatchVariableNode(nodeID string) error {
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

func (a *Studio) GetWatchlist() []WatchlistRowView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.watchlistLocked()
}

func (a *Studio) GetSessionTrend(focusedNodeID string) session.SessionTrendView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inspections.SessionTrend(focusedNodeID)
}

func (a *Studio) RefreshVariableNodeValue(nodeID string) error {
	if strings.TrimSpace(nodeID) == "" {
		a.mu.Lock()
		selected, ok := a.inspections.Selected()
		a.mu.Unlock()
		if !ok {
			return fmt.Errorf("no Variable Node selected")
		}
		nodeID = selected.Node.NodeID
	}
	a.appendLog("info", fmt.Sprintf("Refreshing Live Value for Variable Node %s", nodeID))
	value, err := withClientRead(a, a.timeouts.Read, func(ctx context.Context, client opcua.Client) (opcua.LiveValue, error) {
		return client.ReadValue(ctx, nodeID)
	}, nil)
	a.applyReadBack(nodeID, value, err)
	return err
}

func (a *Studio) WriteVariableNodeValue(request VariableNodeWriteRequest) (VariableNodeWriteResult, error) {
	nodeID := strings.TrimSpace(request.NodeID)
	if nodeID == "" {
		return VariableNodeWriteResult{}, fmt.Errorf("Variable Node Write requires a node ID")
	}
	a.appendLog("info", fmt.Sprintf("Variable Node Write attempted for %s", nodeID))
	a.coordinator.Lock()
	defer a.coordinator.Unlock()
	ctx, cancel := a.operationContext(a.timeouts.Write)
	defer cancel()

	a.mu.Lock()
	if a.mutationOutcomeUncertain {
		a.mu.Unlock()
		return VariableNodeWriteResult{}, ErrUnknownMutationInFlight
	}
	client := a.client
	a.mu.Unlock()
	inspection, err := a.prepareVariableNodeWrite(ctx, client, nodeID)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Variable Node Write failed for %s during validation", nodeID))
		return VariableNodeWriteResult{}, err
	}
	target, err := opcua.ParseScalarValue(inspection.Details.DataType, request.TargetValue)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Variable Node Write failed for %s during validation", nodeID))
		return VariableNodeWriteResult{}, err
	}
	done := startOperation(func() (struct{}, error) {
		return struct{}{}, client.WriteValue(ctx, nodeID, target)
	})
	select {
	case completed := <-done:
		err = completed.err
	case <-ctx.Done():
		a.appendLog("error", fmt.Sprintf("Variable Node Write failed for %s: transport or session failure", nodeID))
		return VariableNodeWriteResult{}, pendingUnknownMutationOutcome(a, client, "Variable Node Write", ctx.Err(), done)
	}
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Variable Node Write failed for %s: transport or session failure", nodeID))
		var rejected *opcua.WriteRejectedError
		if errors.As(err, &rejected) {
			return VariableNodeWriteResult{}, err
		}
		return VariableNodeWriteResult{}, a.unknownMutationOutcome(client, "Variable Node Write", err)
	}
	a.appendLog("info", fmt.Sprintf("Variable Node Write accepted for %s", nodeID))
	readBack, readErr := runOperation(ctx, func() (opcua.LiveValue, error) {
		return client.ReadValue(ctx, nodeID)
	})
	a.applyReadBack(nodeID, readBack, readErr)
	if readErr != nil {
		a.appendLog("error", fmt.Sprintf("Variable Node Write read-back failed for %s", nodeID))
		return VariableNodeWriteResult{}, readErr
	}
	result := VariableNodeWriteResult{NodeID: nodeID, TargetValue: target.Normalized, Status: "success", ReadBack: readBack}
	if !scalarReadBackMatches(target, readBack.Value) {
		result.Status = "warning"
		result.Warning = fmt.Sprintf("read-back mismatch: target %q but server returned %q", target.Normalized, readBack.Value)
	}
	return result, nil
}

func (a *Studio) prepareVariableNodeWrite(ctx context.Context, client opcua.Client, nodeID string) (session.VariableNodeInspection, error) {
	a.mu.Lock()
	connected := a.connected
	readOnly := a.readOnlyMode
	inspection, ok := a.inspections.Inspection(nodeID)
	a.mu.Unlock()
	if !connected {
		return session.VariableNodeInspection{}, fmt.Errorf("Variable Node Write requires a connected session")
	}
	if readOnly {
		return session.VariableNodeInspection{}, fmt.Errorf("Variable Node Write is blocked while Read-Only Mode is active")
	}
	if !ok {
		return session.VariableNodeInspection{}, fmt.Errorf("Variable Node Write requires an active Variable Node Inspection or Watchlist row")
	}
	if inspection.Stale || inspection.Err != nil || inspection.Value.NodeID == "" {
		return session.VariableNodeInspection{}, fmt.Errorf("Variable Node Write is blocked because the current Live Value is stale or unavailable")
	}
	details, err := runOperation(ctx, func() (opcua.NodeDetails, error) {
		return client.ReadNodeDetails(ctx, nodeID)
	})
	a.mu.Lock()
	a.inspections.ApplyDetails(nodeID, details, err)
	inspection, _ = a.inspections.Inspection(nodeID)
	view := a.currentInspectionLocked()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	if err != nil {
		return session.VariableNodeInspection{}, err
	}
	if err := validateVariableNodeWriteDetails(details); err != nil {
		return session.VariableNodeInspection{}, err
	}
	inspection.Details = details
	return inspection, nil
}

func validateVariableNodeWriteDetails(details opcua.NodeDetails) error {
	if details.ValueRank != "" && details.ValueRank != "Scalar" {
		return fmt.Errorf("Variable Node Write does not support arrays or non-scalar ValueRank %q", details.ValueRank)
	}
	if !details.Writable {
		return fmt.Errorf("Variable Node %s is not writable in this session", details.NodeID)
	}
	if _, err := opcua.ParseScalarValue(details.DataType, zeroValueForDataType(details.DataType)); err != nil {
		return err
	}
	return nil
}

func zeroValueForDataType(dataType string) string {
	if dataType == "Boolean" {
		return "false"
	}
	return "0"
}

func scalarReadBackMatches(target opcua.ScalarValue, readBack string) bool {
	parsed, err := opcua.ParseScalarValue(target.DataType, readBack)
	if err != nil {
		return false
	}
	return parsed.Normalized == target.Normalized
}

func (a *Studio) applyReadBack(nodeID string, value opcua.LiveValue, err error) {
	a.mu.Lock()
	requests := a.inspections.ApplyLiveValue(nodeID, value, err)
	view := a.currentInspectionLocked()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	if err != nil {
		a.appendLog("error", fmt.Sprintf("Refresh Live Value failed for %s: %v", nodeID, err))
	}
	a.emitInspection(view)
	a.emitWatchlistRows(rows)
	if err == nil {
		a.scheduleSessionTrendUpdate()
	}
	a.executeInspectionRequests(requests)
}

func (a *Studio) GetDiagnosticLogs() []DiagnosticLogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	logs := make([]DiagnosticLogEntry, len(a.logs))
	copy(logs, a.logs)
	return logs
}

func (a *Studio) executeInspectionRequests(requests []session.Request) {
	for _, request := range requests {
		req := request
		switch req.Kind {
		case session.RequestSubscribeValue:
			go a.subscribeValue(req.NodeID)
		case session.RequestReadDetails:
			go a.readNodeDetails(req.NodeID)
		case session.RequestCancelSubscription:
			go a.cancelValueSubscription(req.Subscription)
		}
	}
}

type subscriptionResult struct {
	updates      <-chan opcua.LiveValue
	subscription opcua.ValueSubscription
}

func (a *Studio) subscribeValue(nodeID string) {
	result, err := withClientRead(a, a.timeouts.Read, func(ctx context.Context, client opcua.Client) (subscriptionResult, error) {
		updates, subscription, err := client.SubscribeValue(ctx, nodeID)
		return subscriptionResult{updates: updates, subscription: subscription}, err
	}, func(late subscriptionResult) {
		a.cancelValueSubscription(late.subscription)
	})
	updates, subscription := result.updates, result.subscription
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

func (a *Studio) cancelValueSubscription(subscription opcua.ValueSubscription) {
	if subscription == nil {
		return
	}
	ctx, cancel := a.operationContext(a.timeouts.Read)
	defer cancel()
	_ = subscription.Cancel(ctx)
}

func (a *Studio) readNodeDetails(nodeID string) {
	details, err := withClientRead(a, a.timeouts.Read, func(ctx context.Context, client opcua.Client) (opcua.NodeDetails, error) {
		return client.ReadNodeDetails(ctx, nodeID)
	}, nil)
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

func (a *Studio) currentInspectionLocked() *VariableNodeInspectionView {
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

func (a *Studio) watchlistLocked() []WatchlistRowView {
	watched := a.inspections.Watched()
	rows := make([]WatchlistRowView, 0, len(watched))
	for _, inspection := range watched {
		rows = append(rows, watchlistRowView(inspection))
	}
	return rows
}

func (a *Studio) emitInspection(view *VariableNodeInspectionView) {
	a.publish(VariableInspectionUpdated{View: view})
}

func (a *Studio) emitWatchlist() {
	a.mu.Lock()
	rows := a.watchlistLocked()
	a.mu.Unlock()
	a.emitWatchlistRows(rows)
}

func (a *Studio) emitWatchlistRows(rows []WatchlistRowView) {
	a.publish(WatchlistUpdated{Rows: rows})
}

func (a *Studio) scheduleSessionTrendUpdate() {
	a.mu.Lock()
	if a.trendNotifyPending {
		a.mu.Unlock()
		return
	}
	a.trendNotifyPending = true
	a.mu.Unlock()
	a.afterFunc(250*time.Millisecond, func() {
		a.mu.Lock()
		a.trendNotifyPending = false
		a.mu.Unlock()
		a.emitSessionTrendUpdated()
	})
}

func (a *Studio) emitSessionTrendUpdated() {
	a.publish(SessionTrendUpdated{})
}

func (a *Studio) sessionSafetyLocked() SessionSafetyView {
	return SessionSafetyView{Connected: a.connected, ReadOnlyMode: a.readOnlyMode}
}

func (a *Studio) emitSessionSafetyUpdated(view SessionSafetyView) {
	a.publish(SessionSafetyUpdated{View: view})
}

func (a *Studio) appendLog(level, message string) {
	entry := DiagnosticLogEntry{Timestamp: a.now().Format(time.RFC3339), Level: level, Message: message}
	a.mu.Lock()
	a.logs = append(a.logs, entry)
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	logger := a.logger
	a.mu.Unlock()
	if logger != nil {
		logger(entry)
	}
	a.publish(DiagnosticLogAppended{Entry: entry})
}

func (a *Studio) publish(event Event) {
	a.mu.Lock()
	sink := a.eventSink
	a.mu.Unlock()
	if sink != nil {
		sink(event)
	}
}
