// Package goschwab is a Go client for the Charles Schwab Trader and Market
// Data APIs, ported from the Python Schwabdev library.
//
// The main entry point is the schwab subpackage, which exposes Client,
// NewClient, typed errors, functional options, and pluggable token storage.
// A WebSocket streamer lives under schwab/stream (added in a later wave).
//
// See the package-level docs in schwab/ for usage details.
package goschwab
