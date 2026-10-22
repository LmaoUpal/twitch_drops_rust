package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/activity"
	"github.com/LmaoUpal/twitch_drops_rust/internal/catalog"
	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

const (
	keyringService = "twitch-drops"
	// defaultAccount is the only account until multi-account support lands.
	defaultAccount = "main"

	eventAuthStatus = "auth:status"
	eventCampaigns  = "campaigns:update"
)

// Auth states reported to the UI.
const (
	authChecking  = "checking"
	authLoggedOut = "loggedOut"
	authLoggedIn  = "loggedIn"
	authOffline   = "offline" // saved token exists but Twitch could not be reached
)

type AuthStatus struct {
	State       string `json:"state"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	Error       string `json:"error"`
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
	feed    *activity.Feed

	mu          sync.Mutex
	status      AuthStatus
	token       *auth.Token
	cancelLogin context.CancelFunc

	sessionID    string // Client-Session-Id for this run
	campaigns    catalog.Snapshot
	stopLoop     context.CancelFunc
	refreshNow   chan struct{}
	loopForToken string // access token the running campaign loop uses

	watch     WatchState
	stopWatch context.CancelFunc

	auto     AutoState
	autoKick chan struct{}

	points     PointsState
	pointsKick chan struct{}

	tray           tray
	quitting       bool // Quit was chosen, so closing must not hide to the tray
	hiddenOnce     bool
	notifyReady    bool
	seenActive     map[string]bool // campaign ids already active, for "campaign started"
	warnedUnlinked map[string]bool

	allDrops AllDrops // cached community catalog for the All drops tab
}

func NewApp(feed *activity.Feed) *App {
	return &App{feed: feed, cfg: config.Default(), status: AuthStatus{State: authChecking}, watch: WatchState{State: watchIdle}, auto: AutoState{Mode: modeAuto, State: "idle", Queue: []QueueItem{}}}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.feed.OnEntry(func(e activity.Entry) { runtime.EventsEmit(ctx, eventActivity, e) })
	if path, err := config.Path(); err != nil {
		slog.Error("Config location unknown", "tag", "App", "err", err)
	} else {
		a.cfgPath = path
		cfg, err := config.Load(path)
		if err != nil {
			slog.Error("Couldn't read config, using defaults", "tag", "App", "err", err)
		}
		a.cfg = cfg
	}
	a.applyTheme(a.cfg.Theme)
	a.setPoints(func(*PointsState) {}) // list the configured channels before login
	a.session = &auth.Session{
		Client: &auth.Client{
			ClientID:  a.cfg.Twitch.ClientID,
			UserAgent: a.cfg.Twitch.UserAgent,
			Scopes:    a.cfg.Twitch.Scopes,
		},
		Store:   auth.KeyringStore{Service: keyringService},
		Account: defaultAccount,
	}
	a.sessionID = gql.NewSessionID()
	a.startTray()
	go a.initNotifications()
	if a.cfg.StartWithWindows {
		if err := setAutostart(true); err != nil { // refresh the path in case the exe moved
			slog.Warn("Couldn't update start with Windows", "tag", "App", "err", err)
		}
	}
	go func() {
		a.ensureDeviceID() // before any GQL call; network only on first launch
		a.Restore()
	}()
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
		slog.Info("Signed in as "+tok.Login, "tag", "App", "ok", true)
		a.setStatus(AuthStatus{State: authLoggedIn, Login: tok.Login}, tok)
	case errors.Is(err, auth.ErrNoToken):
		a.setStatus(AuthStatus{State: authLoggedOut}, nil)
	default:
		slog.Warn("Couldn't check saved login", "tag", "Twitch", "err", err)
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
			slog.Warn("Login failed", "tag", "Twitch", "err", err)
			a.setStatus(AuthStatus{State: authLoggedOut, Error: err.Error()}, nil)
		default:
			slog.Info("Signed in as "+tok.Login, "tag", "App", "ok", true)
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
	switch {
	case s.State == authLoggedIn && tok != nil:
		a.startCampaignLoop(tok)
	case s.State == authLoggedOut:
		a.StopWatching()
		a.stopCampaignLoop()
		a.setPoints(func(p *PointsState) { p.Slots, p.Channels = 0, nil })
	}
}
