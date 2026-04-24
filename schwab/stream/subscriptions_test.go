package stream

import (
	"reflect"
	"sort"
	"testing"
)

func mkReq(svc Service, cmd Command, keys, fields string) Request {
	return Request{
		Service:    svc,
		Command:    cmd,
		Parameters: map[string]string{"keys": keys, "fields": fields},
	}
}

func TestRecordSubs(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD,INTC", "0,1,3"))
	snap := s.snapshot()
	if got := snap[LevelOneEquities]["AMD"]; !reflect.DeepEqual(got, []string{"0", "1", "3"}) {
		t.Errorf("SUBS AMD fields = %v; want [0 1 3]", got)
	}
	if got := snap[LevelOneEquities]["INTC"]; !reflect.DeepEqual(got, []string{"0", "1", "3"}) {
		t.Errorf("SUBS INTC fields = %v", got)
	}
}

func TestRecordAddUnionsFields(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdAdd, "AMD", "0,1"))
	s.record(mkReq(LevelOneEquities, CmdAdd, "AMD", "2,3"))
	snap := s.snapshot()
	if got := snap[LevelOneEquities]["AMD"]; !reflect.DeepEqual(got, []string{"0", "1", "2", "3"}) {
		t.Errorf("ADD union AMD fields = %v", got)
	}
}

func TestRecordUnsubsRemovesKeys(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD,INTC", "0,1"))
	s.record(mkReq(LevelOneEquities, CmdUnsubs, "AMD", ""))
	snap := s.snapshot()
	if _, ok := snap[LevelOneEquities]["AMD"]; ok {
		t.Errorf("UNSUBS did not remove AMD")
	}
	if _, ok := snap[LevelOneEquities]["INTC"]; !ok {
		t.Errorf("UNSUBS removed unrelated key INTC")
	}
	// Unsubbing the last key should delete the service entry entirely.
	s.record(mkReq(LevelOneEquities, CmdUnsubs, "INTC", ""))
	if _, ok := snap[LevelOneEquities]; ok {
		// snap was taken before; take another one now.
	}
	snap = s.snapshot()
	if _, ok := snap[LevelOneEquities]; ok {
		t.Errorf("UNSUBS did not delete empty service")
	}
}

func TestRecordViewOverwritesAllKeys(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD,INTC", "0,1,2"))
	s.record(mkReq(LevelOneEquities, CmdView, "", "5,6"))
	snap := s.snapshot()
	for _, k := range []string{"AMD", "INTC"} {
		if got := snap[LevelOneEquities][k]; !reflect.DeepEqual(got, []string{"5", "6"}) {
			t.Errorf("VIEW %s fields = %v; want [5 6]", k, got)
		}
	}
}

func TestSnapshotDeepCopy(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD", "0,1"))
	snap := s.snapshot()
	snap[LevelOneEquities]["AMD"][0] = "MUTATED"
	snap2 := s.snapshot()
	if snap2[LevelOneEquities]["AMD"][0] == "MUTATED" {
		t.Errorf("snapshot did not return a deep copy")
	}
}

func TestReplayRequestsGroupsByFieldSet(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD,INTC", "0,1"))
	s.record(mkReq(LevelOneEquities, CmdAdd, "NVDA", "0,1,2"))
	reqs := s.replayRequests()
	if len(reqs) != 2 {
		t.Fatalf("replay len = %d; want 2", len(reqs))
	}
	// Sort results by fields for deterministic assertions.
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Parameters["fields"] < reqs[j].Parameters["fields"] })
	if reqs[0].Parameters["fields"] != "0,1" {
		t.Errorf("group 0 fields = %q; want 0,1", reqs[0].Parameters["fields"])
	}
	if reqs[0].Parameters["keys"] != "AMD,INTC" {
		t.Errorf("group 0 keys = %q; want AMD,INTC", reqs[0].Parameters["keys"])
	}
	if reqs[1].Parameters["fields"] != "0,1,2" {
		t.Errorf("group 1 fields = %q; want 0,1,2", reqs[1].Parameters["fields"])
	}
	if reqs[1].Parameters["keys"] != "NVDA" {
		t.Errorf("group 1 keys = %q; want NVDA", reqs[1].Parameters["keys"])
	}
	for _, r := range reqs {
		if r.Command != CmdAdd {
			t.Errorf("replay command = %q; want ADD", r.Command)
		}
	}
}

func TestClearSubs(t *testing.T) {
	s := newSubscriptionState()
	s.record(mkReq(LevelOneEquities, CmdSubs, "AMD,INTC", "0,1"))
	s.clear()
	if snap := s.snapshot(); len(snap) != 0 {
		t.Errorf("clear() left %d entries", len(snap))
	}
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,c,", []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		got := splitCSV(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitCSV(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}
