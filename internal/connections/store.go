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

func (s *FileStore) write(saved []SavedConnection) error {
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
