// Package secrets speichert Zugangsdaten ausschließlich im systemeigenen
// Schlüsselbund (macOS Keychain, Windows Credential Manager, Linux
// Secret Service / GNOME Keyring). Es landet nie ein Passwort in der
// SQLite-Datenbank oder in einer Konfigurationsdatei auf der Platte.
package secrets

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "fmail"

// Kind unterscheidet IMAP- und SMTP-Zugangsdaten für dasselbe Konto,
// falls beide unterschiedliche Passwörter verwenden (z. B. App-Passwörter).
type Kind string

const (
	KindIMAP Kind = "imap"
	KindSMTP Kind = "smtp"
)

func key(accountID int64, kind Kind) string {
	return fmt.Sprintf("account-%d-%s", accountID, kind)
}

// Set speichert (oder überschreibt) das Passwort für ein Konto im Schlüsselbund.
func Set(accountID int64, kind Kind, password string) error {
	if password == "" {
		return fmt.Errorf("secrets: leeres Passwort wird nicht gespeichert")
	}
	return keyring.Set(service, key(accountID, kind), password)
}

// Get liest das Passwort aus dem Schlüsselbund. Der Aufrufer darf den
// Rückgabewert nicht loggen oder in Fehlermeldungen einbetten.
func Get(accountID int64, kind Kind) (string, error) {
	pw, err := keyring.Get(service, key(accountID, kind))
	if err != nil {
		return "", fmt.Errorf("secrets: kein Passwort im Schlüsselbund hinterlegt: %w", err)
	}
	return pw, nil
}

// Delete entfernt ein gespeichertes Passwort, z. B. beim Löschen eines Kontos.
func Delete(accountID int64, kind Kind) error {
	err := keyring.Delete(service, key(accountID, kind))
	if err != nil && err != keyring.ErrNotFound {
		return err
	}
	return nil
}
