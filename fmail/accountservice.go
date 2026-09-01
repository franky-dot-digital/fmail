package main

import (
	"fmt"
	"strings"

	"fmail/internal/mail"
	"fmail/internal/secrets"
	"fmail/internal/store"
)

// AccountInput kommt 1:1 aus dem Kontoformular im Frontend.
type AccountInput struct {
	Label               string `json:"label"`
	Email               string `json:"email"`
	DisplayName         string `json:"displayName"` // Absendername ("Von"), z. B. "Frank Lewandowski"
	IMAPHost            string `json:"imapHost"`
	IMAPPort            int    `json:"imapPort"`
	IMAPSecurity        string `json:"imapSecurity"` // "tls" | "starttls"
	IMAPUsername        string `json:"imapUsername"`
	IMAPPassword        string `json:"imapPassword"`
	SMTPHost            string `json:"smtpHost"`
	SMTPPort            int    `json:"smtpPort"`
	SMTPSecurity        string `json:"smtpSecurity"`
	SMTPUsername        string `json:"smtpUsername"`
	SMTPPassword        string `json:"smtpPassword"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"` // 0 = nur manueller Sync
}

type AccountView struct {
	ID                  int64  `json:"id"`
	Label               string `json:"label"`
	Email               string `json:"email"`
	DisplayName         string `json:"displayName"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"`
}

// AccountEditView liefert die Verbindungsdaten eines Kontos zurück, um
// das Bearbeiten-Formular vorzubefüllen — bewusst ohne Passwörter, die
// bleiben ausschließlich im Schlüsselbund.
type AccountEditView struct {
	ID                  int64  `json:"id"`
	Label               string `json:"label"`
	Email               string `json:"email"`
	DisplayName         string `json:"displayName"`
	IMAPHost            string `json:"imapHost"`
	IMAPPort            int    `json:"imapPort"`
	IMAPSecurity        string `json:"imapSecurity"`
	IMAPUsername        string `json:"imapUsername"`
	SMTPHost            string `json:"smtpHost"`
	SMTPPort            int    `json:"smtpPort"`
	SMTPSecurity        string `json:"smtpSecurity"`
	SMTPUsername        string `json:"smtpUsername"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"`
}

type AccountService struct {
	store *store.Store
}

func NewAccountService(st *store.Store) *AccountService {
	return &AccountService{store: st}
}

func (a *AccountService) ListAccounts() ([]AccountView, error) {
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	out := make([]AccountView, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, AccountView{
			ID: acc.ID, Label: acc.Label, Email: acc.Email, DisplayName: acc.DisplayName,
			SyncIntervalMinutes: acc.SyncIntervalMinutes,
		})
	}
	return out, nil
}

// TestIMAP prüft Host/Port/Zugangsdaten, ohne etwas zu speichern —
// für den "Verbindung testen"-Button im Formular.
func (a *AccountService) TestIMAP(in AccountInput) error {
	if err := validateSecurity(in.IMAPSecurity); err != nil {
		return err
	}
	return mail.TestIMAPConnection(mail.IMAPConfig{
		Host: in.IMAPHost, Port: in.IMAPPort, Security: in.IMAPSecurity,
		Username: coalesce(in.IMAPUsername, in.Email), Password: in.IMAPPassword,
	})
}

func (a *AccountService) TestSMTP(in AccountInput) error {
	if err := validateSecurity(in.SMTPSecurity); err != nil {
		return err
	}
	pw := in.SMTPPassword
	if pw == "" {
		pw = in.IMAPPassword
	}
	return mail.TestSMTPConnection(mail.SMTPConfig{
		Host: in.SMTPHost, Port: in.SMTPPort, Security: in.SMTPSecurity,
		Username: coalesce(in.SMTPUsername, in.IMAPUsername, in.Email), Password: pw,
	})
}

