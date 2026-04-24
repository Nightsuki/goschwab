package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nightsuki/goschwab/schwab"
	"nhooyr.io/websocket"
)

// Defaults for Streamer options.
const (
	// DefaultPingInterval is the default interval between client-side pings.
	DefaultPingInterval = 20 * time.Second
	// DefaultInitialBackoff is the default starting backoff between reconnects.
	DefaultInitialBackoff = 2 * time.Second
	// DefaultMaxBackoff is the default cap on reconnect backoff.
	DefaultMaxBackoff = 120 * time.Second
	// stableConnectionThreshold is the minimum uptime after which an
	// abnormal close still triggers a reconnect. Abnormal closes that happen
	// before this window bail out instead.
	stableConnectionThreshold = 90 * time.Second
	// sendQueueCapacity is the capacity of the offline send queue.
	sendQueueCapacity = 256
)

// config holds the tunable Streamer options.
type config struct {
	logger         *slog.Logger
	pingInterval   time.Duration
	initialBackoff time.Duration
	maxBackoff     time.Duration
	rawHandler     Handler
	typedHandler   TypedHandler
}

// Option configures a Streamer at construction time.
type Option func(*config)

// WithLogger overrides the slog.Logger used for streamer diagnostics. If
// unset, the Streamer uses the client's logger (or slog.Default()).
func WithLogger(l *slog.Logger) Option {
	return func(c *config) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithPingInterval sets the interval between client-side pings. Values <= 0
// are ignored.
func WithPingInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.pingInterval = d
		}
	}
}

// WithInitialBackoff sets the starting reconnect backoff. Values <= 0 are
// ignored.
func WithInitialBackoff(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.initialBackoff = d
		}
	}
}

// WithMaxBackoff caps the reconnect backoff. Values <= 0 are ignored.
func WithMaxBackoff(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.maxBackoff = d
		}
	}
}

// WithHandler registers the raw-message handler. Invoked once per WebSocket
// text frame received from the server, before TypedHandler.
func WithHandler(h Handler) Option {
	return func(c *config) { c.rawHandler = h }
}

// WithTypedHandler registers the parsed-message handler. Invoked once per
// WebSocket text frame after JSON parsing. If parsing fails the typed
// handler is skipped (the raw handler still runs).
func WithTypedHandler(h TypedHandler) Option {
	return func(c *config) { c.typedHandler = h }
}

// Streamer is the Schwab streaming API client. It manages a single
// WebSocket connection with automatic reconnect, subscription tracking,
// and offline queuing.
//
// A Streamer is not safe for concurrent Start/Stop but Send / Subscribe /
// Unsubscribe / Subscriptions / Active may be called from any goroutine
// after construction.
type Streamer struct {
	client *schwab.Client
	cfg    config

	subs *subscriptionState
	bo   *backoff

	// requestID is monotonically incremented for every outbound Request.
	requestID atomic.Int64

	mu            sync.Mutex
	writeMu       sync.Mutex // guards all conn.Write calls
	active        bool
	stopped       bool
	conn          *websocket.Conn
	streamerInfo  schwab.StreamerInfo
	connectedAt   time.Time
	cancelCurrent context.CancelFunc
	// sendQ buffers Requests while inactive; drained on next successful Start.
	sendQ chan Request
	// doneCh is closed when Start returns. Reserved for tests / shutdown.
	doneCh chan struct{}

	// streamerURLOverride is consulted when non-empty instead of the URL
	// returned by GetUserPreferences. It exists solely for in-process
	// testing (see export_test.go for the test-only setter).
	streamerURLOverride string
	// dialHook, when set, is invoked instead of websocket.Dial. For tests.
	dialHook func(ctx context.Context, url string) (*websocket.Conn, error)
	// nowFn returns the current time. Overridable for tests.
	nowFn func() time.Time
}

