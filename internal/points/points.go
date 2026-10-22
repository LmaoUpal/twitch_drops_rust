// Package points decides which channels to watch for channel points. It is pure: no I/O,
// no clock; the app runs the watchers, PubSub and claims around it.
package points

import (
	"strings"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// MaxWatched is how many channels Twitch credits watch-time points on at the same time.
const MaxWatched = 2

// Slots returns how many channels to watch for points. A drops watch takes one of the
// MaxWatched places (and earns points on its own channel).
func Slots(dropsWatching bool) int {
	if dropsWatching {
		return MaxWatched - 1
	}
	return MaxWatched
}

// Pick returns the channels to watch: the first live ones of tracked (logins in priority
// order), at most slots. dropsLogin, the channel watched for drops, is left out because
// that watch already earns its points.
func Pick(tracked []string, live []models.Channel, slots int, dropsLogin string) []models.Channel {
	byLogin := map[string]models.Channel{}
	for _, ch := range live {
		if ch.Online {
			byLogin[strings.ToLower(ch.Login)] = ch
		}
	}
	var out []models.Channel
	for _, login := range tracked {
		if len(out) >= slots {
			break
		}
		key := strings.ToLower(login)
		if ch, ok := byLogin[key]; ok && !strings.EqualFold(key, dropsLogin) {
			out = append(out, ch)
			delete(byLogin, key) // a login listed twice is watched once
		}
	}
	return out
}
