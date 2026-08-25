package application

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
	"opcua-studio/internal/search"
)

func configureSearchSession(app *Studio, options search.SessionOptions) {
	app.newAddressSpaceSearchSession = func(ctx context.Context, browser search.Browser) *search.Session {
		return search.NewSession(ctx, browser, options)
	}
}

type recordingClient struct {
	mu                   sync.Mutex
	connectErr           error
	closeErr             error
	connectRequests      []opcua.ConnectRequest
	browseChildren       map[string][]opcua.AddressNode
	browseErrors         map[string]error
	browseRequests       []string
	browseTimes          []time.Time
	browseContexts       []context.Context
	readValues           map[string]opcua.LiveValue
	readValueErrors      map[string]error
	readValueIDs         []string
	readDetails          map[string]opcua.NodeDetails
	readDetailErrors     map[string]error
	methodDetails        map[string]opcua.MethodDetails
	methodDetailErrors   map[string]error
	methodDetailRequests []MethodNodeRequest
	writeErrors          map[string]error
	writeRequests        []recordedWrite
	methodCallResult     opcua.MethodCallResult
	methodCallErr        error
	methodCallRequests   []recordedMethodCall
	methodCallStarted    chan struct{}
	methodCallRelease    <-chan struct{}
}

type recordedWrite struct {
	nodeID string
	value  opcua.ScalarValue
}

type recordedMethodCall struct {
	objectNodeID string
	methodNodeID string
	inputs       []opcua.ScalarValue
}

func (c *recordingClient) DiscoverEndpoints(context.Context, string) ([]opcua.Endpoint, error) {
	return nil, nil
}

func (c *recordingClient) Connect(_ context.Context, request opcua.ConnectRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectRequests = append(c.connectRequests, request)
	return c.connectErr
}

func (c *recordingClient) BrowseChildren(ctx context.Context, nodeID string) ([]opcua.AddressNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.browseRequests = append(c.browseRequests, nodeID)
	c.browseTimes = append(c.browseTimes, time.Now())
	c.browseContexts = append(c.browseContexts, ctx)
	if err := c.browseErrors[nodeID]; err != nil {
		return nil, err
	}
	if c.browseChildren == nil {
		return nil, nil
	}
	return c.browseChildren[nodeID], nil
}

func (c *recordingClient) recordedBrowseRequests() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	requests := make([]string, len(c.browseRequests))
	copy(requests, c.browseRequests)
	return requests
}

func (c *recordingClient) recordedBrowseTimes() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	times := make([]time.Time, len(c.browseTimes))
	copy(times, c.browseTimes)
	return times
}

func (c *recordingClient) recordedBrowseContexts() []context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	contexts := make([]context.Context, len(c.browseContexts))
	copy(contexts, c.browseContexts)
	return contexts
}

func (c *recordingClient) ReadNodeDetails(_ context.Context, nodeID string) (opcua.NodeDetails, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.readDetailErrors[nodeID]; err != nil {
		return opcua.NodeDetails{}, err
	}
	if c.readDetails == nil {
		return opcua.NodeDetails{}, nil
	}
	return c.readDetails[nodeID], nil
}

func (c *recordingClient) ReadMethodDetails(_ context.Context, objectNodeID, methodNodeID string) (opcua.MethodDetails, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	request := MethodNodeRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID}
	c.methodDetailRequests = append(c.methodDetailRequests, request)
	if err := c.methodDetailErrors[objectNodeID+"\x00"+methodNodeID]; err != nil {
		return opcua.MethodDetails{}, err
	}
	return c.methodDetails[objectNodeID+"\x00"+methodNodeID], nil
}

func (c *recordingClient) ReadValue(_ context.Context, nodeID string) (opcua.LiveValue, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readValueIDs = append(c.readValueIDs, nodeID)
	if err := c.readValueErrors[nodeID]; err != nil {
		return opcua.LiveValue{}, err
	}
	if c.readValues == nil {
		return opcua.LiveValue{NodeID: nodeID}, nil
	}
	return c.readValues[nodeID], nil
}

func (c *recordingClient) WriteValue(_ context.Context, nodeID string, value opcua.ScalarValue) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeRequests = append(c.writeRequests, recordedWrite{nodeID: nodeID, value: value})
	return c.writeErrors[nodeID]
}

func (c *recordingClient) CallMethod(_ context.Context, objectNodeID, methodNodeID string, inputs []opcua.ScalarValue) (opcua.MethodCallResult, error) {
	c.mu.Lock()
	copiedInputs := append([]opcua.ScalarValue{}, inputs...)
	c.methodCallRequests = append(c.methodCallRequests, recordedMethodCall{objectNodeID: objectNodeID, methodNodeID: methodNodeID, inputs: copiedInputs})
	result, err := c.methodCallResult, c.methodCallErr
	started, release := c.methodCallStarted, c.methodCallRelease
	c.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		<-release
	}
	return result, err
}

func (c *recordingClient) SubscribeValue(context.Context, string) (<-chan opcua.LiveValue, opcua.ValueSubscription, error) {
	return nil, nil, nil
}

func (c *recordingClient) Close(context.Context) error { return c.closeErr }

type blockingBrowseClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
}

type blockingCloseClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
}

type blockingConnectClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
}

func (c *blockingConnectClient) Connect(ctx context.Context, request opcua.ConnectRequest) error {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-c.release
	return c.recordingClient.Connect(ctx, request)
}

func (c *blockingCloseClient) Close(context.Context) error {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-c.release
	return nil
}

func (c *blockingBrowseClient) BrowseChildren(context.Context, string) ([]opcua.AddressNode, error) {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-c.release
	return []opcua.AddressNode{{NodeID: "ns=2;s=StalePump", DisplayName: "Stale Pump", BrowseName: "2:StalePump", NodeClass: "Variable"}}, nil
}

func TestAppLoadsSavedConnectionsOnStartup(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	_, err := store.Save(connections.SaveRequest{
		Name:     "Control Gateway",
		Endpoint: "opc.tcp://gateway.local:4840",
		AuthType: "Anonymous",
	}, time.Now())
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	app := NewStudioWithSavedConnectionStore(path)
	app.Start(nil)

	saved := app.GetSavedConnections()
	if len(saved) != 1 {
		t.Fatalf("GetSavedConnections() returned %d Saved Connections, want 1", len(saved))
	}
	if saved[0].Name != "Control Gateway" || saved[0].Endpoint != "opc.tcp://gateway.local:4840" {
		t.Fatalf("loaded Saved Connection = %#v", saved[0])
	}
}

