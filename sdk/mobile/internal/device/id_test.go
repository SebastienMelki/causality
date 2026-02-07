package device

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
)

func newTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetOrCreateDeviceID_CreatesOnFirstCall(t *testing.T) {
	db := newTestDB(t)
	idMgr := NewIDManager(db, false)

	id := idMgr.GetOrCreateDeviceID()
	if id == "" {
		t.Fatal("expected non-empty device ID")
	}

	// Verify it looks like a UUID (36 chars, 4 dashes).
	if len(id) != 36 {
		t.Fatalf("expected UUID (36 chars), got %q (%d chars)", id, len(id))
	}
}

func TestGetOrCreateDeviceID_ReturnsSameOnSecondCall(t *testing.T) {
	db := newTestDB(t)
	idMgr := NewIDManager(db, false)

	id1 := idMgr.GetOrCreateDeviceID()
	id2 := idMgr.GetOrCreateDeviceID()

	if id1 != id2 {
		t.Fatalf("expected same ID on second call, got %q and %q", id1, id2)
	}
}

func TestGetOrCreateDeviceID_PersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First instance: create device ID.
	db1, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 1: %v", err)
	}
	idMgr1 := NewIDManager(db1, false)
	id1 := idMgr1.GetOrCreateDeviceID()
	db1.Close()

	// Second instance: should load same ID from DB.
	db2, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 2: %v", err)
	}
	defer db2.Close()

	idMgr2 := NewIDManager(db2, false)
	id2 := idMgr2.GetOrCreateDeviceID()

	if id1 != id2 {
		t.Fatalf("expected same ID across instances, got %q and %q", id1, id2)
	}
}

func TestRegenerateDeviceID_ReturnsDifferentID(t *testing.T) {
	db := newTestDB(t)
	idMgr := NewIDManager(db, false)

	oldID := idMgr.GetOrCreateDeviceID()
	newID := idMgr.RegenerateDeviceID()

	if oldID == newID {
		t.Fatalf("expected different ID after regenerate, both are %q", oldID)
	}

	// Verify the new ID is now returned by GetOrCreateDeviceID.
	currentID := idMgr.GetOrCreateDeviceID()
	if currentID != newID {
		t.Fatalf("expected GetOrCreateDeviceID to return %q, got %q", newID, currentID)
	}
}

func TestRegenerateDeviceID_PersistsNewID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create and regenerate.
	db1, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 1: %v", err)
	}
	idMgr1 := NewIDManager(db1, false)
	_ = idMgr1.GetOrCreateDeviceID()
	newID := idMgr1.RegenerateDeviceID()
	db1.Close()

	// Reopen and verify.
	db2, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 2: %v", err)
	}
	defer db2.Close()

	idMgr2 := NewIDManager(db2, false)
	loadedID := idMgr2.GetOrCreateDeviceID()

	if loadedID != newID {
		t.Fatalf("expected regenerated ID %q after reopen, got %q", newID, loadedID)
	}
}

func TestConcurrentAccess_Safe(t *testing.T) {
	db := newTestDB(t)
	idMgr := NewIDManager(db, false)

	const goroutines = 50
	var wg sync.WaitGroup
	ids := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ids[idx] = idMgr.GetOrCreateDeviceID()
		}(i)
	}
	wg.Wait()

	// All goroutines should get the same ID.
	expected := ids[0]
	for i, id := range ids {
		if id != expected {
			t.Fatalf("goroutine %d got %q, expected %q", i, id, expected)
		}
	}
}

func TestIsPersistent(t *testing.T) {
	db := newTestDB(t)

	persistent := NewIDManager(db, true)
	if !persistent.IsPersistent() {
		t.Error("expected persistent=true")
	}

	nonPersistent := NewIDManager(db, false)
	if nonPersistent.IsPersistent() {
		t.Error("expected persistent=false")
	}
}