// New constructs a Streamer wrapping the given REST client. The client is
// used to obtain the user's streamerInfo and the current OAuth access token
// on every reconnect.
func New(client *schwab.Client, opts ...Option) *Streamer {
	cfg := config{
		pingInterval:   DefaultPingInterval,
		initialBackoff: DefaultInitialBackoff,
		maxBackoff:     DefaultMaxBackoff,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.logger == nil {
		if l := client.Logger(); l != nil {
			cfg.logger = l
		} else {
			cfg.logger = slog.Default()
		}
	}
	return &Streamer{
		client: client,
		cfg:    cfg,
		subs:   newSubscriptionState(),
		bo:     newBackoff(cfg.initialBackoff, cfg.maxBackoff),
		sendQ:  make(chan Request, sendQueueCapacity),
		nowFn:  time.Now,
	}
}

// Active reports whether the streamer currently has an open, logged-in
// WebSocket connection.
func (s *Streamer) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Subscriptions returns a deep copy of the current subscription state.
func (s *Streamer) Subscriptions() map[Service]map[string][]string {
	return s.subs.snapshot()
}

// nextRequestID returns the next unused request identifier.
func (s *Streamer) nextRequestID() int64 { return s.requestID.Add(1) }

// Start opens the WebSocket connection, authenticates, replays recorded
// subscriptions, and spawns the background reader / reconnect goroutines.
// It returns once the initial LOGIN has succeeded (or immediately on
// failure).
//
// Start must be called exactly once per Streamer instance.
func (s *Streamer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return schwab.ErrStreamClosed
	}
	if s.cancelCurrent != nil {
		s.mu.Unlock()
		return errors.New("schwab: streamer already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancelCurrent = cancel
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	// Initial connect must succeed synchronously; subsequent reconnects are
	// handled asynchronously in the supervisor loop.
	if err := s.connectAndLogin(runCtx); err != nil {
		s.mu.Lock()
		s.cancelCurrent = nil
		close(s.doneCh)
		s.mu.Unlock()
		cancel()
		return err
	}

	go s.supervise(runCtx)
	return nil
}

// Stop sends LOGOUT, closes the WebSocket, and stops all background
// goroutines. If clearSubscriptions is true, the subscription registry is
// wiped; otherwise it is preserved for future Start calls.
func (s *Streamer) Stop(ctx context.Context, clearSubscriptions bool) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	conn := s.conn
	active := s.active
	cancel := s.cancelCurrent
	s.mu.Unlock()

	var logoutErr error
	if active && conn != nil {
		req := s.newRequest(Admin, CmdLogout, nil, nil)
		logoutErr = s.writeFrame(ctx, conn, req)
	}

	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "client stop")
	}
	if cancel != nil {
		cancel()
	}

	if clearSubscriptions {
		s.subs.clear()
	}

	s.mu.Lock()
	s.active = false
	s.conn = nil
	s.mu.Unlock()
	return logoutErr
}

// Send writes one or more subscription requests. The provided Request values are
// populated in place with identity fields (RequestID, SchwabClientCustomerID,
// SchwabClientCorrelID) — callers should not share Request values across goroutines
// without synchronization.
//
// If the streamer is active the requests are written immediately. Otherwise
// they are appended to the offline queue (capacity 256) and replayed after
// the next successful LOGIN. Send also records each request in the
// subscription state so reconnects replay the correct subscriptions.
func (s *Streamer) Send(ctx context.Context, reqs ...Request) error {
	for i := range reqs {
		if reqs[i].RequestID == 0 {
			reqs[i].RequestID = s.nextRequestID()
		}
		if reqs[i].SchwabClientCustomerID == "" {
			reqs[i].SchwabClientCustomerID = s.streamerInfo.SchwabClientCustomerID
		}
		if reqs[i].SchwabClientCorrelID == "" {
			reqs[i].SchwabClientCorrelID = s.streamerInfo.SchwabClientCorrelID
		}
		s.subs.record(reqs[i])

		s.mu.Lock()
		active := s.active
		conn := s.conn
		s.mu.Unlock()
		if active && conn != nil {
			if err := s.writeFrame(ctx, conn, reqs[i]); err != nil {
				return err
			}
			continue
		}
		select {
		case s.sendQ <- reqs[i]:
		default:
			return fmt.Errorf("schwab: send queue full (capacity %d)", sendQueueCapacity)
		}
	}
	return nil
}

// Subscribe issues an ADD request, appending keys to the subscription set.
func (s *Streamer) Subscribe(ctx context.Context, svc Service, keys, fields []string) error {
	return s.Send(ctx, s.newRequest(svc, CmdAdd, keys, fields))
}

// SubscribeReplace issues a SUBS request, replacing the subscription set.
func (s *Streamer) SubscribeReplace(ctx context.Context, svc Service, keys, fields []string) error {
	return s.Send(ctx, s.newRequest(svc, CmdSubs, keys, fields))
}