func TestStartupWithCorruptSavedConnectionStorageLogsDiagnosticAndManualConnectStillWorks(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	corruptStorage := []byte(`[{"name":"Control Gateway",`)
	if err := os.WriteFile(path, corruptStorage, 0o600); err != nil {
		t.Fatalf("write corrupt storage: %v", err)
	}

	client := &recordingClient{}
	app := NewStudioWithSavedConnectionStore(path)
	app.client = client
	app.Start(nil)

	logs := app.GetDiagnosticLogs()
	if len(logs) == 0 || !strings.Contains(logs[0].Message, "Saved Connection storage is not valid JSON") {
		t.Fatalf("diagnostic logs = %#v, want corrupt Saved Connection storage diagnostic", logs)
	}
	if saved := app.GetSavedConnections(); len(saved) != 0 {
		t.Fatalf("GetSavedConnections() = %#v, want no loaded Saved Connections", saved)
	}

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://manual.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("manual Connect() error = %v", err)
	}
	if len(client.connectRequests) != 1 || client.connectRequests[0].Endpoint != "opc.tcp://manual.local:4840" {
		t.Fatalf("manual Connect() requests = %#v", client.connectRequests)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != string(corruptStorage) {
		t.Fatalf("corrupt storage was changed to %q, want original %q", contents, corruptStorage)
	}
}

func TestSessionSafetyDefaultsToReadOnlyAndRequiresConnectedSessionToAllowWrites(t *testing.T) {
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")

	safety := app.GetSessionSafety()
	if safety.Connected || !safety.ReadOnlyMode {
		t.Fatalf("initial GetSessionSafety() = %#v, want disconnected Read-Only Mode", safety)
	}
	if err := app.SetReadOnlyMode(false); err == nil || !strings.Contains(err.Error(), "connected session") {
		t.Fatalf("SetReadOnlyMode(false) while disconnected error = %v, want connected session error", err)
	}
}

func TestSessionSafetyResetsToReadOnlyOnConnectAndDisconnect(t *testing.T) {
	client := &recordingClient{}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if safety := app.GetSessionSafety(); !safety.Connected || !safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after Connect = %#v, want connected Read-Only Mode", safety)
	}
	if err := app.SetReadOnlyMode(false); err != nil {
		t.Fatalf("SetReadOnlyMode(false) error = %v", err)
	}
	if safety := app.GetSessionSafety(); !safety.Connected || safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after Allow Writes = %#v, want writes allowed", safety)
	}

	if err := app.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if safety := app.GetSessionSafety(); safety.Connected || !safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after Disconnect = %#v, want disconnected Read-Only Mode", safety)
	}
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })
	if safety := app.GetSessionSafety(); !safety.Connected || !safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after reconnect = %#v, want new session Read-Only Mode", safety)
	}
}

func TestDisconnectWhenClientCloseFailsStillDisconnectsApp(t *testing.T) {
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	client := &recordingClient{closeErr: errors.New("network timeout")}
	app.client = client
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	_ = app.Disconnect()

	if safety := app.GetSessionSafety(); safety.Connected {
		t.Fatalf("GetSessionSafety().Connected = true, want false after Disconnect even if Close failed")
	}
}

func TestDisconnectReturnsWhenClientCloseBlocks(t *testing.T) {
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.timeouts.Disconnect = 10 * time.Millisecond
	client := &blockingCloseClient{started: make(chan struct{}), release: make(chan struct{})}
	app.client = client
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- app.Disconnect() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Disconnect() error = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Disconnect() hung waiting for client Close")
	}
	select {
	case <-client.started:
	default:
		t.Fatal("Disconnect() did not attempt to close the previous OPC UA client")
	}
	if safety := app.GetSessionSafety(); safety.Connected {
		t.Fatalf("GetSessionSafety().Connected = true, want false after Disconnect even if Close blocks")
	}
	close(client.release)
}

func TestReenablingReadOnlyModeRecordsDiagnosticLog(t *testing.T) {
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = &recordingClient{}
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })
	if err := app.SetReadOnlyMode(false); err != nil {
		t.Fatalf("SetReadOnlyMode(false) error = %v", err)
	}

	if err := app.SetReadOnlyMode(true); err != nil {
		t.Fatalf("SetReadOnlyMode(true) error = %v", err)
	}

	for _, entry := range app.GetDiagnosticLogs() {
		if entry.Level == "info" && strings.Contains(entry.Message, "Read-Only Mode enabled") {
			return
		}
	}
	t.Fatalf("diagnostic logs = %#v, want Read-Only Mode enabled entry", app.GetDiagnosticLogs())
}

func TestConnectStartsShallowAddressSpaceIndexingFromObjectsNode(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Pump")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 && view.Results[0].Node.NodeID == "ns=2;s=PumpA" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Pump")
	t.Fatalf("SearchAddressSpace(\"Pump\") = %#v, want shallow-indexed Variable Node", view)
}

