package feed

import (
	"testing"
	"time"
)

var now = time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)

func TestDiff_GPS(t *testing.T) {
	prev := map[string]any{
		"id": "usr_1", "displayName": "Alice",
		"location": "wrld_abc:12345~public~region(us)",
		"status": "active", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": "",
	}
	next := map[string]any{
		"id": "usr_1", "displayName": "Alice",
		"location": "wrld_xyz:99999~friends~region(jp)",
		"status": "active", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": "",
	}

	entries := Diff("viewer_1", prev, "online", next, "online", now)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	gps, ok := entries[0].(GPSEntry)
	if !ok {
		t.Fatalf("expected GPSEntry, got %T", entries[0])
	}
	if gps.Location != next["location"] {
		t.Errorf("Location = %q, want %q", gps.Location, next["location"])
	}
	if gps.PreviousLocation != prev["location"] {
		t.Errorf("PreviousLocation = %q, want %q", gps.PreviousLocation, prev["location"])
	}
}

func TestDiff_Status(t *testing.T) {
	prev := map[string]any{
		"id": "usr_1", "displayName": "Bob",
		"location": "wrld_abc:12345",
		"status": "active", "statusDescription": "old desc", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": "",
	}
	next := map[string]any{
		"id": "usr_1", "displayName": "Bob",
		"location": "wrld_abc:12345",
		"status": "join me", "statusDescription": "new desc", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": "",
	}

	entries := Diff("viewer_1", prev, "online", next, "online", now)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	se, ok := entries[0].(StatusEntry)
	if !ok {
		t.Fatalf("expected StatusEntry, got %T", entries[0])
	}
	if se.Status != "join me" || se.PreviousStatus != "active" {
		t.Errorf("got Status=%q PreviousStatus=%q", se.Status, se.PreviousStatus)
	}
}

func TestDiff_Online(t *testing.T) {
	prev := map[string]any{"id": "usr_1", "displayName": "Carol", "location": "offline", "status": "offline", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}
	next := map[string]any{"id": "usr_1", "displayName": "Carol", "location": "wrld_abc:12345", "status": "active", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}

	entries := Diff("viewer_1", prev, "offline", next, "online", now)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}
	oo, ok := entries[0].(OnlineOfflineEntry)
	if !ok {
		t.Fatalf("expected OnlineOfflineEntry, got %T", entries[0])
	}
	if oo.Type != "Online" {
		t.Errorf("Type = %q, want Online", oo.Type)
	}
}

func TestDiff_Offline(t *testing.T) {
	prev := map[string]any{"id": "usr_1", "displayName": "Dave", "location": "wrld_abc:12345", "status": "active", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}
	next := map[string]any{"id": "usr_1", "displayName": "Dave", "location": "offline", "status": "offline", "statusDescription": "", "bio": "", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}

	entries := Diff("viewer_1", prev, "online", next, "offline", now)
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}
	oo, ok := entries[0].(OnlineOfflineEntry)
	if !ok {
		t.Fatalf("expected OnlineOfflineEntry, got %T", entries[0])
	}
	if oo.Type != "Offline" {
		t.Errorf("Type = %q, want Offline", oo.Type)
	}
}

func TestDiff_Bio(t *testing.T) {
	prev := map[string]any{"id": "usr_1", "displayName": "Eve", "location": "offline", "status": "active", "statusDescription": "", "bio": "old bio", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}
	next := map[string]any{"id": "usr_1", "displayName": "Eve", "location": "offline", "status": "active", "statusDescription": "", "bio": "new bio", "currentAvatarImageUrl": "", "currentAvatarThumbnailImageUrl": ""}

	entries := Diff("viewer_1", prev, "active", next, "active", now)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	be, ok := entries[0].(BioEntry)
	if !ok {
		t.Fatalf("expected BioEntry, got %T", entries[0])
	}
	if be.Bio != "new bio" || be.PreviousBio != "old bio" {
		t.Errorf("Bio=%q PreviousBio=%q", be.Bio, be.PreviousBio)
	}
}

func TestDiff_NoOp(t *testing.T) {
	snap := map[string]any{"id": "usr_1", "displayName": "Frank", "location": "wrld_abc:12345", "status": "active", "statusDescription": "same", "bio": "same", "currentAvatarImageUrl": "http://img/a", "currentAvatarThumbnailImageUrl": "http://img/b"}
	entries := Diff("viewer_1", snap, "online", snap, "online", now)
	if len(entries) != 0 {
		t.Errorf("expected no entries for identical snapshots, got %d: %v", len(entries), entries)
	}
}