// CreateAccount legt das Konto in der DB an (nur Verbindungsdaten, kein
// Passwort) und speichert die Passwörter getrennt im OS-Schlüsselbund.
func (a *AccountService) CreateAccount(in AccountInput) (AccountView, error) {
	in.Label = strings.TrimSpace(in.Label)
	in.Email = strings.TrimSpace(in.Email)
	if in.Label == "" || in.Email == "" || in.IMAPHost == "" || in.SMTPHost == "" {
		return AccountView{}, fmt.Errorf("bitte Label, E-Mail sowie IMAP- und SMTP-Host ausfüllen")
	}
	if err := validateSecurity(in.IMAPSecurity); err != nil {
		return AccountView{}, err
	}
	if err := validateSecurity(in.SMTPSecurity); err != nil {
		return AccountView{}, err
	}

	imapUsername := coalesce(in.IMAPUsername, in.Email)
	smtpUsername := coalesce(in.SMTPUsername, imapUsername)

	id, err := a.store.CreateAccount(store.Account{
		Label:               in.Label,
		Email:               in.Email,
		DisplayName:         strings.TrimSpace(in.DisplayName),
		IMAPHost:            in.IMAPHost,
		IMAPPort:            in.IMAPPort,
		IMAPSecurity:        in.IMAPSecurity,
		IMAPUsername:        imapUsername,
		SMTPHost:            in.SMTPHost,
		SMTPPort:            in.SMTPPort,
		SMTPSecurity:        in.SMTPSecurity,
		SMTPUsername:        smtpUsername,
		SyncIntervalMinutes: max(0, in.SyncIntervalMinutes),
	})
	if err != nil {
		return AccountView{}, fmt.Errorf("Konto konnte nicht angelegt werden: %w", err)
	}

	if err := secrets.Set(id, secrets.KindIMAP, in.IMAPPassword); err != nil {
		_ = a.store.DeleteAccount(id)
		return AccountView{}, fmt.Errorf("IMAP-Passwort konnte nicht im Schlüsselbund gespeichert werden: %w", err)
	}
	smtpPw := in.SMTPPassword
	if smtpPw == "" {
		smtpPw = in.IMAPPassword
	}
	if err := secrets.Set(id, secrets.KindSMTP, smtpPw); err != nil {
		_ = secrets.Delete(id, secrets.KindIMAP)
		_ = a.store.DeleteAccount(id)
		return AccountView{}, fmt.Errorf("SMTP-Passwort konnte nicht im Schlüsselbund gespeichert werden: %w", err)
	}

	return AccountView{
		ID: id, Label: in.Label, Email: in.Email, DisplayName: in.DisplayName,
		SyncIntervalMinutes: max(0, in.SyncIntervalMinutes),
	}, nil
}

// GetAccountDetails liefert die Verbindungsdaten eines Kontos für das
// Bearbeiten-Formular — ohne Passwörter, siehe AccountEditView.
func (a *AccountService) GetAccountDetails(id int64) (AccountEditView, error) {
	acc, err := a.store.GetAccount(id)
	if err != nil {
		return AccountEditView{}, fmt.Errorf("Konto nicht gefunden: %w", err)
	}
	return AccountEditView{
		ID: acc.ID, Label: acc.Label, Email: acc.Email, DisplayName: acc.DisplayName,
		IMAPHost: acc.IMAPHost, IMAPPort: acc.IMAPPort, IMAPSecurity: acc.IMAPSecurity, IMAPUsername: acc.IMAPUsername,
		SMTPHost: acc.SMTPHost, SMTPPort: acc.SMTPPort, SMTPSecurity: acc.SMTPSecurity, SMTPUsername: acc.SMTPUsername,
		SyncIntervalMinutes: acc.SyncIntervalMinutes,
	}, nil
}