func TestShallowAddressSpaceIndexingAddsSearchableNodeClassesButOnlyRecursesThroughParentNodes(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {
			{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"},
			{NodeID: "ns=2;s=Temperature", DisplayName: "Filler Temperature", BrowseName: "2:FillerTemperature", NodeClass: "Variable"},
			{NodeID: "ns=2;s=Reset", DisplayName: "Reset", BrowseName: "2:Reset", NodeClass: "Method"},
			{NodeID: "", DisplayName: "Nameless", BrowseName: "2:Nameless", NodeClass: "Object"},
		},
		"ns=2;s=Area1":       {{NodeID: "ns=2;s=Pressure", DisplayName: "Bowl Pressure", BrowseName: "2:BowlPressure", NodeClass: "Variable"}},
		"ns=2;s=Temperature": {{NodeID: "ns=2;s=ShouldNotBrowse", DisplayName: "Should Not Browse", BrowseName: "2:ShouldNotBrowse", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Pressure")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, query := range []string{"Area", "Temperature", "Reset", "Pressure"} {
		view, err := app.SearchAddressSpace(query)
		if err != nil {
			t.Fatalf("SearchAddressSpace(%q) error = %v", query, err)
		}
		if len(view.Results) != 1 {
			t.Fatalf("SearchAddressSpace(%q) = %#v, want one indexed Search Result", query, view)
		}
	}
	view, err := app.SearchAddressSpace("Nameless")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 0 {
		t.Fatalf("SearchAddressSpace(\"Nameless\") = %#v, want nodes without valid NodeIDs excluded", view)
	}
	for _, request := range client.recordedBrowseRequests() {
		if request == "ns=2;s=Temperature" {
			t.Fatalf("Shallow Address Space Indexing browsed through Variable Node; requests = %#v", client.recordedBrowseRequests())
		}
	}
}

func TestExplicitBrowsePrioritizesDiscoveredParentNodesAheadOfBackgroundIndexing(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85":                  {{NodeID: "ns=2;s=BackgroundArea", DisplayName: "Background Area", BrowseName: "2:BackgroundArea", NodeClass: "Object"}},
		"ns=2;s=ManualArea":     {{NodeID: "ns=2;s=ManualSkid", DisplayName: "Manual Skid", BrowseName: "2:ManualSkid", NodeClass: "Object"}},
		"ns=2;s=ManualSkid":     {{NodeID: "ns=2;s=ManualPressure", DisplayName: "Manual Pressure", BrowseName: "2:ManualPressure", NodeClass: "Variable"}},
		"ns=2;s=BackgroundArea": {{NodeID: "ns=2;s=BackgroundPressure", DisplayName: "Background Pressure", BrowseName: "2:BackgroundPressure", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 50 * time.Millisecond})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if requests := client.recordedBrowseRequests(); len(requests) >= 1 && requests[0] == "i=85" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if _, err := app.BrowseChildren("ns=2;s=ManualArea"); err != nil {
		t.Fatalf("BrowseChildren() error = %v", err)
	}

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		requests := client.recordedBrowseRequests()
		for i, request := range requests {
			if request == "ns=2;s=ManualArea" && i+1 < len(requests) {
				if requests[i+1] != "ns=2;s=ManualSkid" {
					t.Fatalf("request after explicit BrowseChildren = %q, want prioritized ManualSkid; requests = %#v", requests[i+1], requests)
				}
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("browse requests = %#v, want ManualSkid indexed after explicit BrowseChildren", client.recordedBrowseRequests())
}

func TestExplicitBrowseFailureReturnsErrorAndRecordsDiagnostic(t *testing.T) {
	client := &recordingClient{browseErrors: map[string]error{"ns=2;s=BadArea": errors.New("access denied")}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	_, err := app.BrowseChildren("ns=2;s=BadArea")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("BrowseChildren() error = %v, want explicit browse error", err)
	}
	for _, entry := range app.GetDiagnosticLogs() {
		if strings.Contains(entry.Message, "Browse failed for ns=2;s=BadArea") && strings.Contains(entry.Message, "access denied") {
			return
		}
	}
	t.Fatalf("diagnostic logs = %#v, want explicit browse failure diagnostic", app.GetDiagnosticLogs())
}

func TestExplicitBrowseAddsDiscoveredChildrenToSearchImmediately(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=ManualTemperature", DisplayName: "Manual Temperature", BrowseName: "2:ManualTemperature", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: time.Hour})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	if _, err := app.BrowseChildren("ns=2;s=ManualArea"); err != nil {
		t.Fatalf("BrowseChildren() error = %v", err)
	}
	view, err := app.SearchAddressSpace("Manual Temperature")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 1 || view.Results[0].Node.NodeID != "ns=2;s=ManualTemperature" {
		t.Fatalf("SearchAddressSpace() = %#v, want explicitly browsed child immediately searchable", view)
	}
}

func TestGetMethodDetailsRequiresConnectionButIsAllowedInReadOnlyMode(t *testing.T) {
	request := MethodNodeRequest{ObjectNodeID: "ns=3;s=Demo.CTT.Methods", MethodNodeID: "ns=3;s=Demo.CTT.Methods.MethodIO"}
	details := opcua.MethodDetails{
		ObjectNodeID:    request.ObjectNodeID,
		MethodNodeID:    request.MethodNodeID,
		Description:     "Adds 2 unsigned integers",
		Executable:      true,
		UserExecutable:  true,
		InputArguments:  []opcua.MethodArgument{{Name: "Summand1", DataType: "UInt32"}, {Name: "Summand2", DataType: "UInt32"}},
		OutputArguments: []opcua.MethodArgument{{Name: "Sum", DataType: "UInt32"}},
	}
	client := &recordingClient{methodDetails: map[string]opcua.MethodDetails{request.ObjectNodeID + "\x00" + request.MethodNodeID: details}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if _, err := app.GetMethodDetails(request); err == nil || !strings.Contains(err.Error(), "connected session") {
		t.Fatalf("GetMethodDetails() disconnected error = %v, want connected session error", err)
	}
	app.connected = true
	app.readOnlyMode = true

	got, err := app.GetMethodDetails(request)
	if err != nil {
		t.Fatalf("GetMethodDetails() in Read-Only Mode error = %v", err)
	}
	if got.MethodNodeID != details.MethodNodeID || len(got.InputArguments) != 2 || got.OutputArguments[0].Name != "Sum" {
		t.Fatalf("GetMethodDetails() = %#v, want MethodIO metadata", got)
	}
	if len(client.methodDetailRequests) != 1 || client.methodDetailRequests[0] != request {
		t.Fatalf("ReadMethodDetails requests = %#v, want %#v", client.methodDetailRequests, request)
	}
}

func TestCallMethodRevalidatesMetadataAndForwardsTypedInputs(t *testing.T) {
	const objectNodeID = "ns=3;s=Methods"
	const methodNodeID = "ns=3;s=MethodIO"
	key := objectNodeID + "\x00" + methodNodeID
	details := opcua.MethodDetails{
		ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID, Executable: true, UserExecutable: true,
		InputArguments: []opcua.MethodArgument{
			{Name: "Summand1", DataType: "UInt32", ValueRank: "Scalar", Supported: true},
			{Name: "Summand2", DataType: "UInt32", ValueRank: "Scalar", Supported: true},
		},
	}
	wantResult := opcua.MethodCallResult{
		StatusCode: "StatusGood (0x0)", InputArgumentResults: []string{"StatusGood (0x0)", "StatusGood (0x0)"},
		OutputArguments: []opcua.MethodArgumentValue{{DataType: "UInt32", Value: "uint32(585987)"}},
	}
	client := &recordingClient{methodDetails: map[string]opcua.MethodDetails{key: details}, methodCallResult: wantResult}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.connected = true
	app.readOnlyMode = false

	got, err := app.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID, InputArguments: []string{"314159", "271828"}})
	if err != nil {
		t.Fatalf("CallMethod() error = %v", err)
	}
	if got.StatusCode != wantResult.StatusCode || len(got.OutputArguments) != 1 || got.OutputArguments[0].Value != "uint32(585987)" {
		t.Fatalf("CallMethod() = %#v, want %#v", got, wantResult)
	}
	if len(client.methodDetailRequests) != 1 {
		t.Fatalf("ReadMethodDetails requests = %#v, want fresh metadata read", client.methodDetailRequests)
	}
	if len(client.methodCallRequests) != 1 {
		t.Fatalf("CallMethod requests = %#v, want one call", client.methodCallRequests)
	}
	call := client.methodCallRequests[0]
	if call.objectNodeID != objectNodeID || call.methodNodeID != methodNodeID || len(call.inputs) != 2 || call.inputs[0].Value != uint32(314159) || call.inputs[1].Value != uint32(271828) {
		t.Fatalf("forwarded Method call = %#v, want typed UInt32 inputs", call)
	}
	for _, entry := range app.GetDiagnosticLogs() {
		if strings.Contains(entry.Message, "314159") || strings.Contains(entry.Message, "271828") {
			t.Fatalf("diagnostic log leaked argument value: %q", entry.Message)
		}
	}
}

func TestCallMethodSafetyGatesPreventExecution(t *testing.T) {
	const objectNodeID = "ns=2;s=Object"
	const methodNodeID = "ns=2;s=Method"
	valid := opcua.MethodDetails{Executable: true, UserExecutable: true, InputArguments: []opcua.MethodArgument{{Name: "Value", DataType: "UInt32", ValueRank: "Scalar", Supported: true}}}
	tests := []struct {
		name      string
		connected bool
		readOnly  bool
		details   opcua.MethodDetails
		inputs    []string
		want      string
	}{
		{name: "disconnected", details: valid, inputs: []string{"42"}, want: "connected session"},
		{name: "Read-Only Mode", connected: true, readOnly: true, details: valid, inputs: []string{"42"}, want: "Read-Only Mode"},
		{name: "not executable", connected: true, details: func() opcua.MethodDetails { d := valid; d.Executable = false; return d }(), inputs: []string{"42"}, want: "not executable"},
		{name: "not user executable", connected: true, details: func() opcua.MethodDetails { d := valid; d.UserExecutable = false; return d }(), inputs: []string{"42"}, want: "not executable"},
		{name: "argument count", connected: true, details: valid, inputs: nil, want: "exactly 1 input"},
		{name: "unsupported type", connected: true, details: opcua.MethodDetails{Executable: true, UserExecutable: true, InputArguments: []opcua.MethodArgument{{DataType: "DateTime", ValueRank: "Scalar"}}}, inputs: []string{"now"}, want: "unsupported DataType"},
		{name: "array", connected: true, details: opcua.MethodDetails{Executable: true, UserExecutable: true, InputArguments: []opcua.MethodArgument{{DataType: "UInt32", ValueRank: "One-dimensional array"}}}, inputs: []string{"42"}, want: "non-scalar"},
		{name: "invalid value", connected: true, details: valid, inputs: []string{"314159-secret"}, want: "invalid UInt32"},
		{name: "underflow", connected: true, details: opcua.MethodDetails{Executable: true, UserExecutable: true, InputArguments: []opcua.MethodArgument{{Name: "Value", DataType: "Double", ValueRank: "Scalar", Supported: true}}}, inputs: []string{"1e-999"}, want: "invalid Double"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := objectNodeID + "\x00" + methodNodeID
			client := &recordingClient{methodDetails: map[string]opcua.MethodDetails{key: tt.details}}
			app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
			app.client = client
			app.connected = tt.connected
			app.readOnlyMode = tt.readOnly

			_, err := app.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID, InputArguments: tt.inputs})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("CallMethod() error = %v, want containing %q", err, tt.want)
			}
			if len(client.methodCallRequests) != 0 {
				t.Fatalf("CallMethod requests = %#v, want safety gate to prevent execution", client.methodCallRequests)
			}
			for _, input := range tt.inputs {
				for _, entry := range app.GetDiagnosticLogs() {
					if input != "" && strings.Contains(entry.Message, input) {
						t.Fatalf("diagnostic log leaked rejected argument value: %q", entry.Message)
					}
				}
			}
		})
	}
}

