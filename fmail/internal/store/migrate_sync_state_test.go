package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateSyncState_AllowsOneCheckpointPerMailbox(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE accounts (id INTEGER PRIMARY KEY);
		INSERT INTO accounts (id) VALUES (1);
		CREATE TABLE sync_state (
			account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
			mailbox TEXT NOT NULL,
			last_uid INTEGER NOT NULL DEFAULT 0,
			last_synced_at TEXT
		);
		INSERT INTO sync_state VALUES (1, 'INBOX', 12, NULL);
	`)
	if err != nil {
		t.Fatalf("Altschema anlegen: %v", err)
	}

	s := &Store{db: db}
	if err := s.migrateSyncStatePrimaryKey(); err != nil {
		t.Fatalf("Migration: %v", err)
	}
	if err := s.SetLastUID(1, "Sent", 34); err != nil {
		t.Fatalf("zweiten Ordner speichern: %v", err)
	}

	for mailbox, want := range map[string]uint32{"INBOX": 12, "Sent": 34} {
		got, err := s.GetLastUID(1, mailbox)
		if err != nil || got != want {
			t.Errorf("GetLastUID(%q) = %d, %v; want %d, nil", mailbox, got, err, want)
		}
	}
}
