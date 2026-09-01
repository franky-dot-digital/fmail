package mail

import (
	"net"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// TestPickSentMailbox deckt die reine Auswahllogik ab, ohne einen Server zu
// brauchen: Vorrang des \Sent-Attributs vor Namensraten, case-insensitiver
// Namensabgleich, und der Fehlerfall, wenn nichts passt.
func TestPickSentMailbox(t *testing.T) {
	t.Run("SPECIAL-USE-Attribut hat Vorrang", func(t *testing.T) {
		entries := []*imap.ListData{
			{Mailbox: "INBOX"},
			{Mailbox: "Ausgang", Attrs: []imap.MailboxAttr{imap.MailboxAttrSent}},
			{Mailbox: "Sent"}, // wäre der Namens-Fallback -- darf hier NICHT gewinnen
		}
		got, err := pickSentMailbox(entries)
		if err != nil {
			t.Fatalf("pickSentMailbox: %v", err)
		}
		if got != "Ausgang" {
			t.Errorf("got %q, want %q (SPECIAL-USE sollte vor Namensraten gewinnen)", got, "Ausgang")
		}
	})

	t.Run("Namensraten case-insensitiv, wenn kein SPECIAL-USE-Attribut vorliegt", func(t *testing.T) {
		entries := []*imap.ListData{
			{Mailbox: "INBOX"},
			{Mailbox: "SENT"}, // Server benennt in Großbuchstaben
			{Mailbox: "Trash"},
		}
		got, err := pickSentMailbox(entries)
		if err != nil {
			t.Fatalf("pickSentMailbox: %v", err)
		}
		if got != "SENT" {
			t.Errorf("got %q, want %q", got, "SENT")
		}
	})

	t.Run("kein Treffer liefert Fehler", func(t *testing.T) {
		entries := []*imap.ListData{{Mailbox: "INBOX"}, {Mailbox: "Archiv"}}
		if _, err := pickSentMailbox(entries); err == nil {
			t.Error("erwarteter Fehler blieb aus")
		}
	})
}

func TestPickTrashMailbox(t *testing.T) {
	t.Run("special-use hat Vorrang", func(t *testing.T) {
		entries := []*imap.ListData{
			{Mailbox: "Trash"},
			{Mailbox: "Gelöschte Objekte", Attrs: []imap.MailboxAttr{imap.MailboxAttrTrash}},
		}
		got, err := pickTrashMailbox(entries)
		if err != nil || got != "Gelöschte Objekte" {
			t.Fatalf("pickTrashMailbox = %q, %v", got, err)
		}
	})
	t.Run("deutscher Namensfallback", func(t *testing.T) {
		got, err := pickTrashMailbox([]*imap.ListData{{Mailbox: "INBOX"}, {Mailbox: "Papierkorb"}})
		if err != nil || got != "Papierkorb" {
			t.Fatalf("pickTrashMailbox = %q, %v", got, err)
		}
	})
}

// newTestIMAPServer startet einen imapmemserver auf einem lokalen
// TCP-Port und liefert einen bereits per Login authentifizierten
// *imapclient.Client zurück. InsecureAuth ist hier bewusst nur für den
// Test aktiv (imapclient.New umgeht dial() ohnehin komplett, kein
// Produktionscodepfad läuft unverschlüsselt) — siehe appendSentToConn.
func newTestIMAPServer(t *testing.T, user *imapmemserver.User) *imapclient.Client {
	t.Helper()
	memSrv := imapmemserver.New()
	memSrv.AddUser(user)

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memSrv.NewSession(), nil, nil
		},
		InsecureAuth: true,
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	c := imapclient.New(conn, nil)
	t.Cleanup(func() { c.Close() })

	if err := c.Login("frank", "geheim").Wait(); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return c
}

