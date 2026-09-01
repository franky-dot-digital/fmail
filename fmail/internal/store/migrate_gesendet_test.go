package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateMailsFolderConstraint_PreservesDataAndAttachments simuliert
// eine Datenbank von vor der "Gesendet"-Ansicht (Schema ohne recipients-
// Spalte, CHECK ohne 'gesendet') und prüft, dass migrateMailsFolderConstraint
// (a) bestehende Mails samt body_html unangetastet lässt, (b) die
// Fremdschlüsselbeziehung zu attachments nach dem Tabellen-Neuaufbau
// intakt bleibt, und (c) danach tatsächlich Mails mit folder='gesendet'
// eingefügt werden können.
func TestMigrateMailsFolderConstraint_PreservesDataAndAttachments(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("DB öffnen: %v", err)
	}
	defer db.Close()

	// Altes Schema von Hand anlegen -- exakt der Stand vor dieser Änderung.
	oldSchema := `
	CREATE TABLE accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		label TEXT NOT NULL, email TEXT NOT NULL, display_name TEXT NOT NULL DEFAULT '',
		imap_host TEXT NOT NULL, imap_port INTEGER NOT NULL, imap_security TEXT NOT NULL,
		smtp_host TEXT NOT NULL, smtp_port INTEGER NOT NULL, smtp_security TEXT NOT NULL,
		username TEXT NOT NULL, smtp_username TEXT NOT NULL DEFAULT '',
		sync_interval_minutes INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
	);
	CREATE TABLE mails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		uid INTEGER NOT NULL,
		mailbox TEXT NOT NULL,
		folder TEXT NOT NULL CHECK (folder IN ('post','ablage','pfoertner')),
		sender_name TEXT NOT NULL, sender_email TEXT NOT NULL,
		subject TEXT NOT NULL, preview TEXT NOT NULL, body_text TEXT NOT NULL,
		body_html TEXT NOT NULL DEFAULT '',
		received_at TEXT NOT NULL, unread INTEGER NOT NULL DEFAULT 1, has_attachment INTEGER NOT NULL DEFAULT 0,
		UNIQUE(account_id, mailbox, uid)
	);
	CREATE TABLE attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mail_id INTEGER NOT NULL REFERENCES mails(id) ON DELETE CASCADE,
		filename TEXT NOT NULL, content_type TEXT NOT NULL, size INTEGER NOT NULL, data BLOB NOT NULL
	);`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatalf("altes Schema anlegen: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO accounts (id, label, email, imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security, username, created_at) VALUES (1, 'Privat', 'ich@example.com', 'imap.example.com', 993, 'tls', 'smtp.example.com', 465, 'tls', 'ich@example.com', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("Account einfügen: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mails (id, account_id, uid, mailbox, folder, sender_name, sender_email, subject, preview, body_text, body_html, received_at) VALUES (1, 1, 42, 'INBOX', 'post', 'Alice', 'alice@example.com', 'Betreff', 'Vorschau', 'Klartext', '<p>Hallo</p>', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("Mail einfügen: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO attachments (mail_id, filename, content_type, size, data) VALUES (1, 'bild.png', 'image/png', 3, x'010203')`); err != nil {
		t.Fatalf("Anhang einfügen: %v", err)
	}

	// New() ruft migrate() -> migrateMailsFolderConstraint() auf.
	s, err := New(db)
	if err != nil {
		t.Fatalf("New()/migrate(): %v", err)
	}

	// (a) Bestehende Mail unangetastet.
	m, err := s.GetMail(1)
	if err != nil {
		t.Fatalf("GetMail: %v", err)
	}
	if m.Subject != "Betreff" || m.BodyHTML != "<p>Hallo</p>" || m.SenderEmail != "alice@example.com" {
		t.Errorf("Mail nach Migration verändert: %+v", m)
	}

	// (b) Attachment-FK intakt -- ListAttachments muss den Anhang noch finden.
	atts, err := s.ListAttachments(1)
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(atts) != 1 || atts[0].Filename != "bild.png" {
		t.Errorf("Anhang nach Migration nicht mehr verknüpft: %+v", atts)
	}

	// (c) folder='gesendet' ist jetzt zulässig.
	sentID, err := s.InsertSentMail(1, Mail{
		SenderName: "Ich", SenderEmail: "ich@example.com", Recipients: "bob@example.com, carol@example.com",
		Subject: "Re: Hallo", Preview: "Vorschau", BodyText: "Text",
	})
	if err != nil {
		t.Fatalf("InsertSentMail nach Migration: %v", err)
	}
	sent, err := s.GetMail(sentID)
	if err != nil {
		t.Fatalf("GetMail(sent): %v", err)
	}
	if sent.Folder != FolderGesendet || sent.Recipients != "bob@example.com, carol@example.com" {
		t.Errorf("gesendete Mail falsch gespeichert: %+v", sent)
	}

	// Auch eine vom IMAP-Gesendet-Ordner importierte Mail muss ihre
	// Empfänger behalten und über Serverordner+UID deduplizierbar sein.
	remoteID, err := s.InsertMail(1, "Sent", Mail{
		UID: 99, Folder: FolderGesendet, SenderName: "Ich", SenderEmail: "ich@example.com",
		Recipients: "dave@example.com", Subject: "Serverkopie", BodyText: "Text",
	})
	if err != nil {
		t.Fatalf("remote Gesendet-Mail einfügen: %v", err)
	}
	remote, err := s.GetMail(remoteID)
	if err != nil || remote.Recipients != "dave@example.com" {
		t.Fatalf("Empfänger der Serverkopie nicht gespeichert: mail=%+v err=%v", remote, err)
	}
	exists, err := s.HasRemoteMail(1, "Sent", 99)
	if err != nil || !exists {
		t.Fatalf("HasRemoteMail = %v, %v; want true, nil", exists, err)
	}

	// Erneuter migrate()-Lauf (z. B. beim nächsten Programmstart) muss ein
	// No-Op sein und darf nicht erneut die Tabelle umbauen bzw. fehlschlagen.
	if _, err := New(db); err != nil {
		t.Fatalf("zweiter New()/migrate()-Lauf: %v", err)
	}
}
