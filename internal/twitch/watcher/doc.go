// Package watcher keeps a channel "watched" without decoding video: it polls the
// lowest-quality HLS segment and sends minute-watched events. Implemented in phase 3.
package watcher