// Unsubscribe issues an UNSUBS request, removing keys from the set.
func (s *Streamer) Unsubscribe(ctx context.Context, svc Service, keys []string) error {
	return s.Send(ctx, s.newRequest(svc, CmdUnsubs, keys, nil))
}

// SetFields issues a VIEW request that changes the returned field list for
// every currently-tracked key of svc.
func (s *Streamer) SetFields(ctx context.Context, svc Service, fields []string) error {
	keys := s.subs.keys(svc)
	req := s.newRequest(svc, CmdView, keys, fields)
	return s.Send(ctx, req)
}

// ----------------------------------------------------------------------
// Typed service helpers (spec §3.6).
// ----------------------------------------------------------------------

// LevelOneEquities issues cmd against the LEVELONE_EQUITIES service.
func (s *Streamer) LevelOneEquities(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(LevelOneEquities, cmd, keys, fields))
}

// LevelOneOptions issues cmd against the LEVELONE_OPTIONS service.
func (s *Streamer) LevelOneOptions(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(LevelOneOptions, cmd, keys, fields))
}

// LevelOneFutures issues cmd against the LEVELONE_FUTURES service.
func (s *Streamer) LevelOneFutures(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(LevelOneFutures, cmd, keys, fields))
}

// LevelOneFuturesOptions issues cmd against the LEVELONE_FUTURES_OPTIONS service.
func (s *Streamer) LevelOneFuturesOptions(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(LevelOneFuturesOptions, cmd, keys, fields))
}

// LevelOneForex issues cmd against the LEVELONE_FOREX service.
func (s *Streamer) LevelOneForex(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(LevelOneForex, cmd, keys, fields))
}

// NYSEBook issues cmd against the NYSE_BOOK service.
func (s *Streamer) NYSEBook(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(NYSEBook, cmd, keys, fields))
}

// NasdaqBook issues cmd against the NASDAQ_BOOK service.
func (s *Streamer) NasdaqBook(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(NasdaqBook, cmd, keys, fields))
}

// OptionsBook issues cmd against the OPTIONS_BOOK service.
func (s *Streamer) OptionsBook(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(OptionsBook, cmd, keys, fields))
}

// ChartEquity issues cmd against the CHART_EQUITY service.
func (s *Streamer) ChartEquity(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(ChartEquity, cmd, keys, fields))
}

// ChartFutures issues cmd against the CHART_FUTURES service.
func (s *Streamer) ChartFutures(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(ChartFutures, cmd, keys, fields))
}

// ScreenerEquity issues cmd against the SCREENER_EQUITY service.
func (s *Streamer) ScreenerEquity(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(ScreenerEquity, cmd, keys, fields))
}

// ScreenerOption issues cmd against the SCREENER_OPTION service.
func (s *Streamer) ScreenerOption(ctx context.Context, keys, fields []string, cmd Command) error {
	return s.Send(ctx, s.newRequest(ScreenerOption, cmd, keys, fields))
}

// AccountActivity subscribes to ACCT_ACTIVITY with the canonical defaults
// ("Account Activity" key, fields 0..3).
func (s *Streamer) AccountActivity(ctx context.Context, cmd Command) error {
	return s.Send(ctx, s.newRequest(AccountActivity, cmd,
		[]string{"Account Activity"},
		[]string{"0", "1", "2", "3"},
	))
}

// ----------------------------------------------------------------------
// Internal helpers.
// ----------------------------------------------------------------------

// newRequest builds a Request populated with identity + parameters.
func (s *Streamer) newRequest(svc Service, cmd Command, keys, fields []string) Request {
	params := make(map[string]string)
	if len(keys) > 0 {
		params["keys"] = strings.Join(keys, ",")
	}
	if len(fields) > 0 {
		params["fields"] = strings.Join(fields, ",")
	}
	return Request{
		Service:                svc,
		Command:                cmd,
		RequestID:              s.nextRequestID(),
		SchwabClientCustomerID: s.streamerInfo.SchwabClientCustomerID,
		SchwabClientCorrelID:   s.streamerInfo.SchwabClientCorrelID,
		Parameters:             params,
	}
}