// TestAppendSentToConn_FallbackName prüft den vollen Weg gegen einen
// echten (simulierten) IMAP-Server: ein Ordner namens "Sent" ohne
// SPECIAL-USE-Markierung (die dieser Mock-Server für CREATE ohnehin nicht
// abbildet) muss über den Namens-Fallback gefunden werden, die Nachricht
// muss unverändert und mit \Seen ankommen.
func TestAppendSentToConn_FallbackName(t *testing.T) {
	user := imapmemserver.NewUser("frank", "geheim")
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatalf("INBOX anlegen: %v", err)
	}
	if err := user.Create("Sent", nil); err != nil {
		t.Fatalf("Sent anlegen: %v", err)
	}

	c := newTestIMAPServer(t, user)

	raw := []byte("From: frank@example.com\r\nTo: bob@example.com\r\nSubject: Hallo\r\n\r\nHallo Bob!")
	sentAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	if err := appendSentToConn(c, raw, sentAt); err != nil {
		t.Fatalf("appendSentToConn: %v", err)
	}

	selectData, err := c.Select("Sent", nil).Wait()
	if err != nil {
		t.Fatalf("Select(Sent): %v", err)
	}
	if selectData.NumMessages != 1 {
		t.Fatalf("NumMessages = %d, want 1", selectData.NumMessages)
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(1, 1)
	fetchCmd := c.Fetch(seqSet, &imap.FetchOptions{
		Flags:       true,
		BodySection: []*imap.FetchItemBodySection{{}},
	})
	msg := fetchCmd.Next()
	if msg == nil {
		t.Fatal("keine Nachricht im Sent-Ordner gefunden")
	}
	buf, err := msg.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if err := fetchCmd.Close(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	body := buf.FindBodySection(&imap.FetchItemBodySection{})
	if string(body) != string(raw) {
		t.Errorf("Nachricht im Sent-Ordner weicht vom Original ab:\ngot:  %q\nwant: %q", body, raw)
	}

	var hasSeen bool
	for _, f := range buf.Flags {
		if f == imap.FlagSeen {
			hasSeen = true
		}
	}
	if !hasSeen {
		t.Errorf("Flags = %v, erwarte \\Seen", buf.Flags)
	}
}

// TestAppendSentToConn_NoSentFolder stellt sicher, dass ein Server ohne
// jeden erkennbaren Gesendet-Ordner einen klaren Fehler liefert statt
// z. B. blind in INBOX zu schreiben.
func TestAppendSentToConn_NoSentFolder(t *testing.T) {
	user := imapmemserver.NewUser("frank", "geheim")
	if err := user.Create("INBOX", nil); err != nil {
		t.Fatalf("INBOX anlegen: %v", err)
	}

	c := newTestIMAPServer(t, user)

	err := appendSentToConn(c, []byte("Subject: x\r\n\r\nx"), time.Now())
	if err == nil {
		t.Fatal("erwarteter Fehler blieb aus")
	}
}

func TestMoveToTrashOnConn(t *testing.T) {
	user := imapmemserver.NewUser("frank", "geheim")
	for _, mailbox := range []string{"INBOX", "Trash"} {
		if err := user.Create(mailbox, nil); err != nil {
			t.Fatalf("%s anlegen: %v", mailbox, err)
		}
	}
	c := newTestIMAPServer(t, user)
	raw := []byte("From: alice@example.com\r\nSubject: Löschen\r\n\r\nText")
	cmd := c.Append("INBOX", int64(len(raw)), nil)
	if _, err := cmd.Write(raw); err != nil {
		t.Fatalf("Append Write: %v", err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatalf("Append Close: %v", err)
	}
	if _, err := cmd.Wait(); err != nil {
		t.Fatalf("Append Wait: %v", err)
	}
	if err := moveToTrashOnConn(c, "INBOX", 1); err != nil {
		t.Fatalf("moveToTrashOnConn: %v", err)
	}
	for mailbox, want := range map[string]uint32{"INBOX": 0, "Trash": 1} {
		selected, err := c.Select(mailbox, nil).Wait()
		if err != nil || selected.NumMessages != want {
			t.Fatalf("%s: NumMessages=%d err=%v, want %d", mailbox, selected.NumMessages, err, want)
		}
	}
}
