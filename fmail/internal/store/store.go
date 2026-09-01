// Package store kapselt die gesamte SQLite-Persistenz von f:mail.
//
// Sicherheitsregel für diese Datei: JEDE Query läuft über Platzhalter
// ("?"), niemals über string-Konkatenation mit Nutzereingaben oder
// IMAP-Daten (Absendername, Betreff, ... könnten böswillig gestaltet
// sein). Das schließt SQL-Injection als Angriffsvektor kategorisch aus,
// unabhängig davon, was in einer Mail steht.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

// New übernimmt eine bereits geöffnete *sql.DB (Treiberwahl bleibt bei
// main.go, siehe dortigen Kommentar zu modernc.org/sqlite) und legt das
// Schema an, falls es noch nicht existiert.
func New(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, fmt.Errorf("store: foreign_keys aktivieren: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS accounts (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		label          TEXT NOT NULL,
		email          TEXT NOT NULL,
		display_name   TEXT NOT NULL DEFAULT '',
		imap_host      TEXT NOT NULL,
		imap_port      INTEGER NOT NULL,
		imap_security  TEXT NOT NULL CHECK (imap_security IN ('tls','starttls')),
		smtp_host      TEXT NOT NULL,
		smtp_port      INTEGER NOT NULL,
		smtp_security  TEXT NOT NULL CHECK (smtp_security IN ('tls','starttls')),
		username       TEXT NOT NULL, -- IMAP-Benutzername
		smtp_username  TEXT NOT NULL DEFAULT '',
		sync_interval_minutes INTEGER NOT NULL DEFAULT 0, -- 0 = nur manuell
		created_at     TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sync_state (
		account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		mailbox        TEXT NOT NULL,
		last_uid       INTEGER NOT NULL DEFAULT 0,
		last_synced_at TEXT,
		PRIMARY KEY (account_id, mailbox)
	);

	CREATE TABLE IF NOT EXISTS senders (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		email          TEXT NOT NULL,
		name           TEXT NOT NULL,
		status         TEXT NOT NULL CHECK (status IN ('pending','approved','blocked')) DEFAULT 'pending',
		first_seen     TEXT NOT NULL,
		UNIQUE(account_id, email)
	);

	CREATE TABLE IF NOT EXISTS mails (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		uid            INTEGER NOT NULL,
		mailbox        TEXT NOT NULL,
		folder         TEXT NOT NULL CHECK (folder IN ('post','ablage','pfoertner','gesendet')),
		sender_name    TEXT NOT NULL,
		sender_email   TEXT NOT NULL,
		recipients     TEXT NOT NULL DEFAULT '', -- nur bei folder='gesendet' befüllt (kommagetrennte An-Adressen), siehe InsertSentMail
		subject        TEXT NOT NULL,
		preview        TEXT NOT NULL,
		body_text      TEXT NOT NULL,
		body_html      TEXT NOT NULL DEFAULT '', -- bereits sanitiert (SanitizeHTML), leer wenn die Mail keine HTML-Variante hatte
		received_at    TEXT NOT NULL,
		unread         INTEGER NOT NULL DEFAULT 1,
		has_attachment INTEGER NOT NULL DEFAULT 0,
		UNIQUE(account_id, mailbox, uid)
	);

	-- "Echte" Anhänge (Content-Disposition: attachment). Inline-Bilder
	-- landen NICHT hier, sondern direkt als data:-URI in mails.body_html
	-- (siehe internal/mail/sanitize.go, InlineCIDImages).
	CREATE TABLE IF NOT EXISTS attachments (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		mail_id        INTEGER NOT NULL REFERENCES mails(id) ON DELETE CASCADE,
		filename       TEXT NOT NULL,
		content_type   TEXT NOT NULL,
		size           INTEGER NOT NULL,
		data           BLOB NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_mails_folder ON mails(account_id, folder, received_at DESC);
	CREATE INDEX IF NOT EXISTS idx_senders_status ON senders(account_id, status);
	CREATE INDEX IF NOT EXISTS idx_attachments_mail ON attachments(mail_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if err := s.migrateSyncStatePrimaryKey(); err != nil {
		return err
	}
	if err := s.migrateAccountsColumns(); err != nil {
		return err
	}
	if err := s.migrateMailsColumns(); err != nil {
		return err
	}
	return s.migrateMailsFolderConstraint()
}

// migrateSyncStatePrimaryKey korrigiert das historische Schema, in dem
// account_id allein der Primärschlüssel war. Damit konnte ein Konto nicht
// gleichzeitig einen Sync-Stand für INBOX und Gesendet speichern.
func (s *Store) migrateSyncStatePrimaryKey() error {
	rows, err := s.db.Query(`PRAGMA table_info(sync_state)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	composite := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "mailbox" && pk == 2 {
			composite = true
		}
	}
	if err := rows.Err(); err != nil || composite {
		return err
	}

	_, err = s.db.Exec(`
		BEGIN;
		CREATE TABLE sync_state_new (
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			mailbox TEXT NOT NULL,
			last_uid INTEGER NOT NULL DEFAULT 0,
			last_synced_at TEXT,
			PRIMARY KEY (account_id, mailbox)
		);
		INSERT INTO sync_state_new (account_id, mailbox, last_uid, last_synced_at)
			SELECT account_id, mailbox, last_uid, last_synced_at FROM sync_state;
		DROP TABLE sync_state;
		ALTER TABLE sync_state_new RENAME TO sync_state;
		COMMIT;
	`)
	return err
}

// migrateMailsFolderConstraint erweitert die CHECK-Constraint auf
// mails.folder um 'gesendet' und ergänzt die recipients-Spalte, auf
// Datenbanken, die vor der "Gesendet"-Ansicht angelegt wurden. SQLite kennt
// kein ALTER TABLE ... ALTER CONSTRAINT — die Tabelle wird deshalb nach dem
// Standardmuster (neue Tabelle mit neuer Constraint, Daten kopieren, alte
// löschen, umbenennen) neu aufgebaut, aber nur, wenn nötig: die aktuelle
// Tabellendefinition wird per sqlite_master darauf geprüft, ob 'gesendet'
// bereits im CHECK steht. migrateMailsColumns MUSS vorher gelaufen sein,
// damit body_html beim Kopieren garantiert existiert.
func (s *Store) migrateMailsFolderConstraint() error {
	var ddl string
	if err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'mails'`).Scan(&ddl); err != nil {
		return err
	}
	if strings.Contains(ddl, "'gesendet'") {
		return nil
	}

	// PRAGMA foreign_keys wirkt pro Verbindung und ist wirkungslos, wenn es
	// innerhalb einer Transaktion gesetzt wird (SQLite-Dokumentation) — und
	// über den Pool (*sql.DB) könnte ein Exec/Begin ohnehin auf
	// unterschiedlichen Connections landen. Deshalb hier eine einzelne
	// Connection pinnen (sql.Conn) und die PRAGMA VOR Begin() darauf setzen.
	// Bliebe es bei ON, würde DROP TABLE mails weiter unten ein implizites
	// DELETE FROM mails ausführen und darüber die ON DELETE CASCADE-Regel
	// auf attachments auslösen — alle Anhänge wären weg, bevor die neue
	// Tabelle überhaupt existiert (per Test abgesichert, siehe
	// migrate_gesendet_test.go).
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE mails_new (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			uid            INTEGER NOT NULL,
			mailbox        TEXT NOT NULL,
			folder         TEXT NOT NULL CHECK (folder IN ('post','ablage','pfoertner','gesendet')),
			sender_name    TEXT NOT NULL,
			sender_email   TEXT NOT NULL,
			recipients     TEXT NOT NULL DEFAULT '',
			subject        TEXT NOT NULL,
			preview        TEXT NOT NULL,
			body_text      TEXT NOT NULL,
			body_html      TEXT NOT NULL DEFAULT '',
			received_at    TEXT NOT NULL,
			unread         INTEGER NOT NULL DEFAULT 1,
			has_attachment INTEGER NOT NULL DEFAULT 0,
			UNIQUE(account_id, mailbox, uid)
		)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO mails_new (id, account_id, uid, mailbox, folder, sender_name, sender_email,
			recipients, subject, preview, body_text, body_html, received_at, unread, has_attachment)
		SELECT id, account_id, uid, mailbox, folder, sender_name, sender_email,
			'', subject, preview, body_text, body_html, received_at, unread, has_attachment
		FROM mails`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE mails`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE mails_new RENAME TO mails`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mails_folder ON mails(account_id, folder, received_at DESC)`); err != nil {
		return err
	}
	return tx.Commit()
}

// migrateMailsColumns holt body_html auf Datenbanken nach, die vor der
// HTML-Anzeige angelegt wurden — analog zu migrateAccountsColumns.
func (s *Store) migrateMailsColumns() error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('mails') WHERE name = 'body_html'`).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	_, err := s.db.Exec(`ALTER TABLE mails ADD COLUMN body_html TEXT NOT NULL DEFAULT ''`)
	return err
}

// migrateAccountsColumns holt Spalten nach, die auf Datenbanken fehlen,
// die vor ihrer Einführung angelegt wurden. CREATE TABLE IF NOT EXISTS
// rührt eine bestehende accounts-Tabelle nicht an, daher die expliziten
// ALTER-TABLE-Schritte — abgesichert über pragma_table_info, damit ein
// erneuter Start nicht mit "duplicate column name" fehlschlägt.
func (s *Store) migrateAccountsColumns() error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"display_name", `ALTER TABLE accounts ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`},
		{"smtp_username", `ALTER TABLE accounts ADD COLUMN smtp_username TEXT NOT NULL DEFAULT ''`},
		{"sync_interval_minutes", `ALTER TABLE accounts ADD COLUMN sync_interval_minutes INTEGER NOT NULL DEFAULT 0`},
	}
	for _, c := range columns {
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('accounts') WHERE name = ?`, c.name).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		if _, err := s.db.Exec(c.ddl); err != nil {
			return err
		}
	}

	// Reconciliation statt Einmal-Backfill: läuft bei JEDEM Start, ist
	// aber ein No-Op sobald smtp_username gesetzt ist. Vor dieser Spalte
	// gab es nur ein gemeinsames "username" für IMAP+SMTP — ohne diesen
	// Schritt stünden bestehende Konten mit leerem SMTP-Benutzernamen da,
	// und dialSMTP() (smtpclient.go) überspringt AUTH dann stillschweigend.
	// Als reiner Einmal-Backfill direkt nach dem ALTER TABLE hätte das
	// eine frühere Version dieser Migration verpasst, wenn die Spalte
	// bereits (fehlerhaft) angelegt war — daher unconditional bei jedem
	// Start, nicht nur beim Anlegen der Spalte selbst.
	_, err := s.db.Exec(`UPDATE accounts SET smtp_username = username WHERE smtp_username = ''`)
	return err
}

