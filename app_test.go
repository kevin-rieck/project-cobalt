package main

import (
	"testing"
	"time"

	"opcua-studio/internal/connections"
)

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
