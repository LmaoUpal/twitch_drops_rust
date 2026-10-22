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
	keyringService  = "twitch-drops"
	eventAuthStatus = "auth:status"
	eventLoginError = "login:error" // a device login failed; sent even while another account is signed in
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
	a.session = a.newSession(a.cfg.ActiveAccount)
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

// Restore re-checks the saved token of the active account with Twitch and publishes the result.
func (a *App) Restore() {
	sess, key := a.currentSession()
	if key == "" {
		a.setStatus(AuthStatus{State: authLoggedOut}, nil)
		return
	}
	a.setStatus(AuthStatus{State: authChecking}, nil)
	tok, err := sess.Restore(a.ctx)
	switch {
	case err == nil:
		slog.Info("Signed in as "+tok.Login, "tag", "App", "ok", true)
		a.rememberAccount(key, tok.Login)
		a.setStatus(AuthStatus{State: authLoggedIn, Login: tok.Login}, tok)
	case errors.Is(err, auth.ErrNoToken):
		if a.knownAccount(key) {
			slog.Warn("Saved login expired, sign in again", "tag", "Twitch")
		}
		a.forgetAccount(key)
		a.setStatus(AuthStatus{State: authLoggedOut}, nil)
	default:
		slog.Warn("Couldn't check saved login", "tag", "Twitch", "err", err)
		a.setStatus(AuthStatus{State: authOffline, Error: err.Error()}, nil)
	}
}

// StartLogin begins the device code flow. The UI shows UserCode and opens
// VerificationURI; the result arrives later as an "auth:status" event. While another
// account is signed in it keeps mining until the new login completes, then it is replaced.
func (a *App) StartLogin() (LoginPrompt, error) {
	a.CancelLogin()
	sess, _ := a.currentSession()
	dc, err := sess.Client.StartDevice(a.ctx)
	if err != nil {
		return LoginPrompt{}, err
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.cancelLogin = cancel
	a.mu.Unlock()

	go func() {
		defer cancel()
		key, tok, err := a.completeLogin(ctx, sess, dc)
		switch {
		case errors.Is(err, context.Canceled):
			return
		case err != nil:
			slog.Warn("Login failed", "tag", "Twitch", "err", err)
			runtime.EventsEmit(a.ctx, eventLoginError, err.Error())
			if a.GetAuthStatus().State != authLoggedIn {
				a.setStatus(AuthStatus{State: authLoggedOut, Error: err.Error()}, nil)
			}
		default:
			slog.Info("Signed in as "+tok.Login, "tag", "App", "ok", true)
			a.deactivate()
			a.mu.Lock()
			a.session = a.newSession(key)
			a.mu.Unlock()
			a.rememberAccount(key, tok.Login)
			a.setStatus(AuthStatus{State: authLoggedIn, Login: tok.Login}, tok)
		}
	}()

	return LoginPrompt{UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, ExpiresIn: dc.ExpiresIn}, nil
}

// completeLogin waits for the user to authorize dc and saves the token under its account key.
func (a *App) completeLogin(ctx context.Context, sess *auth.Session, dc *auth.DeviceCode) (string, *auth.Token, error) {
	tok, err := sess.Client.PollToken(ctx, dc)
	if err != nil {
		return "", nil, err
	}
	key, err := a.loginKey(ctx, tok)
	if err != nil {
		return "", nil, err
	}
	a.mu.Lock()
	target := a.newSession(key)
	a.mu.Unlock()
	tok, err = target.Complete(ctx, tok)
	return key, tok, err
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

// Logout forgets the active account and its token, then switches to the next saved account.
func (a *App) Logout() error {
	a.CancelLogin()
	sess, key := a.currentSession()
	if err := sess.Logout(); err != nil {
		return err
	}
	a.forgetAccount(key)
	slog.Info("Signed out", "tag", "App")
	a.setStatus(AuthStatus{State: authLoggedOut}, nil)
	if accs := a.GetAccounts(); len(accs) > 0 {
		return a.SwitchAccount(accs[0].Key)
	}
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
		a.stopWatching()
		a.stopCampaignLoop()
		a.setPoints(func(p *PointsState) { p.Slots, p.Channels = 0, nil })
	}
}