func TestCallMethodSerializesReadOnlyTransitionWithExecution(t *testing.T) {
	const objectNodeID = "ns=2;s=Object"
	const methodNodeID = "ns=2;s=Method"
	key := objectNodeID + "\x00" + methodNodeID
	started := make(chan struct{})
	release := make(chan struct{})
	client := &recordingClient{
		methodDetails:     map[string]opcua.MethodDetails{key: {Executable: true, UserExecutable: true}},
		methodCallStarted: started,
		methodCallRelease: release,
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.connected = true
	app.readOnlyMode = false

	callDone := make(chan error, 1)
	go func() {
		_, err := app.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID})
		callDone <- err
	}()
	<-started

	modeChangeDone := make(chan error, 1)
	go func() { modeChangeDone <- app.SetReadOnlyMode(true) }()
	select {
	case err := <-modeChangeDone:
		t.Fatalf("SetReadOnlyMode(true) completed during Method execution: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-callDone; err != nil {
		t.Fatalf("CallMethod() error = %v", err)
	}
	if err := <-modeChangeDone; err != nil {
		t.Fatalf("SetReadOnlyMode(true) error = %v", err)
	}
	if safety := app.GetSessionSafety(); !safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() = %#v, want Read-Only Mode after execution", safety)
	}
}

func TestCallMethodReturnsNonGoodStatusAndReportsRedactedTransportFailure(t *testing.T) {
	const objectNodeID = "ns=2;s=Object"
	const methodNodeID = "ns=2;s=Method"
	key := objectNodeID + "\x00" + methodNodeID
	details := opcua.MethodDetails{Executable: true, UserExecutable: true, InputArguments: []opcua.MethodArgument{{DataType: "String", ValueRank: "Scalar", Supported: true}}}
	client := &recordingClient{
		methodDetails:    map[string]opcua.MethodDetails{key: details},
		methodCallResult: opcua.MethodCallResult{StatusCode: "BadInvalidArgument (0x80AB0000)", InputArgumentResults: []string{"BadTypeMismatch (0x80740000)"}},
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.connected = true
	app.readOnlyMode = false

	result, err := app.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID, InputArguments: []string{"TOP-SECRET"}})
	if err != nil || result.StatusCode != "BadInvalidArgument (0x80AB0000)" {
		t.Fatalf("CallMethod() = (%#v, %v), want displayable non-Good result", result, err)
	}

	client.methodCallErr = errors.New("transport unavailable for TOP-SECRET")
	_, err = app.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID, InputArguments: []string{"TOP-SECRET"}})
	if err == nil || !strings.Contains(err.Error(), "transport unavailable for TOP-SECRET") {
		t.Fatalf("CallMethod() transport error = %v", err)
	}
	for _, entry := range app.GetDiagnosticLogs() {
		if strings.Contains(entry.Message, "TOP-SECRET") {
			t.Fatalf("diagnostic log leaked argument value: %q", entry.Message)
		}
	}
}

