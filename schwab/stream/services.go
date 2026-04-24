// Package stream implements the Schwab streaming (WebSocket) API client.
//
// The entry point is Streamer. A Streamer is constructed from an existing
// *schwab.Client (which supplies the access token and the user's
// streamerInfo endpoint). Subscriptions are tracked in memory and replayed
// across reconnects.
//
// The wire protocol follows Schwabdev's stream.py; see the repository README
// and the spec document for protocol details.
package stream

// Service identifies a Schwab streaming service. Values are exactly the
// strings that appear in the "service" field on wire (e.g. "LEVELONE_EQUITIES").
type Service string

// Streaming services supported by Schwab. The singular/plural spelling for
// screener services follows the corrected values in spec Appendix C#6.
const (
	// LevelOneEquities is the level-1 equity quote stream.
	LevelOneEquities Service = "LEVELONE_EQUITIES"
	// LevelOneOptions is the level-1 option quote stream.
	LevelOneOptions Service = "LEVELONE_OPTIONS"
	// LevelOneFutures is the level-1 futures quote stream.
	LevelOneFutures Service = "LEVELONE_FUTURES"
	// LevelOneFuturesOptions is the level-1 futures options quote stream.
	LevelOneFuturesOptions Service = "LEVELONE_FUTURES_OPTIONS"
	// LevelOneForex is the level-1 FX quote stream.
	LevelOneForex Service = "LEVELONE_FOREX"
	// NYSEBook is the NYSE order-book feed.
	NYSEBook Service = "NYSE_BOOK"
	// NasdaqBook is the Nasdaq order-book feed.
	NasdaqBook Service = "NASDAQ_BOOK"
	// OptionsBook is the options order-book feed.
	OptionsBook Service = "OPTIONS_BOOK"
	// ChartEquity is the equity chart bars stream.
	ChartEquity Service = "CHART_EQUITY"
	// ChartFutures is the futures chart bars stream.
	ChartFutures Service = "CHART_FUTURES"
	// ScreenerEquity is the equity screener results stream (singular per spec).
	ScreenerEquity Service = "SCREENER_EQUITY"
	// ScreenerOption is the option screener results stream (singular per spec).
	ScreenerOption Service = "SCREENER_OPTION"
	// AccountActivity is the per-account order-event stream.
	AccountActivity Service = "ACCT_ACTIVITY"
	// Admin is the administrative service (LOGIN / LOGOUT / QOS).
	Admin Service = "ADMIN"
)

// Command identifies a streamer command ("command" field on wire).
type Command string

// Streamer commands. Their meaning is defined by the Schwab streaming
// protocol:
//
//	LOGIN   - authenticate and activate the session
//	LOGOUT  - disconnect cleanly
//	SUBS    - replace the subscription set for this service
//	ADD     - add keys to the subscription set; union fields per key
//	UNSUBS  - remove keys from the subscription set
//	VIEW    - change the set of returned fields for the existing keys
const (
	// CmdLogin is the LOGIN command.
	CmdLogin Command = "LOGIN"
	// CmdLogout is the LOGOUT command.
	CmdLogout Command = "LOGOUT"
	// CmdSubs is the SUBS command (replace subscriptions).
	CmdSubs Command = "SUBS"
	// CmdAdd is the ADD command (append subscriptions).
	CmdAdd Command = "ADD"
	// CmdUnsubs is the UNSUBS command (remove subscriptions).
	CmdUnsubs Command = "UNSUBS"
	// CmdView is the VIEW command (change returned fields).
	CmdView Command = "VIEW"
)
