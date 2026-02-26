package keyring

import (
	"fmt"
	"os"
)

type Env struct{}

func NewEnv() *Env { return &Env{} }

func (e *Env) Get(_ string, key string) (string, error) {
	var envVar string
	switch key {
	case KeyToken:
		envVar = "SLACK_TOKEN"
	case KeyCookie:
		envVar = "SLACK_COOKIE"
	default:
		return "", fmt.Errorf("unknown key: %s", key)
	}
	val := os.Getenv(envVar)
	if val == "" {
		return "", ErrNotFound
	}
	return val, nil
}

func (e *Env) Set(_, _, _ string) error {
	return fmt.Errorf("env keyring is read-only; set SLACK_TOKEN / SLACK_COOKIE directly")
}

func (e *Env) Delete(_, _ string) error {
	return fmt.Errorf("env keyring is read-only; unset SLACK_TOKEN / SLACK_COOKIE directly")
}
