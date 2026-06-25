package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
)

type recordingClient struct {
	connectErr      error
	connectRequests []opcua.ConnectRequest
}

func (c *recordingClient) DiscoverEndpoints(context.Context, string) ([]opcua.Endpoint, error) {
	return nil, nil
}

func (c *recordingClient) Connect(_ context.Context, request opcua.ConnectRequest) error {
	c.connectRequests = append(c.connectRequests, request)
	return c.connectErr
}

func (c *recordingClient) BrowseChildren(context.Context, string) ([]opcua.AddressNode, error) {
	return nil, nil
}

func (c *recordingClient) ReadNodeDetails(context.Context, string) (opcua.NodeDetails, error) {
	return opcua.NodeDetails{}, nil
}

func (c *recordingClient) SubscribeValue(context.Context, string) (<-chan opcua.LiveValue, opcua.ValueSubscription, error) {
	return nil, nil, nil
}

func (c *recordingClient) Close(context.Context) error { return nil }

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