// UpdateAccount ändert Label, Anzeigename und Verbindungsdaten eines
// bestehenden Kontos. Die Passwortfelder in AccountInput sind hier
// optional — leer gelassen, bleibt das im Schlüsselbund gespeicherte
// Passwort unangetastet, statt es zu löschen oder mit "" zu überschreiben.
func (a *AccountService) UpdateAccount(id int64, in AccountInput) (AccountView, error) {
	in.Label = strings.TrimSpace(in.Label)
	in.Email = strings.TrimSpace(in.Email)
	if in.Label == "" || in.Email == "" || in.IMAPHost == "" || in.SMTPHost == "" {
		return AccountView{}, fmt.Errorf("bitte Label, E-Mail sowie IMAP- und SMTP-Host ausfüllen")
	}
	if err := validateSecurity(in.IMAPSecurity); err != nil {
		return AccountView{}, err
	}
	if err := validateSecurity(in.SMTPSecurity); err != nil {
		return AccountView{}, err
	}

	imapUsername := coalesce(in.IMAPUsername, in.Email)
	smtpUsername := coalesce(in.SMTPUsername, imapUsername)

	if err := a.store.UpdateAccount(id, store.Account{
		Label:               in.Label,
		Email:               in.Email,
		DisplayName:         strings.TrimSpace(in.DisplayName),
		IMAPHost:            in.IMAPHost,
		IMAPPort:            in.IMAPPort,
		IMAPSecurity:        in.IMAPSecurity,
		IMAPUsername:        imapUsername,
		SMTPHost:            in.SMTPHost,
		SMTPPort:            in.SMTPPort,
		SMTPSecurity:        in.SMTPSecurity,
		SMTPUsername:        smtpUsername,
		SyncIntervalMinutes: max(0, in.SyncIntervalMinutes),
	}); err != nil {
		return AccountView{}, fmt.Errorf("Konto konnte nicht aktualisiert werden: %w", err)
	}

	if in.IMAPPassword != "" {
		if err := secrets.Set(id, secrets.KindIMAP, in.IMAPPassword); err != nil {
			return AccountView{}, fmt.Errorf("IMAP-Passwort konnte nicht gespeichert werden: %w", err)
		}
	}
	switch {
	case in.SMTPPassword != "":
		if err := secrets.Set(id, secrets.KindSMTP, in.SMTPPassword); err != nil {
			return AccountView{}, fmt.Errorf("SMTP-Passwort konnte nicht gespeichert werden: %w", err)
		}
	case in.IMAPPassword != "":
		// Formularkonvention wie bei CreateAccount: SMTP-Passwort leer ->
		// spiegelt das (neue) IMAP-Passwort.
		if err := secrets.Set(id, secrets.KindSMTP, in.IMAPPassword); err != nil {
			return AccountView{}, fmt.Errorf("SMTP-Passwort konnte nicht gespeichert werden: %w", err)
		}
	}

	acc, err := a.store.GetAccount(id)
	if err != nil {
		return AccountView{}, err
	}
	return AccountView{
		ID: acc.ID, Label: acc.Label, Email: acc.Email, DisplayName: acc.DisplayName,
		SyncIntervalMinutes: acc.SyncIntervalMinutes,
	}, nil
}

// DeleteAccount entfernt ein Konto samt aller lokal gespeicherten Mails
// (kaskadiert in der DB) und den zugehörigen Passwörtern im Schlüsselbund.
func (a *AccountService) DeleteAccount(id int64) error {
	if err := secrets.Delete(id, secrets.KindIMAP); err != nil {
		return fmt.Errorf("IMAP-Passwort konnte nicht aus dem Schlüsselbund entfernt werden: %w", err)
	}
	if err := secrets.Delete(id, secrets.KindSMTP); err != nil {
		return fmt.Errorf("SMTP-Passwort konnte nicht aus dem Schlüsselbund entfernt werden: %w", err)
	}
	if err := a.store.DeleteAccount(id); err != nil {
		return fmt.Errorf("Konto konnte nicht gelöscht werden: %w", err)
	}
	return nil
}

// SyncNow ruft neue Mails ab und sortiert sie in Post/Ablage/Pförtner ein.
func (a *AccountService) SyncNow(accountID int64) (int, error) {
	acc, err := a.store.GetAccount(accountID)
	if err != nil {
		return 0, fmt.Errorf("Konto nicht gefunden: %w", err)
	}
	pw, err := secrets.Get(accountID, secrets.KindIMAP)
	if err != nil {
		return 0, err
	}

	syncer := &mail.Syncer{Store: a.store}
	cfg := mail.IMAPConfig{
		Host: acc.IMAPHost, Port: acc.IMAPPort, Security: acc.IMAPSecurity,
		Username: acc.IMAPUsername, Password: pw,
	}
	inboxCount, err := syncer.SyncAccount(accountID, cfg, "INBOX")
	if err != nil {
		return inboxCount, err
	}
	sentCount, err := syncer.SyncSentAccount(accountID, cfg)
	return inboxCount + sentCount, err
}

// SyncAll synchronisiert alle Konten nacheinander (Post/Ablage/Pförtner
// sind ein gemeinsames Postfach über alle Konten hinweg). Ein
// fehlschlagendes Konto (z. B. falsches Passwort) bricht die anderen
// nicht ab — Fehler werden gesammelt und am Ende gemeinsam gemeldet.
func (a *AccountService) SyncAll() (int, error) {
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return 0, err
	}

	total := 0
	var failed []string
	for _, acc := range accounts {
		n, err := a.SyncNow(acc.ID)
		total += n
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", acc.Label, err))
		}
	}

	if len(failed) > 0 {
		return total, fmt.Errorf("%d von %d Konten fehlgeschlagen: %s", len(failed), len(accounts), strings.Join(failed, "; "))
	}
	return total, nil
}

func validateSecurity(s string) error {
	if s != "tls" && s != "starttls" {
		return fmt.Errorf("Sicherheitsstufe muss 'tls' oder 'starttls' sein, nicht %q — unverschlüsselte Verbindungen werden nicht unterstützt", s)
	}
	return nil
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
