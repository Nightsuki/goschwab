package stream

import (
	"context"
	"encoding/json"
)

// Request is a single streamer request frame. Multiple Requests are wrapped
// in an Envelope when sent on the wire.
type Request struct {
	// Service is the target service (e.g. "LEVELONE_EQUITIES").
	Service Service `json:"service"`
	// Command is the streamer command (e.g. "SUBS", "ADD").
	Command Command `json:"command"`
	// RequestID is a monotonically increasing identifier set by the client.
	RequestID int64 `json:"requestid,string"`
	// SchwabClientCustomerID identifies the streaming customer.
	SchwabClientCustomerID string `json:"SchwabClientCustomerId"`
	// SchwabClientCorrelID is the correlation ID from streamerInfo.
	SchwabClientCorrelID string `json:"SchwabClientCorrelId"`
	// Parameters holds service-specific parameters such as "keys" and "fields".
	Parameters map[string]string `json:"parameters"`
}

// Envelope is the on-wire wrapper written to the WebSocket. The Schwab
// streamer expects every frame to include a top-level "requests" array.
type Envelope struct {
	// Requests is the list of commands in this frame.
	Requests []Request `json:"requests"`
}

// ResponseFrame is an acknowledgement frame returned by the streamer after
// a client request is processed. The Content field contains the raw
// service-specific payload (e.g. {"code":0,"msg":"login success"}).
type ResponseFrame struct {
	// Service is the echoed service name.
	Service string `json:"service"`
	// Command is the echoed command name.
	Command string `json:"command"`
	// SchwabClientCorrelID is the echoed correlation ID.
	SchwabClientCorrelID string `json:"SchwabClientCorrelId,omitempty"`
	// RequestID is the echoed request identifier.
	RequestID string `json:"requestid,omitempty"`
	// Timestamp is the server timestamp in epoch milliseconds.
	Timestamp int64 `json:"timestamp,omitempty"`
	// Content is the service-specific response body.
	Content json.RawMessage `json:"content,omitempty"`
}

// NotifyFrame is a lightweight server notification (e.g. heartbeats).
type NotifyFrame struct {
	// Heartbeat is the server heartbeat timestamp as a string. Set on heartbeats.
	Heartbeat string `json:"heartbeat,omitempty"`
	// Service is the notifying service, when applicable.
	Service string `json:"service,omitempty"`
	// Timestamp is the server timestamp in epoch milliseconds.
	Timestamp int64 `json:"timestamp,omitempty"`
	// Content is an arbitrary notification payload.
	Content json.RawMessage `json:"content,omitempty"`
}

// DataFrame is a market-data update carrying content for one or more symbols.
type DataFrame struct {
	// Service is the data service (e.g. "LEVELONE_EQUITIES").
	Service string `json:"service"`
	// Command is the echoed command that produced the data (typically "SUBS").
	Command string `json:"command,omitempty"`
	// Timestamp is the server timestamp in epoch milliseconds.
	Timestamp int64 `json:"timestamp,omitempty"`
	// Content is the data payload; typically a JSON array of per-symbol records
	// with numeric-string field keys.
	Content json.RawMessage `json:"content,omitempty"`
}

// Message is the parsed view of a server frame. Only one of Response,
// Notify, or Data is populated in practice for a single incoming frame.
type Message struct {
	// Response holds acknowledgement frames for client requests.
	Response []ResponseFrame `json:"response,omitempty"`
	// Notify holds server notifications (heartbeats, etc.).
	Notify []NotifyFrame `json:"notify,omitempty"`
	// Data holds market-data frames.
	Data []DataFrame `json:"data,omitempty"`
}

// Handler is the low-level raw-message callback. It receives the complete
// WebSocket text frame exactly as read from the wire.
type Handler func(msg []byte)

// TypedHandler is the high-level parsed-message callback. It receives a
// parsed *Message alongside the enclosing context. Implementations must not
// retain references to msg after returning; the contents may be reused.
type TypedHandler func(ctx context.Context, msg *Message)
