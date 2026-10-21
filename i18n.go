package main

import "fmt"

// Strings the Go side shows outside the web UI: tray menu and desktop notifications.
// The Activity log stays in English.
var messages = map[string]map[string]string{
	"en": {
		"tray.starting":        "Starting…",
		"tray.idle":            "Idle — nothing to watch",
		"tray.paused":          "Paused",
		"tray.watching":        "Watching %s",
		"tray.progress":        " — %d / %d min",
		"tray.offline":         "Channel offline",
		"tray.pause":           "Pause mining",
		"tray.resume":          "Resume mining",
		"tray.manual":          "Manual mode",
		"tray.open":            "Open",
		"tray.quit":            "Quit",
		"notify.trayTitle":     "Still running in the tray",
		"notify.trayBody":      "Twitch Drops Miner keeps mining. Right-click the tray icon to quit.",
		"notify.claimed":       "Drop claimed: %s",
		"notify.started":       "Campaign started: %s",
		"notify.unlinkedTitle": "%s account not linked",
		"notify.unlinkedBody":  "Link it to claim drops from %s.",
	},
	"ru": {
		"tray.starting":        "Запуск…",
		"tray.idle":            "Простой — нечего смотреть",
		"tray.paused":          "На паузе",
		"tray.watching":        "Смотрим %s",
		"tray.progress":        " — %d / %d мин",
		"tray.offline":         "Канал оффлайн",
		"tray.pause":           "Поставить на паузу",
		"tray.resume":          "Продолжить майнинг",
		"tray.manual":          "Ручной режим",
		"tray.open":            "Открыть",
		"tray.quit":            "Выход",
		"notify.trayTitle":     "Работаем в трее",
		"notify.trayBody":      "Twitch Drops Miner продолжает майнить. Чтобы выйти, нажмите правой кнопкой на значок в трее.",
		"notify.claimed":       "Дроп получен: %s",
		"notify.started":       "Кампания началась: %s",
		"notify.unlinkedTitle": "Аккаунт %s не привязан",
		"notify.unlinkedBody":  "Привяжите его, чтобы забрать дропы из «%s».",
	},
}

// tr formats a tray/notification string in the configured language.
func (a *App) tr(key string, args ...any) string {
	a.mu.Lock()
	lang := a.cfg.Language
	a.mu.Unlock()
	msg, ok := messages[lang][key]
	if !ok {
		msg = messages["en"][key]
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
