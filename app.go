package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
)

const (
	keyringService = "twitch-drops"
	// defaultAccount is the only account until multi-account support lands.
	defaultAccount = "main"

	eventAuthStatus = "auth:status"
)

// Auth states reported to the UI.
const (
	authChecking  = "checking"
	authLoggedOut = "loggedOut"
	authLoggedIn  = "loggedIn"
	authOffline   = "offline" // saved token exists but Twitch could not be reached
)

type AuthStatus struct {
	State string `json:"state"`
	Login string `json:"login"`
	Error string `json:"error"`
}

type LoginPrompt struct {
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
}

// App is the bridge between the Go backend and the React frontend; its exported
// methods are callable from the UI via generated wailsjs bindings.
type App struct {
	ctx     context.Context
	cfgPath string
	cfg     config.Config
	session *auth.Session

	mu          sync.Mutex
	status      AuthStatus
	token       *auth.Token
	cancelLogin context.CancelFunc
}

func NewApp() *App {
	return &App{cfg: config.Default(), status: AuthStatus{State: authChecking}}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if path, err := config.Path(); err != nil {
		slog.Error("config path", "err", err)
	} else {
		a.cfgPath = path
		cfg, err := config.Load(path)
		if err != nil {
			slog.Error("load config, using defaults", "path", path, "err", err)
		}
		a.cfg = cfg
	}
	a.session = &auth.Session{
		Client: &auth.Client{
			ClientID:  a.cfg.Twitch.ClientID,
			UserAgent: a.cfg.Twitch.UserAgent,
			Scopes:    a.cfg.Twitch.Scopes,
		},
		Store:   auth.KeyringStore{Service: keyringService},
		Account: defaultAccount,
	}
	go a.Restore()
}

// GetConfig returns the current settings.
func (a *App) GetConfig() config.Config {
	return a.cfg
}

// SetGames replaces the game priority list and persists it.
func (a *App) SetGames(games []string) error {
	a.cfg.Games = games
	return config.Save(a.cfgPath, a.cfg)
}

// GetAuthStatus returns the current login state without touching the network.
func (a *App) GetAuthStatus() AuthStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}

// Restore re-checks the saved token with Twitch and publishes the result.
func (a *App) Restore() {
	a.setStatus(AuthStatus{State: authChecking}, nil)
	tok, err := a.session.Restore(a.ctx)
	switch {
	case err == nil:
		a.setStatus(AuthStatus{State: authLoggedIn, Login: tok.Login}, tok)
	case errors.Is(err, auth.ErrNoToken):
		a.setStatus(AuthStatus{State: authLoggedOut}, nil)
	default:
		slog.Warn("restore session", "err", err)
		a.setStatus(AuthStatus{State: authOffline, Error: err.Error()}, nil)
	}
}

// StartLogin begins the device code flow. The UI shows UserCode and opens
// VerificationURI; the result arrives later as an "auth:status" event.
func (a *App) StartLogin() (LoginPrompt, error) {
	a.CancelLogin()
	dc, err := a.session.Client.StartDevice(a.ctx)
	if err != nil {
		return LoginPrompt{}, err
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.cancelLogin = cancel
	a.mu.Unlock()

	go func() {
		defer cancel()
		tok, err := a.session.Client.PollToken(ctx, dc)
		if err == nil {
			tok, err = a.session.Complete(ctx, tok)
		}
		switch {
		case errors.Is(err, context.Canceled):
			return
		case err != nil:
			slog.Warn("device login", "err", err)
			a.setStatus(AuthStatus{State: authLoggedOut, Error: err.Error()}, nil)
		default:
			slog.Info("logged in", "login", tok.Login)
			a.setStatus(AuthStatus{State: authLoggedIn, Login: tok.Login}, tok)
		}
	}()

	return LoginPrompt{UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, ExpiresIn: dc.ExpiresIn}, nil
}

// CancelLogin stops a pending device code flow, if any.
func (a *App) CancelLogin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelLogin != nil {
		a.cancelLogin()
		a.cancelLogin = nil
	}
}

// Logout forgets the saved token.
func (a *App) Logout() error {
	a.CancelLogin()
	if err := a.session.Logout(); err != nil {
		return err
	}
	a.setStatus(AuthStatus{State: authLoggedOut}, nil)
	return nil
}

func (a *App) setStatus(s AuthStatus, tok *auth.Token) {
	a.mu.Lock()
	a.status, a.token = s, tok
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventAuthStatus, s)
}
