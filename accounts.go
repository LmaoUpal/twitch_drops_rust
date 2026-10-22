package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/LmaoUpal/twitch_drops_rust/internal/catalog"
	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const eventAccounts = "accounts:update"

// AccountInfo is a saved login as shown in the account switcher.
type AccountInfo struct {
	Key         string `json:"key"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	Active      bool   `json:"active"`
}

// GetAccounts lists saved logins in the order they were added. One account mines at a time.
func (a *App) GetAccounts() []AccountInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accountsLocked()
}

func (a *App) accountsLocked() []AccountInfo {
	out := make([]AccountInfo, 0, len(a.cfg.Accounts))
	for _, acc := range a.cfg.Accounts {
		out = append(out, AccountInfo{Key: acc.Key, Login: acc.Login, DisplayName: acc.DisplayName, AvatarURL: acc.AvatarURL, Active: acc.Key == a.cfg.ActiveAccount})
	}
	return out
}

// SwitchAccount stops mining on the current account and signs in with the saved token of key.
func (a *App) SwitchAccount(key string) error {
	known := a.knownAccount(key)
	a.mu.Lock()
	same := key == a.cfg.ActiveAccount && a.status.State == authLoggedIn
	a.mu.Unlock()
	if !known {
		return fmt.Errorf("unknown account %q", key)
	}
	if same {
		return nil
	}
	a.CancelLogin()
	a.deactivate()
	a.mu.Lock()
	a.cfg.ActiveAccount = key
	a.session = a.newSession(key)
	a.mu.Unlock()
	a.persistAccounts()
	go a.Restore()
	return nil
}

func (a *App) knownAccount(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.ContainsFunc(a.cfg.Accounts, func(acc config.Account) bool { return acc.Key == key })
}

// RemoveAccount forgets a saved login and its token. Removing the active one signs it out.
func (a *App) RemoveAccount(key string) error {
	a.mu.Lock()
	active := key == a.cfg.ActiveAccount
	a.mu.Unlock()
	if active {
		return a.Logout()
	}
	if err := a.newSession(key).Logout(); err != nil && !errors.Is(err, auth.ErrNoToken) {
		return err
	}
	a.forgetAccount(key)
	slog.Info("Account removed", "tag", "App")
	return nil
}

// deactivate stops everything the active account runs, before another one takes over.
func (a *App) deactivate() {
	a.stopWatching()
	a.stopCampaignLoop()
	a.setPoints(func(p *PointsState) { p.Slots, p.Channels = 0, nil })
	runtime.EventsEmit(a.ctx, eventCampaigns, catalog.Snapshot{})
}

// newSession builds the auth session for keyring key; callers hold a.mu or own a.cfg.
func (a *App) newSession(key string) *auth.Session {
	return &auth.Session{
		Client: &auth.Client{
			ClientID:  a.cfg.Twitch.ClientID,
			UserAgent: a.cfg.Twitch.UserAgent,
			Scopes:    a.cfg.Twitch.Scopes,
		},
		Store:   auth.KeyringStore{Service: keyringService},
		Account: key,
	}
}

func (a *App) currentSession() (*auth.Session, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session, a.cfg.ActiveAccount
}

// loginKey picks the keyring key for a freshly authorized token: the saved account with
// the same login, so logging in again replaces its token, or a new key from the user id.
func (a *App) loginKey(ctx context.Context, tok *auth.Token) (string, error) {
	sess, _ := a.currentSession()
	v, err := sess.Client.Validate(ctx, tok.AccessToken)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, acc := range a.cfg.Accounts {
		if acc.Login == v.Login {
			return acc.Key, nil
		}
	}
	return "user-" + v.UserID, nil
}

// rememberAccount records the signed-in login of key, making it the active account.
func (a *App) rememberAccount(key, login string) {
	a.mu.Lock()
	i := slices.IndexFunc(a.cfg.Accounts, func(acc config.Account) bool { return acc.Key == key })
	if i < 0 {
		a.cfg.Accounts = append(a.cfg.Accounts, config.Account{Key: key})
		i = len(a.cfg.Accounts) - 1
	}
	changed := a.cfg.Accounts[i].Login != login || a.cfg.ActiveAccount != key
	a.cfg.Accounts[i].Login = login
	a.cfg.ActiveAccount = key
	a.mu.Unlock()
	if changed {
		a.persistAccounts()
	}
}

// updateProfile stores the display name and avatar of login for the switcher.
func (a *App) updateProfile(login, displayName, avatarURL string) {
	a.mu.Lock()
	i := slices.IndexFunc(a.cfg.Accounts, func(acc config.Account) bool { return acc.Login == login })
	changed := i >= 0 && (a.cfg.Accounts[i].DisplayName != displayName || a.cfg.Accounts[i].AvatarURL != avatarURL)
	if changed {
		a.cfg.Accounts[i].DisplayName, a.cfg.Accounts[i].AvatarURL = displayName, avatarURL
	}
	a.mu.Unlock()
	if changed {
		a.persistAccounts()
	}
}

// forgetAccount drops key from the saved list; the active key is cleared too.
func (a *App) forgetAccount(key string) {
	a.mu.Lock()
	a.cfg.Accounts = slices.DeleteFunc(a.cfg.Accounts, func(acc config.Account) bool { return acc.Key == key })
	if a.cfg.ActiveAccount == key {
		a.cfg.ActiveAccount = ""
	}
	a.mu.Unlock()
	a.persistAccounts()
}

// persistAccounts saves the config and tells the UI about the account list.
func (a *App) persistAccounts() {
	a.mu.Lock()
	cfg, path, list := a.cfg, a.cfgPath, a.accountsLocked()
	a.mu.Unlock()
	if path != "" {
		saveConfig(path, cfg)
	}
	runtime.EventsEmit(a.ctx, eventAccounts, list)
}
