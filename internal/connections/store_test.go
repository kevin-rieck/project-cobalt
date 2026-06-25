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

func TestStoreRequiresSavedConnectionName(t *testing.T) {
	store := NewFileStore(t.TempDir() + "/saved-connections.json")

	_, err := store.Save(SaveRequest{Name: "  ", Endpoint: "opc.tcp://gateway.local:4840"}, time.Now())
	if err == nil {
		t.Fatalf("Save() error = nil, want name required")
	}
	if !strings.Contains(err.Error(), "Saved Connection name is required") {
		t.Fatalf("Save() error = %q, want clear name required message", err)
	}
}

func TestStoreUpdatesExistingSavedConnectionWhenNameDiffersOnlyByCase(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	createdAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	_, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: "Anonymous"}, createdAt)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	saved, err := store.Save(SaveRequest{Name: "control gateway", Endpoint: "opc.tcp://gateway.local:4841", AuthType: "UserName", Username: "engineer"}, updatedAt)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("Load() returned %d Saved Connections, want 1", len(reloaded))
	}
	if saved.CreatedAt != createdAt || saved.UpdatedAt != updatedAt {
		t.Fatalf("timestamps = %s/%s, want %s/%s", saved.CreatedAt, saved.UpdatedAt, createdAt, updatedAt)
	}
	if reloaded[0].Name != "control gateway" || reloaded[0].Endpoint != "opc.tcp://gateway.local:4841" || reloaded[0].Username != "engineer" {
		t.Fatalf("updated Saved Connection = %#v", reloaded[0])
	}
}

func TestStoreRenamesExistingSavedConnectionWithoutCreatingDuplicate(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	createdAt := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	_, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: "Anonymous"}, createdAt)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	saved, err := store.Save(SaveRequest{ExistingName: "Control Gateway", Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840", AuthType: "UserName", Username: "engineer"}, updatedAt)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("Load() returned %d Saved Connections, want 1", len(reloaded))
	}
	if saved.Name != "Packaging Line" || saved.CreatedAt != createdAt || saved.UpdatedAt != updatedAt {
		t.Fatalf("renamed Saved Connection = %#v", saved)
	}
	if reloaded[0].Name != "Packaging Line" || reloaded[0].Endpoint != "opc.tcp://packaging.local:4840" || reloaded[0].Username != "engineer" {
		t.Fatalf("reloaded Saved Connection = %#v", reloaded[0])
	}
}

func TestStoreRejectsRenamingSavedConnectionToAnotherSavedConnectionName(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	if _, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840"}, now); err != nil {
		t.Fatalf("seed first Saved Connection: %v", err)
	}
	if _, err := store.Save(SaveRequest{Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840"}, now); err != nil {
		t.Fatalf("seed second Saved Connection: %v", err)
	}

	_, err := store.Save(SaveRequest{ExistingName: "Control Gateway", Name: "packaging line", Endpoint: "opc.tcp://gateway.local:4841"}, now.Add(time.Hour))
	if err == nil {
		t.Fatalf("Save() error = nil, want duplicate name rejected")
	}
	if !strings.Contains(err.Error(), "Saved Connection name \"packaging line\" already exists") {
		t.Fatalf("Save() error = %q, want clear duplicate name message", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 2 || reloaded[0].Name != "Control Gateway" || reloaded[1].Name != "Packaging Line" {
		t.Fatalf("Saved Connections changed after rejected rename: %#v", reloaded)
	}
}

func TestStoreAllowsDifferentSavedConnectionNamesForSameEndpoint(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	if _, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840"}, now); err != nil {
		t.Fatalf("save first Saved Connection: %v", err)
	}
	if _, err := store.Save(SaveRequest{Name: "Line 2 Gateway", Endpoint: "opc.tcp://gateway.local:4840"}, now.Add(time.Hour)); err != nil {
		t.Fatalf("save second Saved Connection with same endpoint: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("Load() returned %d Saved Connections, want 2", len(reloaded))
	}
}

func TestStoreDeletesSavedConnectionByID(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	controlGateway, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840"}, now)
	if err != nil {
		t.Fatalf("seed first Saved Connection: %v", err)
	}
	packagingLine, err := store.Save(SaveRequest{Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840"}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("seed second Saved Connection: %v", err)
	}

	deleted, err := store.Delete(controlGateway.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatalf("Delete() deleted = false, want true")
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].ID != packagingLine.ID || reloaded[0].Name != "Packaging Line" {
		t.Fatalf("Saved Connections after delete = %#v, want only Packaging Line", reloaded)
	}
}

func TestStoreDeletingMissingSavedConnectionIDLeavesStorageUnchanged(t *testing.T) {
	path := t.TempDir() + "/saved-connections.json"
	store := NewFileStore(path)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	saved, err := store.Save(SaveRequest{Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840"}, now)
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}

	deleted, err := store.Delete("missing-id")
	if err != nil {
		t.Fatalf("Delete() missing id error = %v", err)
	}
	if deleted {
		t.Fatalf("Delete() missing id deleted = true, want false")
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded) != 1 || reloaded[0] != saved {
		t.Fatalf("Saved Connections changed after missing delete: %#v, want %#v", reloaded, saved)
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
