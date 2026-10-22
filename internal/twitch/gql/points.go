package gql

import (
	"context"
	"errors"
	"fmt"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// PointsInfo is the user's channel points state on one channel.
type PointsInfo struct {
	ChannelID string
	Balance   int
	// Enabled is false when the streamer turned channel points off; watching earns nothing.
	Enabled bool
	// ClaimID is the bonus chest waiting to be claimed, empty when there is none.
	ClaimID string
}

// ChannelPoints returns the user's balance on a channel and the pending bonus claim, if any.
func (c *Client) ChannelPoints(ctx context.Context, login string) (PointsInfo, error) {
	var out struct {
		Community *struct {
			Channel *struct {
				ID   string `json:"id"`
				Self *struct {
					CommunityPoints *struct {
						Balance        int `json:"balance"`
						AvailableClaim *struct {
							ID string `json:"id"`
						} `json:"availableClaim"`
					} `json:"communityPoints"`
				} `json:"self"`
				CommunityPointsSettings *struct {
					IsEnabled bool `json:"isEnabled"`
				} `json:"communityPointsSettings"`
			} `json:"channel"`
		} `json:"community"`
	}
	if err := c.Do(ctx, "ChannelPointsContext", map[string]any{"channelLogin": login}, &out); err != nil {
		return PointsInfo{}, err
	}
	if out.Community == nil || out.Community.Channel == nil {
		return PointsInfo{}, fmt.Errorf("gql: no channel %s", login)
	}
	ch := out.Community.Channel
	info := PointsInfo{ChannelID: ch.ID, Enabled: ch.CommunityPointsSettings != nil && ch.CommunityPointsSettings.IsEnabled}
	if ch.Self == nil || ch.Self.CommunityPoints == nil {
		return info, ErrUnauthorized
	}
	info.Balance = ch.Self.CommunityPoints.Balance
	if cl := ch.Self.CommunityPoints.AvailableClaim; cl != nil {
		info.ClaimID = cl.ID
	}
	return info, nil
}

// ClaimPoints claims a bonus chest and returns the new balance.
func (c *Client) ClaimPoints(ctx context.Context, channelID, claimID string) (int, error) {
	var out struct {
		ClaimCommunityPoints *struct {
			CurrentPoints *int `json:"currentPoints"`
			Error         *struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"claimCommunityPoints"`
	}
	vars := map[string]any{"input": map[string]any{"channelID": channelID, "claimID": claimID}}
	if err := c.Do(ctx, "ClaimCommunityPoints", vars, &out); err != nil {
		return 0, err
	}
	r := out.ClaimCommunityPoints
	if r == nil {
		return 0, errors.New("gql: claim returned no result")
	}
	if r.Error != nil && r.Error.Code != "" {
		return 0, fmt.Errorf("gql: claim points: %s", r.Error.Code)
	}
	if r.CurrentPoints == nil {
		return 0, nil
	}
	return *r.CurrentPoints, nil
}

// followQuery and isFollowingQuery are raw queries: no persisted hash is known for following
// with this client (checked 2026-10-21, the TV client accepts raw queries).
const (
	followQuery      = `mutation($id: ID!) { followUser(input: {targetID: $id, disableNotifications: false}) { follow { followedAt } error { code } } }`
	isFollowingQuery = `query($login: String!) { user(login: $login) { id self { follower { followedAt } } } }`
)

// IsFollowing reports whether the user follows the channel.
func (c *Client) IsFollowing(ctx context.Context, login string) (bool, error) {
	var out struct {
		User *struct {
			Self *struct {
				Follower *struct {
					FollowedAt string `json:"followedAt"`
				} `json:"follower"`
			} `json:"self"`
		} `json:"user"`
	}
	if err := c.DoQuery(ctx, isFollowingQuery, map[string]any{"login": login}, &out); err != nil {
		return false, err
	}
	if out.User == nil {
		return false, fmt.Errorf("gql: no channel %s", login)
	}
	return out.User.Self != nil && out.User.Self.Follower != nil, nil
}

// Follow follows a channel. Following again resets the follow date, so check IsFollowing first.
func (c *Client) Follow(ctx context.Context, channelID string) error {
	var out struct {
		FollowUser *struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"followUser"`
	}
	if err := c.DoQuery(ctx, followQuery, map[string]any{"id": channelID}, &out); err != nil {
		return err
	}
	if out.FollowUser == nil {
		return errors.New("gql: follow returned no result")
	}
	if e := out.FollowUser.Error; e != nil && e.Code != "" {
		return fmt.Errorf("gql: follow: %s", e.Code)
	}
	return nil
}

// followsPageSize and followsMaxPages bound Follows to 2000 channels.
const (
	followsPageSize = 100
	followsMaxPages = 20
)

// Follows returns the channels the user follows, oldest follow first.
func (c *Client) Follows(ctx context.Context) ([]models.Channel, error) {
	var out []models.Channel
	cursor := ""
	for page := 0; page < followsMaxPages; page++ {
		var resp struct {
			User *struct {
				Follows struct {
					Edges []struct {
						Cursor string `json:"cursor"`
						Node   struct {
							ID              string `json:"id"`
							Login           string `json:"login"`
							DisplayName     string `json:"displayName"`
							ProfileImageURL string `json:"profileImageURL"`
						} `json:"node"`
					} `json:"edges"`
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
				} `json:"follows"`
			} `json:"user"`
		}
		vars := map[string]any{"limit": followsPageSize, "order": "ASC", "cursor": cursor}
		if err := c.Do(ctx, "ChannelFollows", vars, &resp); err != nil {
			return nil, err
		}
		if resp.User == nil {
			return nil, ErrUnauthorized
		}
		for _, e := range resp.User.Follows.Edges {
			n := e.Node
			out = append(out, models.Channel{ID: n.ID, Login: n.Login, DisplayName: n.DisplayName, AvatarURL: n.ProfileImageURL})
			cursor = e.Cursor
		}
		if !resp.User.Follows.PageInfo.HasNextPage || len(resp.User.Follows.Edges) == 0 {
			break
		}
	}
	return out, nil
}
