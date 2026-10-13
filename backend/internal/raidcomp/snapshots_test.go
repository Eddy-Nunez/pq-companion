package raidcomp

import (
	"errors"
	"testing"
)

func TestStore_RosterSnapshots(t *testing.T) {
	s := openTemp(t)

	if _, err := s.SaveSnapshot(Snapshot{}, "empty"); err == nil {
		t.Fatal("SaveSnapshot with no members: want error")
	}

	live := Snapshot{
		ZoneID: 71, Zone: "airplane",
		Members: []Member{
			{Name: "Tanky", Level: 60, Class: 1, Code: "war", Group: "1", Rank: "Raid Leader"},
			{Name: "Healz", Level: 60, Class: 2, Code: "clr", Group: "1"},
			{Name: "Floater", Level: 58, Class: 14, Code: "enc", Group: "0"},
		},
	}
	saved, err := s.SaveSnapshot(live, "  Plane of Sky  ")
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if saved.ID <= 0 || saved.Label != "Plane of Sky" || saved.MemberCount != 3 {
		t.Fatalf("saved = %+v", saved)
	}
	second, err := s.SaveSnapshot(live, "")
	if err != nil {
		t.Fatalf("SaveSnapshot second: %v", err)
	}

	list, err := s.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != second.ID {
		t.Fatalf("list = %+v, want 2 newest-first", list)
	}
	if list[0].Members != nil {
		t.Fatal("list rows should not carry members")
	}

	got, err := s.GetSnapshot(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Zone != "airplane" || len(got.Members) != 3 || got.Members[0] != live.Members[0] {
		t.Fatalf("got = %+v", got)
	}

	if err := s.RenameSnapshot(saved.ID, "PoSky clear"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetSnapshot(saved.ID)
	if got.Label != "PoSky clear" {
		t.Fatalf("label = %q", got.Label)
	}
	if err := s.RenameSnapshot(9999, "x"); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("rename missing: err = %v", err)
	}

	if err := s.DeleteSnapshot(saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSnapshot(saved.ID); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("get deleted: err = %v", err)
	}
}
