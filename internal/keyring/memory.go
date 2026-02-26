package keyring

import "fmt"

type Memory struct {
	store map[string]string
}

func NewMemory() *Memory {
	return &Memory{store: make(map[string]string)}
}

func (m *Memory) key(workspaceID, key string) string {
	return fmt.Sprintf("%s:%s", workspaceID, key)
}

func (m *Memory) Get(workspaceID, k string) (string, error) {
	val, ok := m.store[m.key(workspaceID, k)]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (m *Memory) Set(workspaceID, k, value string) error {
	m.store[m.key(workspaceID, k)] = value
	return nil
}

func (m *Memory) Delete(workspaceID, k string) error {
	delete(m.store, m.key(workspaceID, k))
	return nil
}
