package auth

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

// ErrNoToken means no saved token exists and the user has to log in.
var ErrNoToken = errors.New("auth: no saved token")

// Store persists tokens per account name.
type Store interface {
	Load(account string) (*Token, error)
	Save(account string, t *Token) error
	Delete(account string) error
}

// KeyringStore keeps tokens in the OS credential store (Windows Credential Manager,
// macOS Keychain, Secret Service on Linux).
type KeyringStore struct {
	Service string
}

func (s KeyringStore) Load(account string) (*Token, error) {
	raw, err := keyring.Get(s.Service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNoToken
	}
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s KeyringStore) Save(account string, t *Token) error {
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return keyring.Set(s.Service, account, string(raw))
}

func (s KeyringStore) Delete(account string) error {
	err := keyring.Delete(s.Service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
