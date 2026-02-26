package keyring

import (
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

type MacOS struct{}

func NewMacOS() *MacOS { return &MacOS{} }

func (m *MacOS) serviceName(workspaceID, key string) string {
	return fmt.Sprintf("slack-pipe:%s:%s", workspaceID, key)
}

func (m *MacOS) Get(workspaceID, key string) (string, error) {
	val, err := gokeyring.Get(m.serviceName(workspaceID, key), workspaceID)
	if err != nil {
		if err == gokeyring.ErrNotFound {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain get: %w", err)
	}
	return val, nil
}

func (m *MacOS) Set(workspaceID, key, value string) error {
	return gokeyring.Set(m.serviceName(workspaceID, key), workspaceID, value)
}

func (m *MacOS) Delete(workspaceID, key string) error {
	return gokeyring.Delete(m.serviceName(workspaceID, key), workspaceID)
}