func TestRefreshVariableNodeValueUpdatesSelectedInspectionAndWatchlist(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	client := &recordingClient{readValues: map[string]opcua.LiveValue{
		"ns=2;s=Level": {NodeID: "ns=2;s=Level", Value: "120", Status: "Good"},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.inspections.Select(node)
	app.inspections.Watch(node)
	app.inspections.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "80", Status: "Good"}, nil)
	app.inspections.ApplyDetails("ns=2;s=Level", opcua.NodeDetails{NodeID: "ns=2;s=Level", EURange: &opcua.ValueRange{Low: 0, High: 100}}, nil)

	if err := app.RefreshVariableNodeValue(""); err != nil {
		t.Fatalf("RefreshVariableNodeValue() error = %v", err)
	}

	selected, ok := app.inspections.Selected()
	if !ok {
		t.Fatalf("selected inspection missing after refresh")
	}
	if selected.Value.Value != "120" || selected.Value.Status != "Good" || selected.Stale || selected.UpdateCount != 2 || selected.OutOfRange != "120 is above 100" || selected.Err != nil {
		t.Fatalf("selected inspection after refresh = %#v", selected)
	}
	rows := app.GetWatchlist()
	if len(rows) != 1 || rows[0].Value.Value != "120" || rows[0].OutOfRange != "120 is above 100" || rows[0].UpdateCount != 2 {
		t.Fatalf("watchlist rows after refresh = %#v", rows)
	}
	trend := app.GetSessionTrend("ns=2;s=Level")
	if len(trend.Points) != 2 || trend.Points[0].Value != "120" {
		t.Fatalf("Session Trend after refresh = %#v, want refreshed value appended for watched node", trend)
	}
}

func TestRefreshVariableNodeValueFailureMarksStaleAndRecordsInlineError(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	client := &recordingClient{readValueErrors: map[string]error{"ns=2;s=Level": errors.New("read denied")}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.inspections.Select(node)
	app.inspections.Watch(node)
	app.inspections.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "80", Status: "Good"}, nil)

	err := app.RefreshVariableNodeValue("ns=2;s=Level")
	if err == nil || !strings.Contains(err.Error(), "read denied") {
		t.Fatalf("RefreshVariableNodeValue() error = %v, want read denied", err)
	}

	selected, _ := app.inspections.Selected()
	if !selected.Stale || selected.Value.Value != "80" || selected.Err == nil || !strings.Contains(selected.Err.Error(), "read denied") {
		t.Fatalf("selected inspection after failed refresh = %#v", selected)
	}
	rows := app.GetWatchlist()
	if len(rows) != 1 || !rows[0].Stale || !strings.Contains(rows[0].Error, "read denied") || rows[0].Value.Value != "80" {
		t.Fatalf("watchlist rows after failed refresh = %#v", rows)
	}
	trend := app.GetSessionTrend("ns=2;s=Level")
	if len(trend.Points) != 1 || trend.Points[0].Value != "80" {
		t.Fatalf("Session Trend after failed refresh = %#v, want no failed read point appended", trend)
	}
}

func TestWriteVariableNodeValueWritesTypedScalarAndRefreshesReadBack(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	client := &recordingClient{
		readDetails: map[string]opcua.NodeDetails{"ns=2;s=Level": {NodeID: "ns=2;s=Level", DataType: "Double", Writable: true, ValueRank: "Scalar"}},
		readValues:  map[string]opcua.LiveValue{"ns=2;s=Level": {NodeID: "ns=2;s=Level", Value: "42", Status: "Good"}},
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.connected = true
	app.readOnlyMode = false
	app.inspections.Select(node)
	app.inspections.Watch(node)
	app.inspections.ApplyDetails("ns=2;s=Level", opcua.NodeDetails{NodeID: "ns=2;s=Level", DataType: "Double", Writable: true, ValueRank: "Scalar"}, nil)
	app.inspections.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "40", Status: "Good"}, nil)

	result, err := app.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: "ns=2;s=Level", TargetValue: "42"})
	if err != nil {
		t.Fatalf("WriteVariableNodeValue() error = %v", err)
	}
	if result.Status != "success" || result.TargetValue != "42" || result.ReadBack.Value != "42" || result.Warning != "" {
		t.Fatalf("WriteVariableNodeValue() result = %#v, want success with read-back", result)
	}
	if len(client.writeRequests) != 1 || client.writeRequests[0].nodeID != "ns=2;s=Level" || client.writeRequests[0].value.Value != float64(42) {
		t.Fatalf("write requests = %#v, want typed Double write", client.writeRequests)
	}
	selected, _ := app.inspections.Selected()
	if selected.Value.Value != "42" || selected.UpdateCount != 2 || selected.Stale {
		t.Fatalf("selected inspection after write = %#v", selected)
	}
}

func TestWriteVariableNodeValueRejectsFloatUnderflowBeforeWriting(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	details := opcua.NodeDetails{NodeID: node.NodeID, DataType: "Float", Writable: true, ValueRank: "Scalar"}
	client := &recordingClient{readDetails: map[string]opcua.NodeDetails{node.NodeID: details}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client
	app.connected = true
	app.readOnlyMode = false
	app.inspections.Select(node)
	app.inspections.ApplyDetails(node.NodeID, details, nil)
	app.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "1", Status: "Good"}, nil)

	_, err := app.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "1e-50"})
	if err == nil || !strings.Contains(err.Error(), "invalid Float") {
		t.Fatalf("WriteVariableNodeValue() error = %v, want invalid Float", err)
	}
	if len(client.writeRequests) != 0 {
		t.Fatalf("write requests = %#v, want underflow rejected before client write", client.writeRequests)
	}
}