// writeFrame marshals env and writes it as a single WebSocket text frame.
// It serializes all writes through writeMu so that concurrent callers never
// race on the same connection (nhooyr.io/websocket allows one writer at a time).
func (s *Streamer) writeFrame(ctx context.Context, conn *websocket.Conn, reqs ...Request) error {
	if conn == nil {
		return schwab.ErrStreamInactive
	}
	env := Envelope{Requests: reqs}
	buf, err := json.Marshal(env)
	if err != nil {
		return &schwab.StreamError{Op: "write", Err: err}
	}
	s.writeMu.Lock()
	werr := conn.Write(ctx, websocket.MessageText, buf)
	s.writeMu.Unlock()
	if werr != nil {
		return &schwab.StreamError{Op: "write", Err: werr}
	}
	return nil
}

// connectAndLogin refreshes tokens, opens the WebSocket, sends LOGIN, waits
// for the acknowledgement, and replays recorded subscriptions. On success
// it installs the connection into s and marks the Streamer active.
func (s *Streamer) connectAndLogin(ctx context.Context) error {
	// 1) refresh tokens + fetch user preferences for the streamer endpoint.
	prefs, err := s.client.GetUserPreferences(ctx)
	if err != nil {
		return &schwab.StreamError{Op: "connect", Err: err}
	}
	if len(prefs.StreamerInfo) == 0 {
		return &schwab.StreamError{Op: "connect", Err: errors.New("user preferences returned no streamerInfo")}
	}
	info := prefs.StreamerInfo[0]

	s.mu.Lock()
	urlOverride := s.streamerURLOverride
	if urlOverride != "" {
		info.StreamerSocketURL = urlOverride
	}
	s.streamerInfo = info
	dialHook := s.dialHook
	s.mu.Unlock()

	dialURL := info.StreamerSocketURL

	// Enforce TLS for the streamer URL unless a test override is in effect.
	if urlOverride == "" && !strings.HasPrefix(dialURL, "wss://") {
		return &schwab.StreamError{Op: "connect", Err: fmt.Errorf("refusing insecure streamer URL %q (must be wss://)", dialURL)}
	}

	// 2) open the WebSocket.
	var conn *websocket.Conn
	if dialHook != nil {
		conn, err = dialHook(ctx, dialURL)
	} else {
		conn, _, err = websocket.Dial(ctx, dialURL, nil)
	}
	if err != nil {
		return &schwab.StreamError{Op: "connect", Err: err}
	}
	// Schwab's first snapshot frame after SUBS can be several hundred KB when
	// subscribing to a large symbol set; nhooyr's 32 KB default trips that.
	// Bump generously — 16 MB matches Schwab's own documented frame ceiling.
	conn.SetReadLimit(16 * 1024 * 1024)

	// 3) send LOGIN.
	token, err := s.client.CurrentToken(ctx)
	if err != nil {
		_ = conn.Close(websocket.StatusAbnormalClosure, "token refresh failed")
		return &schwab.StreamError{Op: "login", Err: err}
	}
	loginReq := Request{
		Service:                Admin,
		Command:                CmdLogin,
		RequestID:              s.nextRequestID(),
		SchwabClientCustomerID: info.SchwabClientCustomerID,
		SchwabClientCorrelID:   info.SchwabClientCorrelID,
		Parameters: map[string]string{
			"Authorization":          token.AccessToken,
			"SchwabClientChannel":    info.SchwabClientChannel,
			"SchwabClientFunctionId": info.SchwabClientFunctionID,
		},
	}
	if err := s.writeFrame(ctx, conn, loginReq); err != nil {
		_ = conn.Close(websocket.StatusAbnormalClosure, "login write failed")
		return &schwab.StreamError{Op: "login", Err: err}
	}

	// 4) wait for LOGIN response.
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, raw, err := conn.Read(readCtx)
	cancel()
	if err != nil {
		_ = conn.Close(websocket.StatusAbnormalClosure, "login read failed")
		return &schwab.StreamError{Op: "login", Err: err}
	}
	if err := s.verifyLoginAck(raw); err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "login rejected")
		return err
	}

	// 5) install the connection + mark active.
	s.mu.Lock()
	s.conn = conn
	s.active = true
	s.connectedAt = s.nowFn()
	s.mu.Unlock()
	s.bo.Reset()

	// 6) replay tracked subscriptions.
	replay := s.subs.replayRequests()
	for i := range replay {
		replay[i].RequestID = s.nextRequestID()
		replay[i].SchwabClientCustomerID = info.SchwabClientCustomerID
		replay[i].SchwabClientCorrelID = info.SchwabClientCorrelID
		if err := s.writeFrame(ctx, conn, replay[i]); err != nil {
			return err
		}
	}

	// 7) drain the offline send queue.
	s.drainQueue(ctx, conn)

	s.cfg.logger.Info("schwab: stream login succeeded",
		slog.String("url", info.StreamerSocketURL))
	return nil
}

