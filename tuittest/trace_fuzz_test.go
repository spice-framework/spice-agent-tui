package tuittest

import (
	"bytes"
	"testing"
)

func FuzzTraceCanonicalReplay(f *testing.F) {
	typeEvent, err := NewTypeEvent("owners")
	if err != nil {
		f.Fatal(err)
	}
	resizeEvent, err := NewResizeEvent(40, 10)
	if err != nil {
		f.Fatal(err)
	}
	snapshotEvent, err := NewSnapshotEvent("owners-typed")
	if err != nil {
		f.Fatal(err)
	}
	seed, err := NewTrace(TraceOptions{Width: 32, Height: 8}, []TraceEvent{
		typeEvent, resizeEvent, snapshotEvent,
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.CanonicalJSON())
	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 16<<10 {
			t.Skip()
		}
		trace, err := ParseTrace(content)
		if err != nil {
			return
		}
		if !bytes.Equal(content, trace.CanonicalJSON()) {
			t.Fatal("strict parser accepted noncanonical trace")
		}
		replay, err := trace.Replay()
		if err != nil {
			t.Fatal(err)
		}
		if replay.TraceDigest() != trace.Digest() || len(replay.Steps()) != len(trace.Events()) {
			t.Fatal("replay evidence does not cover the canonical trace")
		}
	})
}