func TestWriteVariableNodeValueRejectsReadOnlyStaleUnsupportedAndNotWritable(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	tests := []struct {
		name     string
		readOnly bool
		details  opcua.NodeDetails
		stale    bool
		want     string
	}{
		{name: "read-only", readOnly: true, details: opcua.NodeDetails{NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}, want: "Read-Only Mode"},
		{name: "stale", details: opcua.NodeDetails{NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}, stale: true, want: "stale"},
		{name: "unsupported", details: opcua.NodeDetails{NodeID: node.NodeID, DataType: "DateTime", Writable: true, ValueRank: "Scalar"}, want: "unsupported"},
		{name: "array", details: opcua.NodeDetails{NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "OneDimension"}, want: "arrays or non-scalar"},
		{name: "not writable", details: opcua.NodeDetails{NodeID: node.NodeID, DataType: "Double", Writable: false, ValueRank: "Scalar"}, want: "not writable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
			app.client = &recordingClient{readDetails: map[string]opcua.NodeDetails{node.NodeID: tt.details}}
			app.connected = true
			app.readOnlyMode = tt.readOnly
			app.inspections.Select(node)
			app.inspections.ApplyDetails(node.NodeID, tt.details, nil)
			app.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)
			if tt.stale {
				app.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{}, errors.New("read failed"))
			}

			_, err := app.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("WriteVariableNodeValue() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestWriteVariableNodeValueReportsFailureAndReadBackMismatch(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", DisplayName: "Tank Level", NodeClass: "Variable"}
	details := opcua.NodeDetails{NodeID: node.NodeID, DataType: "Int32", Writable: true, ValueRank: "Scalar"}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = &recordingClient{readDetails: map[string]opcua.NodeDetails{node.NodeID: details}, readValues: map[string]opcua.LiveValue{node.NodeID: {NodeID: node.NodeID, Value: "41", Status: "Good"}}}
	app.connected = true
	app.readOnlyMode = false
	app.inspections.Select(node)
	app.inspections.ApplyDetails(node.NodeID, details, nil)
	app.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	result, err := app.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
	if err != nil {
		t.Fatalf("WriteVariableNodeValue() mismatch error = %v", err)
	}
	if result.Status != "warning" || !strings.Contains(result.Warning, "read-back mismatch") {
		t.Fatalf("mismatch result = %#v, want warning", result)
	}

	failing := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	failing.client = &recordingClient{readDetails: map[string]opcua.NodeDetails{node.NodeID: details}, writeErrors: map[string]error{node.NodeID: errors.New("write denied")}}
	failing.connected = true
	failing.readOnlyMode = false
	failing.inspections.Select(node)
	failing.inspections.ApplyDetails(node.NodeID, details, nil)
	failing.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)
	if _, err := failing.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"}); err == nil || !strings.Contains(err.Error(), "write denied") {
		t.Fatalf("WriteVariableNodeValue() failure error = %v, want write denied", err)
	}
}

func TestBackgroundShallowAddressSpaceIndexingBrowseFailureRecordsDiagnostic(t *testing.T) {
	client := &recordingClient{
		browseErrors: map[string]error{"i=85": errors.New("access denied")},
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, entry := range app.GetDiagnosticLogs() {
			if strings.Contains(entry.Message, "Shallow Address Space Indexing browse failed for i=85") && strings.Contains(entry.Message, "access denied") {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("diagnostic logs = %#v, want background browse failure with parent node identity and error", app.GetDiagnosticLogs())
}

func TestBackgroundShallowAddressSpaceIndexingDoesNotRetryFailedParentInLoop(t *testing.T) {
	client := &recordingClient{
		browseChildren: map[string][]opcua.AddressNode{
			"i=85": {{NodeID: "ns=2;s=BadArea", DisplayName: "Bad Area", BrowseName: "2:BadArea", NodeClass: "Object"}},
		},
		browseErrors: map[string]error{"ns=2;s=BadArea": errors.New("access denied")},
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		requests := client.recordedBrowseRequests()
		if countBrowseRequests(requests, "ns=2;s=BadArea") == 1 {
			time.Sleep(50 * time.Millisecond)
			requests = client.recordedBrowseRequests()
			if got := countBrowseRequests(requests, "ns=2;s=BadArea"); got != 1 {
				t.Fatalf("browse requests = %#v, want failed parent browsed once", requests)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("browse requests = %#v, want failed parent attempted", client.recordedBrowseRequests())
}

func countBrowseRequests(requests []string, nodeID string) int {
	count := 0
	for _, request := range requests {
		if request == nodeID {
			count++
		}
	}
	return count
}

func TestBackgroundShallowAddressSpaceIndexingContinuesAfterBrowseFailure(t *testing.T) {
	client := &recordingClient{
		browseChildren: map[string][]opcua.AddressNode{
			"i=85": {
				{NodeID: "ns=2;s=BadArea", DisplayName: "Bad Area", BrowseName: "2:BadArea", NodeClass: "Object"},
				{NodeID: "ns=2;s=GoodArea", DisplayName: "Good Area", BrowseName: "2:GoodArea", NodeClass: "Object"},
			},
			"ns=2;s=GoodArea": {{NodeID: "ns=2;s=GoodPressure", DisplayName: "Good Pressure", BrowseName: "2:GoodPressure", NodeClass: "Variable"}},
		},
		browseErrors: map[string]error{"ns=2;s=BadArea": errors.New("access denied")},
	}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Good Pressure")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 && view.Results[0].Node.NodeID == "ns=2;s=GoodPressure" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Good Pressure")
	t.Fatalf("SearchAddressSpace() = %#v, want indexing to continue after failed parent", view)
}

func TestAddressSpaceSearchStatusMentionsBrowsedAndShallowIndexedMetadata(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Pump")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 {
			if !strings.Contains(view.Status, "browsed and shallow-indexed Address Space metadata") {
				t.Fatalf("SearchAddressSpace() status = %q, want browsed and shallow-indexed metadata message", view.Status)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Pump")
	t.Fatalf("SearchAddressSpace() = %#v, want shallow-indexed result", view)
}

func TestAddressSpaceSearchStatusShowsIndexedCoverageIsExpanding(t *testing.T) {
	client := &blockingBrowseClient{started: make(chan struct{}), release: make(chan struct{})}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	<-client.started
	defer func() {
		close(client.release)
		_ = app.Disconnect()
	}()

	view, err := app.SearchAddressSpace("Pump")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if !strings.Contains(view.Status, "indexed coverage is still expanding") {
		t.Fatalf("SearchAddressSpace() status = %q, want indexed coverage expanding message", view.Status)
	}
}

func TestAddressSpaceSearchStatusShowsIndexingBudgetExhausted(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond, BrowseBudget: 1})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Missing")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if strings.Contains(view.Status, "some Address Space areas may not be indexed") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Missing")
	t.Fatalf("SearchAddressSpace() status = %q, want indexing budget exhausted message", view.Status)
}

func TestShallowAddressSpaceIndexingStopsAtSessionBudget(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85":         {{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"}},
		"ns=2;s=Area1": {{NodeID: "ns=2;s=Area2", DisplayName: "Area 2", BrowseName: "2:Area2", NodeClass: "Object"}},
		"ns=2;s=Area2": {{NodeID: "ns=2;s=Area3", DisplayName: "Area 3", BrowseName: "2:Area3", NodeClass: "Object"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond, BrowseBudget: 2})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(client.recordedBrowseRequests()) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	requests := client.recordedBrowseRequests()
	if len(requests) != 2 || requests[0] != "i=85" || requests[1] != "ns=2;s=Area1" {
		t.Fatalf("browse requests = %#v, want exactly two background-indexed parent nodes", requests)
	}
}

func TestExplicitBrowseWorksAfterShallowAddressSpaceIndexingBudgetIsExhausted(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85":              {{NodeID: "ns=2;s=ManualArea", DisplayName: "Manual Area", BrowseName: "2:ManualArea", NodeClass: "Object"}},
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=ManualTemperature", DisplayName: "Manual Temperature", BrowseName: "2:ManualTemperature", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond, BrowseBudget: 1})
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(client.recordedBrowseRequests()) < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := app.BrowseChildren("ns=2;s=ManualArea"); err != nil {
		t.Fatalf("BrowseChildren() after budget exhaustion error = %v", err)
	}
	view, err := app.SearchAddressSpace("Manual Temperature")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 1 || view.Results[0].Node.NodeID != "ns=2;s=ManualTemperature" {
		t.Fatalf("SearchAddressSpace() = %#v, want explicitly browsed child after budget exhaustion", view)
	}
}

func TestReconnectResetsShallowAddressSpaceIndexingBudget(t *testing.T) {
	firstClient := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=FirstArea", DisplayName: "First Area", BrowseName: "2:FirstArea", NodeClass: "Object"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond, BrowseBudget: 1})
	app.client = firstClient

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://first.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(firstClient.recordedBrowseRequests()) < 1 {
		time.Sleep(10 * time.Millisecond)
	}
	if requests := firstClient.recordedBrowseRequests(); len(requests) != 1 {
		t.Fatalf("first session browse requests = %#v, want exhausted one-node budget", requests)
	}

	secondClient := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=SecondPump", DisplayName: "Second Pump", BrowseName: "2:SecondPump", NodeClass: "Variable"}},
	}}
	app.newClient = func() opcua.Client { return secondClient }
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://second.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Second Pump")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 && view.Results[0].Node.NodeID == "ns=2;s=SecondPump" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Second Pump")
	t.Fatalf("SearchAddressSpace() after reconnect = %#v, want budget reset for new session", view)
}

