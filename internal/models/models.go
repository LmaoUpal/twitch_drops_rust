// Package models holds the domain types shared by the Twitch client, scheduler and miner.
package models

import (
	"strings"
	"time"
)

type Game struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Channel struct {
	ID           string `json:"id"`
	Login        string `json:"login"`
	DisplayName  string `json:"displayName"`
	Online       bool   `json:"online"`
	GameID       string `json:"gameId"`
	DropsEnabled bool   `json:"dropsEnabled"`
	Viewers      int    `json:"viewers"`
	BroadcastID  string `json:"broadcastId"` // current stream id, empty when offline
}

type Drop struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Benefits        []string  `json:"benefits"`
	RequiredMinutes int       `json:"requiredMinutes"`
	CurrentMinutes  int       `json:"currentMinutes"`
	Claimed         bool      `json:"claimed"`
	InstanceID      string    `json:"instanceId"` // set by Twitch once the drop can be claimed
	ImageURL        string    `json:"imageUrl"`
	StartAt         time.Time `json:"startAt"`
	EndAt           time.Time `json:"endAt"`
}

// Active reports whether the drop can still progress at time now.
func (d Drop) Active(now time.Time) bool {
	return !d.Claimed && !now.Before(d.StartAt) && now.Before(d.EndAt)
}

type Campaign struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Game          Game      `json:"game"`
	StartAt       time.Time `json:"startAt"`
	EndAt         time.Time `json:"endAt"`
	AccountLinked bool      `json:"accountLinked"`
	LinkURL       string    `json:"linkUrl"`
	// Source is "twitch" for campaigns from the Twitch inventory, "facepunch" for the Rust schedule
	// (no progress or real Twitch IDs until the campaign shows up in the inventory).
	Source string `json:"source"`
	// AllowedChannels restricts progress to these channels; nil means any channel playing Game.
	AllowedChannels []Channel `json:"allowedChannels"`
	Drops           []Drop    `json:"drops"`
}

// Restricted reports whether the campaign only counts on specific channels.
func (c Campaign) Restricted() bool { return c.AllowedChannels != nil }

// Active reports whether the campaign is running and has at least one drop left to earn.
func (c Campaign) Active(now time.Time) bool {
	if now.Before(c.StartAt) || !now.Before(c.EndAt) {
		return false
	}
	for _, d := range c.Drops {
		if d.Active(now) {
			return true
		}
	}
	return false
}

// Allows reports whether watching ch progresses this campaign.
func (c Campaign) Allows(ch Channel) bool {
	if ch.GameID != c.Game.ID {
		return false
	}
	if !c.Restricted() {
		return true
	}
	for _, a := range c.AllowedChannels {
		if a.ID != "" && a.ID == ch.ID {
			return true
		}
		// Facepunch only gives logins.
		if a.ID == "" && a.Login != "" && strings.EqualFold(a.Login, ch.Login) {
			return true
		}
	}
	return false
}
