package main

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRecordsSeparateRegistrationConfirmationAndReplay(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	st := Station{Callsign: "ZZ0ENTRY", Name: "Example", City: "Exampletown", Notes: "Permanent"}
	key, _ := newSubmissionKey()
	qid, err := s.saveRecord(ctx, st, "", "", "", "Draft notes", key, false)
	if err != nil || qid != 0 {
		t.Fatalf("registration: %d %v", qid, err)
	}
	contacts, err := s.contacts(ctx, "")
	if err != nil || len(contacts) != 0 {
		t.Fatal("registration confirmed a QSO", err)
	}
	key, _ = newSubmissionKey()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.saveRecord(ctx, st, "DMR", "70 cm", "Example round", "Conversation", key, true); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	contacts, err = s.contacts(ctx, "")
	if err != nil || len(contacts) != 1 || contacts[0].Band != "70 cm" || contacts[0].RoundName != "Example round" || contacts[0].Notes != "Conversation" {
		t.Fatalf("confirmation: %+v %v", contacts, err)
	}
	saved, err := s.lookup(ctx, st.Callsign)
	if err != nil || saved.Notes != "Permanent" {
		t.Fatal("permanent notes overwritten", err)
	}
	saved.Name = "Updated"
	if _, err = s.saveStation(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if _, err = s.saveRecord(ctx, st, "FM", "2 m", "Changed", "Changed", key, true); err != nil {
		t.Fatal(err)
	}
	current, _ := s.lookup(ctx, st.Callsign)
	if current.Name != "Updated" {
		t.Fatal("replay overwrote registration")
	}
	snapshot, _ := s.contact(ctx, contacts[0].ID)
	if snapshot != contacts[0] {
		t.Fatal("replay changed historical contact")
	}
	st.Callsign = "ZZ1OTHER"
	if _, err = s.saveRecord(ctx, st, "FM", "", "", "", key, true); !errors.Is(err, errSubmissionConflict) {
		t.Fatalf("reused form for another operator: %v", err)
	}
	if _, err = s.lookup(ctx, "ZZ1OTHER"); err == nil {
		t.Fatal("conflicting replay inserted an operator")
	}
}

func TestRecordsInvalidConfirmationRollsBack(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	key, _ := newSubmissionKey()
	for _, tc := range []struct{ mode, band, key string }{{"", "2 m", key}, {"FM", "bad\x00band", key}, {"FM", "2 m", "invalid"}} {
		if _, err := s.saveRecord(ctx, Station{Callsign: "ZZ0INVALID"}, tc.mode, tc.band, "", "", tc.key, true); err == nil {
			t.Fatal("invalid record accepted")
		}
	}
	if _, err := s.lookup(ctx, "ZZ0INVALID"); err == nil {
		t.Fatal("invalid record saved an operator")
	}
}