func TestShallowAddressSpaceIndexingLimitsBrowseRequestsToDefaultRate(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85":         {{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"}},
		"ns=2;s=Area1": nil,
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) && len(client.recordedBrowseTimes()) < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	times := client.recordedBrowseTimes()
	if len(times) < 2 {
		t.Fatalf("recorded browse times = %#v, want two browse requests", times)
	}
	if elapsed := times[1].Sub(times[0]); elapsed < 950*time.Millisecond {
		t.Fatalf("second Browse request started after %s, want about one per second", elapsed)
	}
}

func TestDisconnectClearsSessionLocalShallowIndexedSearchMetadata(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, _ := app.SearchAddressSpace("Pump")
		if len(view.Results) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := app.Disconnect(); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	view, err := app.SearchAddressSpace("Pump")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 0 || !strings.Contains(view.Status, "Connect to an OPC UA Server") {
		t.Fatalf("SearchAddressSpace() after Disconnect = %#v, want cleared disconnected search metadata", view)
	}
}

func TestSuccessfulReconnectReplacesAddressSpaceSearchSessionOnce(t *testing.T) {
	firstClient := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=FirstPump", DisplayName: "First Pump", BrowseName: "2:FirstPump", NodeClass: "Variable"}},
	}}
	secondClient := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=SecondPump", DisplayName: "Second Pump", BrowseName: "2:SecondPump", NodeClass: "Variable"}},
	}}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = firstClient
	app.newClient = func() opcua.Client { return secondClient }
	configureSearchSession(app, search.SessionOptions{BrowseInterval: 10 * time.Millisecond, BrowseBudget: 1})

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://first.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(firstClient.recordedBrowseRequests()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://second.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && len(secondClient.recordedBrowseRequests()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if requests := secondClient.recordedBrowseRequests(); len(requests) != 1 || requests[0] != "i=85" {
		t.Fatalf("replacement client browse requests = %#v, want one replacement session root browse", requests)
	}
	firstBrowseContexts := firstClient.recordedBrowseContexts()
	if len(firstBrowseContexts) != 1 {
		t.Fatalf("previous client browse contexts = %d, want one", len(firstBrowseContexts))
	}
	select {
	case <-firstBrowseContexts[0].Done():
	default:
		t.Fatal("previous Address Space Search session context remains active after reconnect")
	}
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		view, err := app.SearchAddressSpace("Pump")
		if err != nil {
			t.Fatalf("SearchAddressSpace() error = %v", err)
		}
		if len(view.Results) == 1 && view.Results[0].Node.NodeID == "ns=2;s=SecondPump" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	view, _ := app.SearchAddressSpace("Pump")
	t.Fatalf("SearchAddressSpace() after successful reconnect = %#v, want only replacement metadata", view)
}

func TestFailedReconnectPreservesExistingAddressSpaceSearchSession(t *testing.T) {
	firstClient := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"ns=2;s=Area": {{NodeID: "ns=2;s=ExistingPump", DisplayName: "Existing Pump", BrowseName: "2:ExistingPump", NodeClass: "Variable"}},
	}}
	failedClient := &recordingClient{connectErr: errors.New("dial failed")}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = firstClient
	app.newClient = func() opcua.Client { return failedClient }
	configureSearchSession(app, search.SessionOptions{BrowseInterval: time.Hour})

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://first.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	if _, err := app.BrowseChildren("ns=2;s=Area"); err != nil {
		t.Fatalf("BrowseChildren() before reconnect error = %v", err)
	}
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://failed.local:4840", AuthType: opcua.AuthAnonymous}); err == nil {
		t.Fatal("second Connect() error = nil, want reconnect failure")
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	if safety := app.GetSessionSafety(); !safety.Connected {
		t.Fatalf("GetSessionSafety() after failed reconnect = %#v, want existing connection preserved", safety)
	}
	view, err := app.SearchAddressSpace("Existing Pump")
	if err != nil {
		t.Fatalf("SearchAddressSpace() after failed reconnect error = %v", err)
	}
	if len(view.Results) != 1 || view.Results[0].Node.NodeID != "ns=2;s=ExistingPump" {
		t.Fatalf("SearchAddressSpace() after failed reconnect = %#v, want existing session preserved", view)
	}
	if _, err := app.BrowseChildren("ns=2;s=Area"); err != nil {
		t.Fatalf("BrowseChildren() after failed reconnect error = %v", err)
	}
	if requests := failedClient.recordedBrowseRequests(); len(requests) != 0 {
		t.Fatalf("failed replacement client browse requests = %#v, want none", requests)
	}
}

