package connections

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestStoreSavesAndReloadsSavedConnection(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	saved, err := store.Save(SaveRequest{
		Name:                        "Control Gateway",
		Endpoint:                    "opc.tcp://gateway.local:4840",
		SecurityPolicy:              "Basic256Sha256",
		SecurityMode:                "MessageSecurityModeSignAndEncrypt",
		AuthType:                    "UserName",
		Username:                    "engineer",
		Password:                    "do-not-store",
		ClientCertificatePath:       "C:/certs/client.crt",
		ClientPrivateKeyPath:        "C:/certs/client.key",
		ServerCertificateThumbprint: "ABC123",
	}, now)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := NewFileStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("Load() returned %d Saved Connections, want 1", len(reloaded))
	}
	got := reloaded[0]
	if got != saved {
		t.Fatalf("reloaded Saved Connection = %#v, want %#v", got, saved)
	}
	if got.CreatedAt != now || got.UpdatedAt != now {
		t.Fatalf("timestamps = %s/%s, want %s", got.CreatedAt, got.UpdatedAt, now)
	}
}

func TestStoreLoadsEmptyListFromMissingOrEmptyStorage(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)

	missing, err := store.Load()
	if err != nil {
		t.Fatalf("Load() missing storage error = %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("Load() missing storage returned %d Saved Connections, want 0", len(missing))
	}

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write empty storage: %v", err)
	}
	empty, err := store.Load()
	if err != nil {
		t.Fatalf("Load() empty storage error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("Load() empty storage returned %d Saved Connections, want 0", len(empty))
	}
}

func TestStoreNeverPersistsPassword(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	password := "super-secret-password"

	_, err := store.Save(SaveRequest{
		Name:     "Control Gateway",
		Endpoint: "opc.tcp://gateway.local:4840",
		AuthType: "UserName",
		Username: "engineer",
		Password: password,
	}, time.Now())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	serialized := string(contents)
	if strings.Contains(serialized, password) || strings.Contains(strings.ToLower(serialized), "password") {
		t.Fatalf("persisted storage contains password material: %s", serialized)
	}
}
