// Package realtime listens to Twitch PubSub (wss://pubsub-edge.twitch.tv/v1): channel points
// earned and bonus claims, stream up/down, raids and predictions. It keeps one connection,
// re-listening to its topics after every reconnect.
package realtime
