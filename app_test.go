package main

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
)

type recordingClient struct {
	mu              sync.Mutex
	connectErr      error
	connectRequests []opcua.ConnectRequest
	browseChildren  map[string][]opcua.AddressNode
	browseRequests  []string
	browseTimes     []time.Time
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

func (c *recordingClient) BrowseChildren(_ context.Context, nodeID string) ([]opcua.AddressNode, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.browseRequests = append(c.browseRequests, nodeID)
	c.browseTimes = append(c.browseTimes, time.Now())
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

func (c *recordingClient) ReadNodeDetails(context.Context, string) (opcua.NodeDetails, error) {
	return opcua.NodeDetails{}, nil
}

func (c *recordingClient) SubscribeValue(context.Context, string) (<-chan opcua.LiveValue, opcua.ValueSubscription, error) {
	return nil, nil, nil
}

func (c *recordingClient) Close(context.Context) error { return nil }

type blockingBrowseClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
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

	app := NewAppWithSavedConnectionStore(path)
	app.startup(nil)

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
	app := NewAppWithSavedConnectionStore(path)
	app.client = client
	app.startup(nil)

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

func TestConnectStartsShallowAddressSpaceIndexingFromObjectsNode(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}},
	}}
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
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
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.shallowIndexBrowseInterval = 10 * time.Millisecond
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
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.shallowIndexBrowseInterval = 50 * time.Millisecond
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

func TestExplicitBrowseAddsDiscoveredChildrenToSearchImmediately(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=ManualTemperature", DisplayName: "Manual Temperature", BrowseName: "2:ManualTemperature", NodeClass: "Variable"}},
	}}
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.shallowIndexBrowseInterval = time.Hour
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

func TestShallowAddressSpaceIndexingLimitsBrowseRequestsToDefaultRate(t *testing.T) {
	client := &recordingClient{browseChildren: map[string][]opcua.AddressNode{
		"i=85":         {{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"}},
		"ns=2;s=Area1": nil,
	}}
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
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
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
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

func TestReconnectCancelsPreviousShallowAddressSpaceIndexerAndClearsItsSearchMetadata(t *testing.T) {
	firstClient := &blockingBrowseClient{started: make(chan struct{}), release: make(chan struct{})}
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
	app.client = firstClient

	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://first.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	<-firstClient.started

	secondClient := &recordingClient{}
	app.client = secondClient
	if err := app.Connect(ConnectionRequest{Endpoint: "opc.tcp://second.local:4840", AuthType: opcua.AuthAnonymous}); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Disconnect() })
	close(firstClient.release)
	time.Sleep(20 * time.Millisecond)

	view, err := app.SearchAddressSpace("StalePump")
	if err != nil {
		t.Fatalf("SearchAddressSpace() error = %v", err)
	}
	if len(view.Results) != 0 {
		t.Fatalf("SearchAddressSpace() after reconnect = %#v, want previous indexer cancelled and metadata cleared", view)
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
	app := NewAppWithSavedConnectionStore(path)
	app.client = client
	app.startup(nil)

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

	app := NewAppWithSavedConnectionStore(path)
	app.startup(nil)

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

	app := NewAppWithSavedConnectionStore(path)
	app.connected = true
	app.startup(nil)

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
	app := NewAppWithSavedConnectionStore(t.TempDir() + "/saved-connections.json")
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
	app := NewAppWithSavedConnectionStore(path)
	app.client = client
	app.startup(nil)

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

	app := NewAppWithSavedConnectionStore(path)
	app.client = &recordingClient{connectErr: errors.New("dial failed")}
	app.startup(nil)

	if err := app.Connect(ConnectionRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous}); err == nil {
		t.Fatalf("Connect() error = nil, want failure")
	}

	saved := app.GetSavedConnections()
	if saved[0].LastConnectedAt != nil {
		t.Fatalf("LastConnectedAt = %s after failed connect, want nil", saved[0].LastConnectedAt)
	}
}
