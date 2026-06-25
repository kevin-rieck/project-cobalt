package connections

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SavedConnection is the persisted, non-secret information needed to reconnect
// to an OPC UA Server.
type SavedConnection struct {
	ID                          string     `json:"id"`
	Name                        string     `json:"name"`
	Endpoint                    string     `json:"endpoint"`
	SecurityPolicy              string     `json:"securityPolicy"`
	SecurityMode                string     `json:"securityMode"`
	AuthType                    string     `json:"authType"`
	Username                    string     `json:"username,omitempty"`
	ClientCertificatePath       string     `json:"clientCertificatePath,omitempty"`
	ClientPrivateKeyPath        string     `json:"clientPrivateKeyPath,omitempty"`
	ServerCertificateThumbprint string     `json:"serverCertificateThumbprint,omitempty"`
	CreatedAt                   time.Time  `json:"createdAt"`
	UpdatedAt                   time.Time  `json:"updatedAt"`
	LastConnectedAt             *time.Time `json:"lastConnectedAt,omitempty"`
}

// SaveRequest accepts connection details from the app boundary. Secret fields
// are intentionally accepted here only so the store can ignore them at the
// persistence boundary.
type SaveRequest struct {
	// ExistingName identifies the Saved Connection being edited. It is empty
	// when creating or overwriting by the requested name.
	ExistingName                string
	Name                        string
	Endpoint                    string
	SecurityPolicy              string
	SecurityMode                string
	AuthType                    string
	Username                    string
	Password                    string
	ClientCertificatePath       string
	ClientPrivateKeyPath        string
	ServerCertificateThumbprint string
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func DefaultStorePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return "saved-connections.json"
	}
	return filepath.Join(configDir, "OPC UA Studio", "saved-connections.json")
}

func (s *FileStore) Load() ([]SavedConnection, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []SavedConnection{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return []SavedConnection{}, nil
	}
	var saved []SavedConnection
	if err := json.Unmarshal(contents, &saved); err != nil {
		return nil, err
	}
	if saved == nil {
		return []SavedConnection{}, nil
	}
	ensureSavedConnectionIDs(saved)
	return saved, nil
}

func (s *FileStore) Save(request SaveRequest, now time.Time) (SavedConnection, error) {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return SavedConnection{}, fmt.Errorf("Saved Connection name is required")
	}
	existing, err := s.Load()
	if err != nil {
		return SavedConnection{}, err
	}
	saved := SavedConnection{
		ID:                          savedConnectionID(request.Name, now),
		Name:                        request.Name,
		Endpoint:                    request.Endpoint,
		SecurityPolicy:              request.SecurityPolicy,
		SecurityMode:                request.SecurityMode,
		AuthType:                    request.AuthType,
		Username:                    request.Username,
		ClientCertificatePath:       request.ClientCertificatePath,
		ClientPrivateKeyPath:        request.ClientPrivateKeyPath,
		ServerCertificateThumbprint: request.ServerCertificateThumbprint,
		CreatedAt:                   now,
		UpdatedAt:                   now,
	}
	targetName := strings.TrimSpace(request.ExistingName)
	targetIndex := -1
	for i, candidate := range existing {
		if targetName != "" && strings.EqualFold(candidate.Name, targetName) {
			targetIndex = i
			break
		}
		if targetName == "" && strings.EqualFold(candidate.Name, saved.Name) {
			targetIndex = i
			break
		}
	}
	for i, candidate := range existing {
		if i != targetIndex && strings.EqualFold(candidate.Name, saved.Name) {
			return SavedConnection{}, fmt.Errorf("Saved Connection name %q already exists", saved.Name)
		}
	}
	if targetIndex >= 0 {
		candidate := existing[targetIndex]
		saved.ID = candidate.ID
		saved.CreatedAt = candidate.CreatedAt
		saved.LastConnectedAt = candidate.LastConnectedAt
		existing[targetIndex] = saved
		return saved, s.write(existing)
	}
	existing = append(existing, saved)
	return saved, s.write(existing)
}

func (s *FileStore) MarkConnected(name string, now time.Time) (SavedConnection, bool, error) {
	saved, err := s.Load()
	if err != nil {
		return SavedConnection{}, false, err
	}
	for i := range saved {
		if saved[i].Name == name {
			saved[i].LastConnectedAt = &now
			saved[i].UpdatedAt = now
			return saved[i], true, s.write(saved)
		}
	}
	return SavedConnection{}, false, nil
}

func (s *FileStore) Delete(id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	saved, err := s.Load()
	if err != nil {
		return false, err
	}
	for i, candidate := range saved {
		if candidate.ID == id {
			remaining := append(saved[:i], saved[i+1:]...)
			return true, s.write(remaining)
		}
	}
	return false, nil
}

func (s *FileStore) write(saved []SavedConnection) error {
	ensureSavedConnectionIDs(saved)
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(s.path, contents, 0o600)
}

func ensureSavedConnectionIDs(saved []SavedConnection) {
	used := map[string]bool{}
	for i := range saved {
		id := strings.TrimSpace(saved[i].ID)
		if id == "" {
			id = savedConnectionID(saved[i].Name, saved[i].CreatedAt)
		}
		base := id
		for suffix := 2; used[id]; suffix++ {
			id = fmt.Sprintf("%s-%d", base, suffix)
		}
		saved[i].ID = id
		used[id] = true
	}
}

func savedConnectionID(name string, createdAt time.Time) string {
	var slug strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			slug.WriteRune(r)
		case r == '-' || r == '_':
			slug.WriteRune(r)
		case r == ' ' && slug.Len() > 0:
			slug.WriteRune('-')
		}
	}
	if slug.Len() == 0 {
		slug.WriteString("saved-connection")
	}
	return fmt.Sprintf("%s-%d", slug.String(), createdAt.UTC().UnixNano())
}
