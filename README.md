# Twitch Drops Miner

Desktop app that earns Twitch drops automatically (Rust / Facepunch first, any game supported).
It does not play video: it keeps a channel "watched" via Twitch GQL, HLS segment polling and
minute-watched events, switches channels when needed and claims drops.

> Automated watching is against Twitch's Terms of Service. Use at your own risk, preferably on a separate account.

Stack: Go + [Wails v2](https://wails.io) + React/TypeScript.

## Layout

- `main.go`, `app.go` — Wails entry point and methods exposed to the UI
- `internal/models` — campaigns, drops, channels
- `internal/scheduler` — picks which channel to watch (pure, unit-tested)
- `internal/config` — settings and GQL hashes in `%APPDATA%\twitch-drops\config.toml`
- `internal/twitch/gql` — GraphQL client (persisted queries)
- `internal/twitch/{auth,watcher,realtime}`, `internal/miner` — upcoming phases
- `frontend/` — React UI

## Develop

```bash
wails dev
```

```bash
go test ./...
```

## Build

```bash
wails build
```

The executable lands in `build/bin/`.

## Installer

Requires [NSIS](https://nsis.sourceforge.io) (`winget install NSIS.NSIS`, makensis on PATH):

```bash
wails build -nsis -installscope user
```

Produces `build/bin/TwitchDropsMiner-amd64-installer.exe`, which installs per user into
`%LOCALAPPDATA%\Programs\Twitch Drops Miner` (no admin rights) and installs WebView2 if missing.
