package stream

import (
	"sort"
	"strings"
	"sync"
)

// subscriptionState tracks the currently-active subscriptions so the streamer
// can replay them after a reconnect. It is the Go analogue of Schwabdev's
// Stream._record_request.
type subscriptionState struct {
	mu sync.Mutex
	// byService maps service → key → sorted unique field list.
	byService map[Service]map[string][]string
}

// newSubscriptionState returns an empty subscription registry.
func newSubscriptionState() *subscriptionState {
	return &subscriptionState{byService: make(map[Service]map[string][]string)}
}

// record mutates the registry to reflect the effect of a single Request.
// The command semantics are:
//
//	ADD     - union req.Parameters["fields"] into each req.Parameters["keys"]
//	SUBS    - replace the per-key field list for each key
//	UNSUBS  - delete each key (and the service entry if it becomes empty)
//	VIEW    - overwrite the field list for EVERY existing key in the service
//
// Unknown commands (LOGIN, LOGOUT, heartbeat-only) are ignored.
func (s *subscriptionState) record(req Request) {
	keys := splitCSV(req.Parameters["keys"])
	fields := splitCSV(req.Parameters["fields"])

	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Command {
	case CmdAdd:
		byKey, ok := s.byService[req.Service]
		if !ok {
			byKey = make(map[string][]string)
			s.byService[req.Service] = byKey
		}
		for _, k := range keys {
			byKey[k] = unionSorted(byKey[k], fields)
		}
	case CmdSubs:
		byKey := make(map[string][]string, len(keys))
		for _, k := range keys {
			byKey[k] = uniqSorted(fields)
		}
		s.byService[req.Service] = byKey
	case CmdUnsubs:
		byKey, ok := s.byService[req.Service]
		if !ok {
			return
		}
		for _, k := range keys {
			delete(byKey, k)
		}
		if len(byKey) == 0 {
			delete(s.byService, req.Service)
		}
	case CmdView:
		byKey, ok := s.byService[req.Service]
		if !ok {
			return
		}
		sorted := uniqSorted(fields)
		for k := range byKey {
			dup := make([]string, len(sorted))
			copy(dup, sorted)
			byKey[k] = dup
		}
	}
}

// snapshot returns a deep copy of the current subscription state. Callers may
// mutate the returned structure without affecting the tracker.
func (s *subscriptionState) snapshot() map[Service]map[string][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[Service]map[string][]string, len(s.byService))
	for svc, byKey := range s.byService {
		dupByKey := make(map[string][]string, len(byKey))
		for k, fields := range byKey {
			dup := make([]string, len(fields))
			copy(dup, fields)
			dupByKey[k] = dup
		}
		out[svc] = dupByKey
	}
	return out
}

// keys returns the current subscribed keys for a service, sorted.
// Returns nil when the service has no subscriptions.
func (s *subscriptionState) keys(svc Service) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	byKey, ok := s.byService[svc]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(byKey))
	for k := range byKey {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// clear drops every tracked subscription.
func (s *subscriptionState) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byService = make(map[Service]map[string][]string)
}

// replayRequests returns the list of ADD requests required to replay the
// current subscription state after a reconnect. Keys within the same service
// that share identical field lists are batched into a single ADD request.
//
// This mirrors stream.py:94-105 which groups symbols by field set to minimise
// the number of frames sent during replay.
func (s *subscriptionState) replayRequests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Request
	// Sort services for deterministic output.
	services := make([]Service, 0, len(s.byService))
	for svc := range s.byService {
		services = append(services, svc)
	}
	sort.Slice(services, func(i, j int) bool { return services[i] < services[j] })

	for _, svc := range services {
		byKey := s.byService[svc]
		// Group keys by identical field-set signature.
		groups := make(map[string][]string)
		sigOrder := make([]string, 0)
		for k, fields := range byKey {
			sig := strings.Join(fields, ",")
			if _, seen := groups[sig]; !seen {
				sigOrder = append(sigOrder, sig)
			}
			groups[sig] = append(groups[sig], k)
		}
		sort.Strings(sigOrder)
		for _, sig := range sigOrder {
			ks := groups[sig]
			sort.Strings(ks)
			out = append(out, Request{
				Service: svc,
				Command: CmdAdd,
				Parameters: map[string]string{
					"keys":   strings.Join(ks, ","),
					"fields": sig,
				},
			})
		}
	}
	return out
}

// splitCSV splits a comma-separated string into a trimmed non-empty slice.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// uniqSorted returns the sorted unique elements of v.
func uniqSorted(v []string) []string {
	if len(v) == 0 {
		return nil
	}
	cp := make([]string, len(v))
	copy(cp, v)
	sort.Strings(cp)
	out := cp[:0]
	var prev string
	for i, s := range cp {
		if i == 0 || s != prev {
			out = append(out, s)
			prev = s
		}
	}
	return out
}

// unionSorted returns the sorted union of a (already sorted+unique) and b (any).
func unionSorted(a, b []string) []string {
	merged := make([]string, 0, len(a)+len(b))
	merged = append(merged, a...)
	merged = append(merged, b...)
	return uniqSorted(merged)
}
