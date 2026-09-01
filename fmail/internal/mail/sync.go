// sync.go verbindet IMAP-Fetch, Klassifizierung (Post/Ablage/Pförtner)
// und Persistenz. Das ist die einzige Stelle, die "weiß", was mit einer
// frisch abgerufenen Mail passiert — Absenderstatus prüfen, ggf. als
// neuen Pförtner-Fall anlegen, sonst in Post oder Ablage einsortieren.
package mail

import (
	"fmt"
	"strings"

	"fmail/internal/store"
)

const previewLength = 140

// Syncer bündelt Store-Zugriff für den Sync-Lauf eines Kontos.
type Syncer struct {
	Store *store.Store
}

// SyncSentAccount importiert neue Nachrichten aus dem serverseitigen
// Gesendet-Ordner. Anders als INBOX durchlaufen eigene Mails weder Pförtner
// noch Ablage-Klassifizierung.
func (s *Syncer) SyncSentAccount(accountID int64, cfg IMAPConfig) (int, error) {
	mailbox, err := FindSentMailbox(cfg)
	if err != nil {
		return 0, fmt.Errorf("sync gesendet: %w", err)
	}
	lastUID, err := s.Store.GetLastUID(accountID, mailbox)
	if err != nil {
		return 0, fmt.Errorf("sync gesendet: letzte UID konnte nicht gelesen werden: %w", err)
	}
	messages, newLastUID, err := FetchNew(cfg, mailbox, lastUID)
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, m := range messages {
		exists, err := s.Store.HasRemoteMail(accountID, mailbox, m.UID)
		if err != nil {
			return imported, fmt.Errorf("sync gesendet: Duplikatprüfung fehlgeschlagen: %w", err)
		}
		if exists {
			continue
		}
		mail := store.Mail{
			UID: m.UID, Folder: store.FolderGesendet,
			SenderName: m.SenderName, SenderEmail: m.SenderEmail,
			Recipients: strings.Join(m.Recipients, ", "), Subject: m.Subject,
			Preview: Preview(m.PlainText, previewLength), BodyText: m.PlainText,
			BodyHTML: m.HTMLBody, ReceivedAt: m.Date,
			HasAttachment: m.HasAttachment,
		}
		mailID, err := s.Store.InsertMail(accountID, mailbox, mail)
		if err != nil {
			return imported, fmt.Errorf("sync gesendet: Mail konnte nicht gespeichert werden: %w", err)
		}
		for _, a := range m.Attachments {
			if err := s.Store.InsertAttachment(mailID, store.Attachment{
				Filename: a.Filename, ContentType: a.ContentType, Size: int64(len(a.Data)), Data: a.Data,
			}); err != nil {
				return imported, fmt.Errorf("sync gesendet: Anhang konnte nicht gespeichert werden: %w", err)
			}
		}
		imported++
	}
	if newLastUID > lastUID {
		if err := s.Store.SetLastUID(accountID, mailbox, newLastUID); err != nil {
			return imported, fmt.Errorf("sync gesendet: Sync-Status konnte nicht aktualisiert werden: %w", err)
		}
	}
	return imported, nil
}

// SyncAccount holt neue Mails für ein Konto ab und sortiert sie ein.
// Gesperrte Absender werden verworfen, bevor irgendetwas in die
// Datenbank geschrieben wird — sie sollen so wirken, als kämen sie nie an.
func (s *Syncer) SyncAccount(accountID int64, cfg IMAPConfig, mailbox string) (int, error) {
	lastUID, err := s.Store.GetLastUID(accountID, mailbox)
	if err != nil {
		return 0, fmt.Errorf("sync: letzte UID konnte nicht gelesen werden: %w", err)
	}

	messages, newLastUID, err := FetchNew(cfg, mailbox, lastUID)
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, m := range messages {
		exists, err := s.Store.HasRemoteMail(accountID, mailbox, m.UID)
		if err != nil {
			return imported, fmt.Errorf("sync: Duplikatprüfung fehlgeschlagen: %w", err)
		}
		if exists {
			continue
		}
		status, known, err := s.Store.SenderStatus(accountID, m.SenderEmail)
		if err != nil {
			return imported, fmt.Errorf("sync: Absenderstatus konnte nicht gelesen werden: %w", err)
		}

		if known && status == store.StatusBlocked {
			continue // wurde gesperrt — nie geschehen
		}

		folder := store.FolderPost
		switch {
		case !known:
			folder = store.FolderPfoertner
			if err := s.Store.InsertPendingSender(accountID, m.SenderEmail, m.SenderName); err != nil {
				return imported, fmt.Errorf("sync: Absender konnte nicht angelegt werden: %w", err)
			}
		case IsAblage(m.SenderEmail, m.Subject):
			folder = store.FolderAblage
		}

		mail := store.Mail{
			UID:           m.UID,
			Folder:        folder,
			SenderName:    m.SenderName,
			SenderEmail:   m.SenderEmail,
			Subject:       m.Subject,
			Preview:       Preview(m.PlainText, previewLength),
			BodyText:      m.PlainText,
			BodyHTML:      m.HTMLBody,
			ReceivedAt:    m.Date,
			Unread:        true,
			HasAttachment: m.HasAttachment,
		}
		mailID, err := s.Store.InsertMail(accountID, mailbox, mail)
		if err != nil {
			return imported, fmt.Errorf("sync: Mail konnte nicht gespeichert werden: %w", err)
		}
		for _, a := range m.Attachments {
			if err := s.Store.InsertAttachment(mailID, store.Attachment{
				Filename: a.Filename, ContentType: a.ContentType, Size: int64(len(a.Data)), Data: a.Data,
			}); err != nil {
				return imported, fmt.Errorf("sync: Anhang konnte nicht gespeichert werden: %w", err)
			}
		}
		imported++
	}

	if newLastUID > lastUID {
		if err := s.Store.SetLastUID(accountID, mailbox, newLastUID); err != nil {
			return imported, fmt.Errorf("sync: Sync-Status konnte nicht aktualisiert werden: %w", err)
		}
	}

	return imported, nil
}
