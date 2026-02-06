package identity

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

func TestSetUser_StoresIdentity(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	traits := map[string]interface{}{
		"name": "Alice",
		"plan": "premium",
	}
	aliases := []string{"alice@example.com", "alice-legacy-123"}

	if err := mgr.SetUser("user-123", traits, aliases); err != nil {
		t.Fatalf("SetUser: %v", err)
	}

	user := mgr.GetUser()
	if user == nil {
		t.Fatal("expected non-nil user after SetUser")
	}
	if user.UserID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %q", user.UserID)
	}
	if user.Traits["name"] != "Alice" {
		t.Errorf("expected trait name 'Alice', got %v", user.Traits["name"])
	}
	if user.Traits["plan"] != "premium" {
		t.Errorf("expected trait plan 'premium', got %v", user.Traits["plan"])
	}
	if len(user.Aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(user.Aliases))
	}
	if user.Aliases[0] != "alice@example.com" {
		t.Errorf("expected alias[0] 'alice@example.com', got %q", user.Aliases[0])
	}
}

func TestSetUser_EmptyUserID(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	err := mgr.SetUser("", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty user ID")
	}
}

func TestSetUser_NilTraitsAndAliases(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.SetUser("user-456", nil, nil); err != nil {
		t.Fatalf("SetUser: %v", err)
	}

	user := mgr.GetUser()
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.UserID != "user-456" {
		t.Errorf("expected user_id 'user-456', got %q", user.UserID)
	}
}

func TestGetUser_ReturnsSetIdentity(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.SetUser("user-789", map[string]interface{}{"key": "value"}, nil); err != nil {
		t.Fatalf("SetUser: %v", err)
	}

	user := mgr.GetUser()
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.UserID != "user-789" {
		t.Errorf("expected user_id 'user-789', got %q", user.UserID)
	}
	if user.Traits["key"] != "value" {
		t.Errorf("expected trait key 'value', got %v", user.Traits["key"])
	}
}

func TestGetUser_ReturnsCopy(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.SetUser("user-copy", map[string]interface{}{"k": "v"}, []string{"a"}); err != nil {
		t.Fatalf("SetUser: %v", err)
	}

	user1 := mgr.GetUser()
	user2 := mgr.GetUser()

	// Mutate the copy; should not affect the original.
	user1.Traits["k"] = "mutated"
	user1.Aliases[0] = "mutated"

	user3 := mgr.GetUser()
	if user3.Traits["k"] != "v" {
		t.Errorf("expected original trait 'v', got %v (copy mutation leaked)", user3.Traits["k"])
	}
	if user3.Aliases[0] != "a" {
		t.Errorf("expected original alias 'a', got %q (copy mutation leaked)", user3.Aliases[0])
	}
	_ = user2
}

func TestGetUser_ReturnsNilIfNotSet(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	user := mgr.GetUser()
	if user != nil {
		t.Fatalf("expected nil user before SetUser, got %+v", user)
	}
}

func TestReset_ClearsIdentity(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.SetUser("user-reset", nil, nil); err != nil {
		t.Fatalf("SetUser: %v", err)
	}

	mgr.Reset()

	user := mgr.GetUser()
	if user != nil {
		t.Fatalf("expected nil user after Reset, got %+v", user)
	}
}

func TestReset_ClearsFromDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Set user identity.
	db1, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 1: %v", err)
	}
	mgr1 := NewIdentityManager(db1)
	if err := mgr1.SetUser("user-db-reset", nil, nil); err != nil {
		t.Fatalf("SetUser: %v", err)
	}
	mgr1.Reset()
	db1.Close()

	// Reopen and verify identity is gone.
	db2, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 2: %v", err)
	}
	defer db2.Close()

	mgr2 := NewIdentityManager(db2)
	if err := mgr2.LoadFromDB(); err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}

	user := mgr2.GetUser()
	if user != nil {
		t.Fatalf("expected nil user after Reset + reopen, got %+v", user)
	}
}

func TestLoadFromDB_RestoresIdentity(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Set user identity.
	db1, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 1: %v", err)
	}
	mgr1 := NewIdentityManager(db1)

	traits := map[string]interface{}{"plan": "enterprise"}
	aliases := []string{"alice@corp.com"}
	if err := mgr1.SetUser("user-persist", traits, aliases); err != nil {
		t.Fatalf("SetUser: %v", err)
	}
	db1.Close()

	// Reopen and load from DB.
	db2, err := storage.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB 2: %v", err)
	}
	defer db2.Close()

	mgr2 := NewIdentityManager(db2)
	if err := mgr2.LoadFromDB(); err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}

	user := mgr2.GetUser()
	if user == nil {
		t.Fatal("expected non-nil user after LoadFromDB")
	}
	if user.UserID != "user-persist" {
		t.Errorf("expected user_id 'user-persist', got %q", user.UserID)
	}
	if user.Traits["plan"] != "enterprise" {
		t.Errorf("expected trait plan 'enterprise', got %v", user.Traits["plan"])
	}
	if len(user.Aliases) != 1 || user.Aliases[0] != "alice@corp.com" {
		t.Errorf("expected alias 'alice@corp.com', got %v", user.Aliases)
	}
}

func TestLoadFromDB_NoPersistedIdentity(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.LoadFromDB(); err != nil {
		t.Fatalf("LoadFromDB should succeed with no persisted identity: %v", err)
	}

	user := mgr.GetUser()
	if user != nil {
		t.Fatalf("expected nil user when no identity persisted, got %+v", user)
	}
}

func TestSetUser_OverwritesPrevious(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	if err := mgr.SetUser("user-first", nil, nil); err != nil {
		t.Fatalf("SetUser 1: %v", err)
	}
	if err := mgr.SetUser("user-second", map[string]interface{}{"new": true}, nil); err != nil {
		t.Fatalf("SetUser 2: %v", err)
	}

	user := mgr.GetUser()
	if user.UserID != "user-second" {
		t.Errorf("expected user_id 'user-second', got %q", user.UserID)
	}
}

func TestConcurrentAccess_Safe(t *testing.T) {
	db := newTestDB(t)
	mgr := NewIdentityManager(db)

	const goroutines = 50
	var wg sync.WaitGroup

	// Half the goroutines set users, half read.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = mgr.SetUser("user-concurrent", nil, nil)
			} else {
				_ = mgr.GetUser()
			}
		}(i)
	}
	wg.Wait()

	// After all goroutines complete, user should be set.
	user := mgr.GetUser()
	if user == nil {
		t.Fatal("expected non-nil user after concurrent writes")
	}
	if user.UserID != "user-concurrent" {
		t.Errorf("expected user_id 'user-concurrent', got %q", user.UserID)
	}
}
