package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"` // e.g., https://myteam.slack.com
}

type Data struct {
	DefaultWorkspace string               `json:"default_workspace,omitempty"`
	Workspaces       map[string]Workspace `json:"workspaces"`
}

// FileSystem abstracts file I/O for testability.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Exists(path string) bool
}

// Store manages workspace configuration on disk.
type Store struct {
	fs      FileSystem
	dirPath string
}

func NewStore(fs FileSystem, dirPath string) *Store {
	return &Store{fs: fs, dirPath: dirPath}
}

func DefaultDirPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "slack-pipe")
}

func (s *Store) filePath() string {
	return filepath.Join(s.dirPath, "workspaces.json")
}

func (s *Store) Load() (*Data, error) {
	if err := s.fs.MkdirAll(s.dirPath, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if !s.fs.Exists(s.filePath()) {
		return &Data{Workspaces: make(map[string]Workspace)}, nil
	}
	raw, err := s.fs.ReadFile(s.filePath())
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if data.Workspaces == nil {
		data.Workspaces = make(map[string]Workspace)
	}
	return &data, nil
}

func (s *Store) Save(data *Data) error {
	if err := s.fs.MkdirAll(s.dirPath, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return s.fs.WriteFile(s.filePath(), raw, 0o600)
}

func (s *Store) AddWorkspace(ws Workspace) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	data.Workspaces[ws.ID] = ws
	if data.DefaultWorkspace == "" {
		data.DefaultWorkspace = ws.ID
	}
	return s.Save(data)
}

func (s *Store) RemoveWorkspace(id string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := data.Workspaces[id]; !ok {
		return ErrWorkspaceNotFound
	}
	delete(data.Workspaces, id)
	if data.DefaultWorkspace == id {
		data.DefaultWorkspace = ""
		// Pick deterministic next default (sorted by ID) to avoid map iteration randomness
		var ids []string
		for nextID := range data.Workspaces {
			ids = append(ids, nextID)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			data.DefaultWorkspace = ids[0]
		}
	}
	return s.Save(data)
}

func (s *Store) SetDefault(id string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := data.Workspaces[id]; !ok {
		return ErrWorkspaceNotFound
	}
	data.DefaultWorkspace = id
	return s.Save(data)
}

func (s *Store) GetWorkspace(identifier string) (*Workspace, error) {
	data, err := s.Load()
	if err != nil {
		return nil, err
	}
	// If no identifier, use default
	if identifier == "" {
		if data.DefaultWorkspace == "" {
			return nil, ErrWorkspaceNotFound
		}
		ws, ok := data.Workspaces[data.DefaultWorkspace]
		if !ok {
			return nil, ErrWorkspaceNotFound
		}
		return &ws, nil
	}
	// Try by ID
	if ws, ok := data.Workspaces[identifier]; ok {
		return &ws, nil
	}
	// Try by name
	for _, ws := range data.Workspaces {
		if ws.Name == identifier {
			return &ws, nil
		}
	}
	return nil, ErrWorkspaceNotFound
}

func (s *Store) ListWorkspaces() ([]Workspace, string, error) {
	data, err := s.Load()
	if err != nil {
		return nil, "", err
	}
	var list []Workspace
	for _, ws := range data.Workspaces {
		list = append(list, ws)
	}
	return list, data.DefaultWorkspace, nil
}

func (s *Store) ClearAll() error {
	return s.Save(&Data{Workspaces: make(map[string]Workspace)})
}