func TestReconnectWaitsForBoundedBackgroundBrowseOnPreviousClient(t *testing.T) {
	firstClient := &blockingBrowseClient{started: make(chan struct{}), release: make(chan struct{})}
	secondClient := &blockingConnectClient{started: make(chan struct{}), release: make(chan struct{})}
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = firstClient
	app.newClient = func() opcua.Client { return secondClient }

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://first.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	<-firstClient.started

	reconnectDone := make(chan error, 1)
	go func() {
		reconnectDone <- app.Connect(ConnectionRequest{Endpoint: "opc.tcp://second.local:4840", AuthType: opcua.AuthAnonymous})
	}()
	select {
	case <-secondClient.started:
		t.Fatal("replacement Connect started before the previous bounded browse completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(firstClient.release)
	<-secondClient.started

	if requests := secondClient.recordedBrowseRequests(); len(requests) != 0 {
		t.Fatalf("replacement client browsed while Connect was in progress: %#v", requests)
	}
	close(secondClient.release)
	if err := <-reconnectDone; err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })

	view, err := app.SearchAddressSpace("StalePump")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 0 {
		t.Fatalf("SearchAddressSpace() after reconnect = %#v, want previous metadata cleared", view)
	}
}

func TestConnectWithSavedConnectionUpdatesLastConnectedTimeAfterSuccessfulConnect(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	createdAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	_, err := store.Save(connections.SaveRequest{
		Name:     "Control Gateway",
		Endpoint: "opc.tcp://gateway.local:4840",
		AuthType: string(opcua.AuthUsername),
		Username: "engineer",
	}, createdAt)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	client := &recordingClient{}
	app := NewStudioWithSavedConnectionStore(path)
	app.client = client
	app.Start(nil)

	if err := app.Connect(ConnectionRequest{
		Name:           "Control Gateway",
		Endpoint:       "opc.tcp://gateway.local:4840",
		AuthType:       opcua.AuthUsername,
		Username:       "engineer",
		Password:       "entered-at-connect-time",
		SecurityPolicy: "None",
		SecurityMode:   "MessageSecurityModeNone",
	}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if len(client.connectRequests) != 1 || client.connectRequests[0].Password != "entered-at-connect-time" {
		t.Fatalf("Connect() passed request %#v, want password entered at connect time", client.connectRequests)
	}
	saved := app.GetSavedConnections()
	if len(saved) != 1 {
		t.Fatalf("GetSavedConnections() returned %d Saved Connections, want 1", len(saved))
	}
	if saved[0].LastConnectedAt == nil {
		t.Fatalf("LastConnectedAt is nil after successful Saved Connection connect")
	}
	if !saved[0].LastConnectedAt.After(createdAt) {
		t.Fatalf("LastConnectedAt = %s, want after %s", saved[0].LastConnectedAt, createdAt)
	}
}

func TestSaveSavedConnectionEditsExistingSavedConnection(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	_, err := store.Save(connections.SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: string(opcua.AuthAnonymous)}, time.Now())
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	app := NewStudioWithSavedConnectionStore(path)
	app.Start(nil)

	_, err = app.SaveSavedConnection(ConnectionRequest{ExistingName: "Control Gateway", Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840", AuthType: opcua.AuthUsername, Username: "engineer"})
	if err != nil {
		t.Fatalf("SaveSavedConnection() error = %v", err)
	}

	saved := app.GetSavedConnections()
	if len(saved) != 1 {
		t.Fatalf("GetSavedConnections() returned %d Saved Connections, want 1", len(saved))
	}
	if saved[0].Name != "Packaging Line" || saved[0].Endpoint != "opc.tcp://packaging.local:4840" || saved[0].Username != "engineer" {
		t.Fatalf("edited Saved Connection = %#v", saved[0])
	}
}

func TestDeleteSavedConnectionRemovesOnlyThatSavedConnection(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	controlGateway, err := store.Save(connections.SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: string(opcua.AuthAnonymous)}, now)
	if err != nil {
		t.Fatalf("seed first Saved Connection: %v", err)
	}
	if _, err := store.Save(connections.SaveRequest{Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840", AuthType: string(opcua.AuthAnonymous)}, now.Add(time.Hour)); err != nil {
		t.Fatalf("seed second Saved Connection: %v", err)
	}

	app := NewStudioWithSavedConnectionStore(path)
	app.connected = true
	app.Start(nil)

	deleted, err := app.DeleteSavedConnection(controlGateway.ID)
	if err != nil {
		t.Fatalf("DeleteSavedConnection() error = %v", err)
	}
	if !deleted {
		t.Fatalf("DeleteSavedConnection() deleted = false, want true")
	}

	if !app.connected {
		t.Fatalf("DeleteSavedConnection() changed current connection state")
	}
	saved := app.GetSavedConnections()
	if len(saved) != 1 || saved[0].Name != "Packaging Line" {
		t.Fatalf("GetSavedConnections() after delete = %#v, want only Packaging Line", saved)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Name != "Packaging Line" {
		t.Fatalf("persisted Saved Connections after delete = %#v, want only Packaging Line", reloaded)
	}
}

func TestUsernameConnectRequiresPasswordEntry(t *testing.T) {
	app := NewStudioWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	client := &recordingClient{}
	app.client = client

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthUsername, Username: "engineer"}); err == nil {
		t.Fatalf("Connect() error = nil, want password required")
	}
	if len(client.connectRequests) != 0 {
		t.Fatalf("Connect() called client with missing password: %#v", client.connectRequests)
	}
}

func TestConnectWithRenamedUnsavedSavedConnectionUpdatesSelectedSavedConnection(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	createdAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	savedConnection, err := store.Save(connections.SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: string(opcua.AuthAnonymous)}, createdAt)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	client := &recordingClient{}
	app := NewStudioWithSavedConnectionStore(path)
	app.client = client
	app.Start(nil)

	if err := app.Connect(ConnectionRequest{SavedConnectionID: savedConnection.ID, Name: "Unsaved Rename", Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	saved := app.GetSavedConnections()
	if len(saved) != 1 {
		t.Fatalf("GetSavedConnections() returned %d Saved Connections, want 1", len(saved))
	}
	if saved[0].ID != savedConnection.ID || saved[0].Name != "Control Gateway" {
		t.Fatalf("updated Saved Connection = %#v, want original selected record", saved[0])
	}
	if saved[0].LastConnectedAt == nil {
		t.Fatalf("LastConnectedAt is nil after successful connect from selected Saved Connection")
	}
}

func TestFailedConnectWithSavedConnectionDoesNotUpdateLastConnectedTime(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := connections.NewFileStore(path)
	createdAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	_, err := store.Save(connections.SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: string(opcua.AuthAnonymous)}, createdAt)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	app := NewStudioWithSavedConnectionStore(path)
	app.client = &recordingClient{connectErr: errors.New("dial failed")}
	app.Start(nil)

	if err := app.Connect(ConnectionRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err == nil {
		t.Fatalf("Connect() error = nil, want failure")
	}

	saved := app.GetSavedConnections()
	if saved[0].LastConnectedAt != nil {
		t.Fatalf("LastConnectedAt = %s after failed connect, want nil", saved[0].LastConnectedAt)
	}
}