// ---------- Accounts ----------

type Account struct {
	ID                  int64
	Label               string
	Email               string
	DisplayName         string // erscheint als Absendername ("Von") in gesendeten Mails
	IMAPHost            string
	IMAPPort            int
	IMAPSecurity        string // "tls" | "starttls"
	IMAPUsername        string
	SMTPHost            string
	SMTPPort            int
	SMTPSecurity        string
	SMTPUsername        string
	SyncIntervalMinutes int // 0 = nur manueller Sync
	CreatedAt           time.Time
}

func (s *Store) CreateAccount(a Account) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO accounts
			(label, email, display_name, imap_host, imap_port, imap_security,
			 smtp_host, smtp_port, smtp_security, username, smtp_username,
			 sync_interval_minutes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Label, a.Email, a.DisplayName, a.IMAPHost, a.IMAPPort, a.IMAPSecurity,
		a.SMTPHost, a.SMTPPort, a.SMTPSecurity, a.IMAPUsername, a.SMTPUsername,
		a.SyncIntervalMinutes, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(`
		SELECT id, label, email, display_name, imap_host, imap_port, imap_security,
		       smtp_host, smtp_port, smtp_security, username, smtp_username,
		       sync_interval_minutes, created_at
		FROM accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Account
	for rows.Next() {
		var a Account
		var created string
		if err := rows.Scan(&a.ID, &a.Label, &a.Email, &a.DisplayName, &a.IMAPHost, &a.IMAPPort, &a.IMAPSecurity,
			&a.SMTPHost, &a.SMTPPort, &a.SMTPSecurity, &a.IMAPUsername, &a.SMTPUsername,
			&a.SyncIntervalMinutes, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAccount(id int64) (Account, error) {
	var a Account
	var created string
	err := s.db.QueryRow(`
		SELECT id, label, email, display_name, imap_host, imap_port, imap_security,
		       smtp_host, smtp_port, smtp_security, username, smtp_username,
		       sync_interval_minutes, created_at
		FROM accounts WHERE id = ?`, id).
		Scan(&a.ID, &a.Label, &a.Email, &a.DisplayName, &a.IMAPHost, &a.IMAPPort, &a.IMAPSecurity,
			&a.SMTPHost, &a.SMTPPort, &a.SMTPSecurity, &a.IMAPUsername, &a.SMTPUsername,
			&a.SyncIntervalMinutes, &created)
	if err != nil {
		return Account{}, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return a, nil
}

// UpdateAccount aktualisiert Label, Anzeigename und Verbindungsdaten
// eines bestehenden Kontos. Passwörter liegen separat im Schlüsselbund
// und werden hier nicht angefasst — das übernimmt der Aufrufer nur dann,
// wenn tatsächlich ein neues Passwort eingegeben wurde (siehe
// accountservice.go UpdateAccount).
func (s *Store) UpdateAccount(id int64, a Account) error {
	_, err := s.db.Exec(`
		UPDATE accounts SET
			label = ?, email = ?, display_name = ?,
			imap_host = ?, imap_port = ?, imap_security = ?, username = ?,
			smtp_host = ?, smtp_port = ?, smtp_security = ?, smtp_username = ?,
			sync_interval_minutes = ?
		WHERE id = ?`,
		a.Label, a.Email, a.DisplayName,
		a.IMAPHost, a.IMAPPort, a.IMAPSecurity, a.IMAPUsername,
		a.SMTPHost, a.SMTPPort, a.SMTPSecurity, a.SMTPUsername,
		a.SyncIntervalMinutes, id)
	return err
}

// DeleteAccount entfernt das Konto und kaskadiert per Fremdschlüssel auf
// mails, senders und sync_state. Die im OS-Schlüsselbund liegenden
// Passwörter kaskadieren nicht mit — die muss der Aufrufer separat über
// internal/secrets löschen (siehe accountservice.go).
func (s *Store) DeleteAccount(id int64) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	return err
}

// ---------- Sync-Status (letzte gesehene UID pro Konto/Mailbox) ----------

func (s *Store) GetLastUID(accountID int64, mailbox string) (uint32, error) {
	var last int64
	err := s.db.QueryRow(`SELECT last_uid FROM sync_state WHERE account_id = ? AND mailbox = ?`,
		accountID, mailbox).Scan(&last)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if last < 0 || last > math.MaxUint32 {
		return 0, fmt.Errorf("ungültige gespeicherte IMAP-UID: %d", last)
	}
	return uint32(last), nil
}

func (s *Store) SetLastUID(accountID int64, mailbox string, uid uint32) error {
	_, err := s.db.Exec(`
		INSERT INTO sync_state (account_id, mailbox, last_uid, last_synced_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, mailbox) DO UPDATE SET
			last_uid = excluded.last_uid,
			last_synced_at = excluded.last_synced_at`,
		accountID, mailbox, uid, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ---------- Sender / Pförtner ----------

type SenderStatus string

const (
	StatusPending  SenderStatus = "pending"
	StatusApproved SenderStatus = "approved"
	StatusBlocked  SenderStatus = "blocked"
)

// SenderStatus liefert den bekannten Status eines Absenders für ein Konto.
// Der zweite Rückgabewert ist false, wenn der Absender noch nie gesehen wurde.
func (s *Store) SenderStatus(accountID int64, email string) (SenderStatus, bool, error) {
	var status string
	err := s.db.QueryRow(`SELECT status FROM senders WHERE account_id = ? AND email = ?`,
		accountID, email).Scan(&status)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return SenderStatus(status), true, nil
}

func (s *Store) InsertPendingSender(accountID int64, email, name string) error {
	_, err := s.db.Exec(`
		INSERT INTO senders (account_id, email, name, status, first_seen)
		VALUES (?, ?, ?, 'pending', ?)
		ON CONFLICT(account_id, email) DO NOTHING`,
		accountID, email, name, time.Now().UTC().Format(time.RFC3339))
	return err
}

// SetSenderStatus setzt Freigabe/Sperrung. Bei Freigabe werden alle
// bereits im Pförtner liegenden Mails dieses Absenders nach "post"
// verschoben; bei Sperrung werden sie gelöscht (sie sollen so wirken,
// als wären sie nie angekommen).
func (s *Store) SetSenderStatus(accountID int64, email string, status SenderStatus) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE senders SET status = ? WHERE account_id = ? AND email = ?`,
		status, accountID, email); err != nil {
		return err
	}

	switch status {
	case StatusApproved:
		if _, err := tx.Exec(`
			UPDATE mails SET folder = 'post'
			WHERE account_id = ? AND sender_email = ? AND folder = 'pfoertner'`,
			accountID, email); err != nil {
			return err
		}
	case StatusBlocked:
		if _, err := tx.Exec(`
			DELETE FROM mails
			WHERE account_id = ? AND sender_email = ? AND folder = 'pfoertner'`,
			accountID, email); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// PendingSendersAll liefert wartende Absender über alle Konten hinweg —
// f:mail zeigt Post/Ablage/Pförtner als ein gemeinsames Postfach über
// mehrere Konten, siehe mailservice.go.
func (s *Store) PendingSendersAll() ([]Sender, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.account_id, s.email, s.name, s.status, s.first_seen,
		       m.id, m.subject, m.preview
		FROM senders s
		JOIN mails m ON m.account_id = s.account_id AND m.sender_email = s.email AND m.folder = 'pfoertner'
		WHERE s.status = 'pending'
		GROUP BY s.id
		ORDER BY s.first_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Sender
	for rows.Next() {
		var sd Sender
		var firstSeen string
		if err := rows.Scan(&sd.ID, &sd.AccountID, &sd.Email, &sd.Name, &sd.Status, &firstSeen,
			&sd.MailID, &sd.Subject, &sd.Preview); err != nil {
			return nil, err
		}
		out = append(out, sd)
	}
	return out, rows.Err()
}

type Sender struct {
	ID        int64
	AccountID int64
	Email     string
	Name      string
	Status    string
	MailID    int64
	Subject   string
	Preview   string
}

// ---------- Mails ----------

type Folder string

const (
	FolderPost      Folder = "post"
	FolderAblage    Folder = "ablage"
	FolderPfoertner Folder = "pfoertner"
	FolderGesendet  Folder = "gesendet"
)

type Mail struct {
	ID            int64
	AccountID     int64
	UID           uint32
	Mailbox       string
	Folder        Folder
	SenderName    string
	SenderEmail   string
	Recipients    string // kommagetrennt, nur bei Folder == FolderGesendet befüllt
	Subject       string
	Preview       string
	BodyText      string
	BodyHTML      string
	ReceivedAt    time.Time
	Unread        bool
	HasAttachment bool
}

// InsertMail legt eine Mail an und liefert ihre ID zurück. Kollidiert
// (account_id, mailbox, uid) mit einem bereits vorhandenen Datensatz
// (derselbe Sync-Lauf lief doppelt), wird der INSERT stillschweigend
// ignoriert statt zu duplizieren — die ID wird trotzdem zuverlässig per
// SELECT nachgeschlagen, da sich bei ON CONFLICT DO NOTHING nicht auf
// LastInsertId() verlassen lässt (der bliebe bei einem No-Op auf dem
// Stand des vorherigen erfolgreichen INSERTs stehen).
func (s *Store) InsertMail(accountID int64, mailbox string, m Mail) (int64, error) {
	unread, attachment := 0, 0
	if m.Unread {
		unread = 1
	}
	if m.HasAttachment {
		attachment = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO mails
			(account_id, uid, mailbox, folder, sender_name, sender_email, recipients,
			 subject, preview, body_text, body_html, received_at, unread, has_attachment)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id, mailbox, uid) DO NOTHING`,
		accountID, m.UID, mailbox, string(m.Folder), m.SenderName, m.SenderEmail, m.Recipients,
		m.Subject, m.Preview, m.BodyText, m.BodyHTML, m.ReceivedAt.UTC().Format(time.RFC3339), unread, attachment)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(`SELECT id FROM mails WHERE account_id = ? AND mailbox = ? AND uid = ?`,
		accountID, mailbox, m.UID).Scan(&id)
	return id, err
}

// HasRemoteMail prüft, ob eine IMAP-Nachricht bereits lokal vorhanden ist.
// Das verhindert insbesondere doppelte Anhänge, wenn eine direkt nach dem
// Versand gespeicherte Serverkopie beim nächsten Sync erneut auftaucht.
func (s *Store) HasRemoteMail(accountID int64, mailbox string, uid uint32) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM mails WHERE account_id = ? AND mailbox = ? AND uid = ?)`,
		accountID, mailbox, uid).Scan(&exists)
	return exists == 1, err
}

// InsertSentMail speichert eine lokal versendete Mail im Ordner
// FolderGesendet. Anders als InsertMail (Sync von echten IMAP-UIDs, mit
// ON CONFLICT DO NOTHING zur Deduplizierung eines doppelt gelaufenen
// Sync-Vorgangs) gibt es hier keine server-seitige UID und keine
// Duplikat-Gefahr — jede Mail-Komposition ist per Definition ein neues,
// einmaliges Ereignis. Die uid-Spalte muss trotzdem befüllt werden
// (UNIQUE(account_id, mailbox, uid) verlangt einen Wert): die global
// eindeutige mails.id wird nach dem Insert einfach als uid übernommen,
// das erfüllt die Eindeutigkeit trivial, ohne einen separaten Zähler zu
// brauchen.
func (s *Store) InsertSentMail(accountID int64, m Mail) (int64, error) {
	attachment := 0
	if m.HasAttachment {
		attachment = 1
	}
	res, err := s.db.Exec(`
		INSERT INTO mails
			(account_id, uid, mailbox, folder, sender_name, sender_email, recipients,
			 subject, preview, body_text, body_html, received_at, unread, has_attachment)
		VALUES (?, 0, 'LOCAL_SENT', ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		accountID, string(FolderGesendet), m.SenderName, m.SenderEmail, m.Recipients,
		m.Subject, m.Preview, m.BodyText, m.BodyHTML, m.ReceivedAt.UTC().Format(time.RFC3339), attachment)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`UPDATE mails SET uid = ? WHERE id = ?`, id, id); err != nil {
		return 0, err
	}
	return id, nil
}

// GetMail liefert eine einzelne Mail über ihre ID — eindeutig über alle
// Konten hinweg, da mails.id ein globaler AUTOINCREMENT-Schlüssel ist.
func (s *Store) GetMail(mailID int64) (Mail, error) {
	var m Mail
	var received string
	var unread, attachment int
	err := s.db.QueryRow(`
		SELECT id, account_id, uid, mailbox, folder, sender_name, sender_email, recipients, subject, preview,
		       body_text, body_html, received_at, unread, has_attachment
		FROM mails WHERE id = ?`, mailID).
		Scan(&m.ID, &m.AccountID, &m.UID, &m.Mailbox, &m.Folder, &m.SenderName, &m.SenderEmail, &m.Recipients,
			&m.Subject, &m.Preview, &m.BodyText, &m.BodyHTML, &received, &unread, &attachment)
	if err != nil {
		return Mail{}, err
	}
	m.ReceivedAt, _ = time.Parse(time.RFC3339, received)
	m.Unread = unread == 1
	m.HasAttachment = attachment == 1
	return m, nil
}

func (s *Store) MarkRead(mailID int64) error {
	_, err := s.db.Exec(`UPDATE mails SET unread = 0 WHERE id = ?`, mailID)
	return err
}

// DeleteMail entfernt eine Mail lokal — analog zur Sperrung eines
// Absenders (SetSenderStatus) betrifft das nur die lokale Ablage, nicht
// das Postfach auf dem Server.
func (s *Store) DeleteMail(mailID int64) error {
	_, err := s.db.Exec(`DELETE FROM mails WHERE id = ?`, mailID)
	return err
}

// MoveMail versetzt eine Mail in einen anderen Ordner (Post <-> Ablage).
func (s *Store) MoveMail(mailID int64, folder Folder) error {
	_, err := s.db.Exec(`UPDATE mails SET folder = ? WHERE id = ?`, string(folder), mailID)
	return err
}

// ListAllMails liefert einen Ordner über alle Konten hinweg (gemeinsames
// Postfach), sortiert nach Empfangsdatum.
// ListAllMails wählt bewusst body_html NICHT mit aus — die Listenansicht
// zeigt nur Preview/Subject, aber HTML-Mails mit eingebetteten
// data:-URI-Bildern können pro Mail mehrere hundert KB groß sein. Das für
// jede Zeile einer möglicherweise langen Liste mitzuladen würde Post/
// Ablage unnötig verlangsamen. GetMail (Detailansicht, eine einzelne
// Mail) lädt body_html weiterhin vollständig.
func (s *Store) ListAllMails(folder Folder) ([]Mail, error) {
	rows, err := s.db.Query(`
		SELECT id, account_id, uid, folder, sender_name, sender_email, recipients, subject, preview,
		       body_text, received_at, unread, has_attachment
		FROM mails
		WHERE folder = ?
		ORDER BY received_at DESC`, string(folder))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Mail
	for rows.Next() {
		var m Mail
		var received string
		var unread, attachment int
		if err := rows.Scan(&m.ID, &m.AccountID, &m.UID, &m.Folder, &m.SenderName, &m.SenderEmail, &m.Recipients,
			&m.Subject, &m.Preview, &m.BodyText, &received, &unread, &attachment); err != nil {
			return nil, err
		}
		m.ReceivedAt, _ = time.Parse(time.RFC3339, received)
		m.Unread = unread == 1
		m.HasAttachment = attachment == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------- Anhänge ----------

type Attachment struct {
	ID          int64
	MailID      int64
	Filename    string
	ContentType string
	Size        int64
	Data        []byte // nur von GetAttachment befüllt, ListAttachments lässt es leer (siehe dort)
}

func (s *Store) InsertAttachment(mailID int64, a Attachment) error {
	_, err := s.db.Exec(`
		INSERT INTO attachments (mail_id, filename, content_type, size, data)
		VALUES (?, ?, ?, ?, ?)`,
		mailID, a.Filename, a.ContentType, a.Size, a.Data)
	return err
}

// ListAttachments liefert nur Metadaten (kein data-BLOB) — für die
// Anhang-Liste in der Detailansicht reicht das, und die eigentlichen
// Bytes potenziell großer Anhänge sollen nicht bei jedem Mail-Öffnen
// mitgeladen werden. Für den Download-Fall siehe GetAttachment.
func (s *Store) ListAttachments(mailID int64) ([]Attachment, error) {
	rows, err := s.db.Query(`
		SELECT id, mail_id, filename, content_type, size
		FROM attachments WHERE mail_id = ? ORDER BY id`, mailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.MailID, &a.Filename, &a.ContentType, &a.Size); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAttachment lädt einen einzelnen Anhang inklusive Inhalt — für den
// tatsächlichen Download (siehe mailservice.go SaveAttachment).
func (s *Store) GetAttachment(id int64) (Attachment, error) {
	var a Attachment
	err := s.db.QueryRow(`
		SELECT id, mail_id, filename, content_type, size, data
		FROM attachments WHERE id = ?`, id).
		Scan(&a.ID, &a.MailID, &a.Filename, &a.ContentType, &a.Size, &a.Data)
	return a, err
}