// verifyLoginAck inspects a LOGIN response frame and returns a StreamError
// if the code is non-zero or the frame shape is unexpected.
func (s *Streamer) verifyLoginAck(raw []byte) error {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return &schwab.StreamError{Op: "login", Err: fmt.Errorf("parse ack: %w", err)}
	}
	if len(msg.Response) == 0 {
		return &schwab.StreamError{Op: "login", Err: fmt.Errorf("no response frame in ack: %s", string(raw))}
	}
	var content struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(msg.Response[0].Content, &content); err != nil {
		return &schwab.StreamError{Op: "login", Err: fmt.Errorf("parse ack content: %w", err)}
	}
	if content.Code != 0 {
		return &schwab.StreamError{Op: "login", Err: fmt.Errorf("login rejected: code=%d msg=%q", content.Code, content.Msg)}
	}
	return nil
}

// drainQueue writes every pending request in the offline queue.
// On write error the failed request is pushed back onto sendQ so that the
// supervisor can reconnect and retry; if the queue is full it is logged and
// dropped.
func (s *Streamer) drainQueue(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case req := <-s.sendQ:
			if err := s.writeFrame(ctx, conn, req); err != nil {
				s.cfg.logger.Warn("schwab: drain queue write failed", slog.Any("error", err))
				// On write error, push the request back onto sendQ and bail out;
				// supervise will reconnect and retry.
				select {
				case s.sendQ <- req:
				default:
					// queue full — log and drop
					s.cfg.logger.Warn("schwab.stream: sendQ full on drain-error; request dropped", "service", req.Service)
				}
				return
			}
		default:
			return
		}
	}
}

// supervise runs the reader loop, and on connection loss decides whether to
// reconnect. It exits when ctx is cancelled (clean Stop) or when a bail-out
// condition is met (unstable connection dropped within 90s).
func (s *Streamer) supervise(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		if s.doneCh != nil {
			select {
			case <-s.doneCh:
			default:
				close(s.doneCh)
			}
		}
		s.mu.Unlock()
	}()

	for {
		s.mu.Lock()
		conn := s.conn
		connectedAt := s.connectedAt
		s.mu.Unlock()

		closeErr := s.readLoop(ctx, conn)

		s.mu.Lock()
		s.active = false
		s.conn = nil
		stopped := s.stopped
		s.mu.Unlock()

		if stopped || ctx.Err() != nil {
			return
		}

		// Evaluate close status for reconnect decision.
		var ce websocket.CloseError
		cleanClose := errors.As(closeErr, &ce) && ce.Code == websocket.StatusNormalClosure
		if cleanClose {
			s.cfg.logger.Info("schwab: stream closed cleanly; not reconnecting")
			return
		}

		uptime := s.nowFn().Sub(connectedAt)
		if uptime < stableConnectionThreshold {
			s.cfg.logger.Error("schwab: stream closed during unstable window; bailing",
				slog.Duration("uptime", uptime),
				slog.Any("error", closeErr))
			return
		}

		delay := s.bo.Next()
		s.cfg.logger.Warn("schwab: stream dropped; reconnecting",
			slog.Duration("backoff", delay),
			slog.Any("error", closeErr))
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		if err := s.connectAndLogin(ctx); err != nil {
			s.cfg.logger.Error("schwab: reconnect failed", slog.Any("error", err))
			// loop again; next Next() doubles the backoff.
			continue
		}
	}
}

// readLoop reads frames from conn until error. It dispatches each frame to
// the configured raw and typed handlers. Returns the terminating error.
func (s *Streamer) readLoop(ctx context.Context, conn *websocket.Conn) error {
	if conn == nil {
		return errors.New("schwab: nil connection")
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if h := s.cfg.rawHandler; h != nil {
			h(data)
		}
		if h := s.cfg.typedHandler; h != nil {
			var msg Message
			if perr := json.Unmarshal(data, &msg); perr == nil {
				h(ctx, &msg)
			} else {
				s.cfg.logger.Warn("schwab: stream parse failed",
					slog.Any("error", perr))
			}
		}
	}
}
