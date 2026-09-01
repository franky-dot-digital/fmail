package main

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"fmail/internal/mail"
	"fmail/internal/secrets"
	"fmail/internal/store"
)

type MailView struct {
	ID            int64  `json:"id"`
	AccountID     int64  `json:"accountId"`
	AccountLabel  string `json:"accountLabel"`
	Sender        string `json:"sender"` // bei Folder "gesendet": die Empfänger (kommagetrennt), nicht der Absender — siehe GetGesendet
	Initials      string `json:"initials"`
	AvatarColor   string `json:"avatarColor"`
	BrandIcon     string `json:"brandIcon"` // z. B. "github" — leer, wenn Absender-Domain nicht erkannt
	Subject       string `json:"subject"`
	Preview       string `json:"preview"`
	Date          string `json:"date"`
	Unread        bool   `json:"unread"`
	HasAttachment bool   `json:"hasAttachment"`
}

type MailDetailView struct {
	ID            int64            `json:"id"`
	AccountID     int64            `json:"accountId"`
	AccountLabel  string           `json:"accountLabel"`
	Sender        string           `json:"sender"`
	SenderEmail   string           `json:"senderEmail"`
	Initials      string           `json:"initials"`
	AvatarColor   string           `json:"avatarColor"`
	BrandIcon     string           `json:"brandIcon"`
	Subject       string           `json:"subject"`
	Body          string           `json:"body"`
	BodyHTML      string           `json:"bodyHtml"` // leer, wenn die Mail keine HTML-Variante hatte -> Frontend zeigt dann Body als Klartext
	Date          string           `json:"date"`
	HasAttachment bool             `json:"hasAttachment"`
	Folder        string           `json:"folder"`
	Recipients    []string         `json:"recipients"` // nur bei Folder "gesendet" befüllt
	Attachments   []AttachmentView `json:"attachments"`
}

// AttachmentView beschreibt einen Anhang ohne seinen Inhalt (siehe
// store.ListAttachments) — der Inhalt wird erst beim tatsächlichen
// Speichern geladen, siehe SaveAttachment.
type AttachmentView struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type PendingView struct {
	ID           string `json:"id"` // "<accountId>:<E-Mail>" — eindeutig über alle Konten, dient als Frontend-Schlüssel
	AccountID    int64  `json:"accountId"`
	AccountLabel string `json:"accountLabel"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Initials     string `json:"initials"`
	AvatarColor  string `json:"avatarColor"`
	BrandIcon    string `json:"brandIcon"`
	Subject      string `json:"subject"`
	Preview      string `json:"preview"`
}

type MailService struct {
	store *store.Store
}

func NewMailService(st *store.Store) *MailService {
	return &MailService{store: st}
}

// accountLabels liefert eine Konto-ID → Label-Zuordnung, damit
// Post/Ablage/Pförtner (ein gemeinsames Postfach über alle Konten hinweg)
// pro Mail anzeigen können, zu welchem Konto sie gehört.
func (m *MailService) accountLabels() (map[int64]string, error) {
	accounts, err := m.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	labels := make(map[int64]string, len(accounts))
	for _, a := range accounts {
		labels[a.ID] = a.Label
	}
	return labels, nil
}

func (m *MailService) GetPost() ([]MailView, error) {
	return m.list(store.FolderPost)
}

func (m *MailService) GetAblage() ([]MailView, error) {
	return m.list(store.FolderAblage)
}

// GetGesendet liefert lokal versendete Mails — anders als Post/Ablage kommt
// dieser Ordner nie aus einem IMAP-Sync, sondern wird direkt beim
// erfolgreichen SendMail befüllt (siehe dort). Sender/Initials/AvatarColor
// beziehen sich hier bewusst auf den (ersten) Empfänger statt auf den
// Absender — in der eigenen "Gesendet"-Ansicht ist ja immer man selbst der
// Absender, interessant ist, an wen die Mail ging.
func (m *MailService) GetGesendet() ([]MailView, error) {
	mails, err := m.store.ListAllMails(store.FolderGesendet)
	if err != nil {
		return nil, err
	}
	labels, err := m.accountLabels()
	if err != nil {
		return nil, err
	}
	out := make([]MailView, 0, len(mails))
	for _, mm := range mails {
		recipients := splitRecipients(mm.Recipients)
		firstRecipient := mm.SenderEmail
		if len(recipients) > 0 {
			firstRecipient = recipients[0]
		}
		out = append(out, MailView{
			ID:            mm.ID,
			AccountID:     mm.AccountID,
			AccountLabel:  labels[mm.AccountID],
			Sender:        strings.Join(recipients, ", "),
			Initials:      initials(firstRecipient),
			AvatarColor:   avatarColor(firstRecipient),
			BrandIcon:     brandIcon(firstRecipient),
			Subject:       mm.Subject,
			Preview:       mm.Preview,
			Date:          mm.ReceivedAt.Format("2. Jan"),
			Unread:        false,
			HasAttachment: mm.HasAttachment,
		})
	}
	return out, nil
}

func (m *MailService) list(folder store.Folder) ([]MailView, error) {
	mails, err := m.store.ListAllMails(folder)
	if err != nil {
		return nil, err
	}
	labels, err := m.accountLabels()
	if err != nil {
		return nil, err
	}
	out := make([]MailView, 0, len(mails))
	for _, mm := range mails {
		out = append(out, MailView{
			ID:            mm.ID,
			AccountID:     mm.AccountID,
			AccountLabel:  labels[mm.AccountID],
			Sender:        mm.SenderName,
			Initials:      initials(mm.SenderName),
			AvatarColor:   avatarColor(mm.SenderEmail),
			BrandIcon:     brandIcon(mm.SenderEmail),
			Subject:       mm.Subject,
			Preview:       mm.Preview,
			Date:          mm.ReceivedAt.Format("2. Jan"),
			Unread:        mm.Unread,
			HasAttachment: mm.HasAttachment,
		})
	}
	return out, nil
}

// splitRecipients zerlegt die kommagetrennte recipients-Spalte zurück in
// eine Liste — Gegenstück zu strings.Join(to, ", ") beim Anlegen in
// SendMail. Leere/Whitespace-Einträge werden herausgefiltert, falls to
// versehentlich Leerstrings enthielt.
func splitRecipients(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ", ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// GetMail liefert den vollen Inhalt einer Mail und markiert sie als
// gelesen — der Klartext (body_text) wird bereits beim Sync erzeugt,
// siehe internal/mail/sanitize.go.
func (m *MailService) GetMail(mailID int64) (MailDetailView, error) {
	mm, err := m.store.GetMail(mailID)
	if err != nil {
		return MailDetailView{}, err
	}
	if mm.Unread {
		if err := m.store.MarkRead(mailID); err != nil {
			return MailDetailView{}, err
		}
	}
	acc, err := m.store.GetAccount(mm.AccountID)
	if err != nil {
		return MailDetailView{}, err
	}

	var attachmentViews []AttachmentView
	if mm.HasAttachment {
		attachments, err := m.store.ListAttachments(mailID)
		if err != nil {
			return MailDetailView{}, err
		}
		attachmentViews = make([]AttachmentView, 0, len(attachments))
		for _, a := range attachments {
			attachmentViews = append(attachmentViews, AttachmentView{
				ID: a.ID, Filename: a.Filename, ContentType: a.ContentType, Size: a.Size,
			})
		}
	}

	// Bei Folder "gesendet" bezieht sich das Avatar auf den (ersten)
	// Empfänger statt auf den Absender, genau wie in GetGesendet — man ist
	// hier ja immer selbst der Absender, interessant ist, an wen es ging.
	avatarName, avatarEmail := mm.SenderName, mm.SenderEmail
	if mm.Folder == store.FolderGesendet {
		if recipients := splitRecipients(mm.Recipients); len(recipients) > 0 {
			avatarName, avatarEmail = recipients[0], recipients[0]
		}
	}

	return MailDetailView{
		ID:            mm.ID,
		AccountID:     mm.AccountID,
		AccountLabel:  acc.Label,
		Sender:        mm.SenderName,
		SenderEmail:   mm.SenderEmail,
		Initials:      initials(avatarName),
		AvatarColor:   avatarColor(avatarEmail),
		BrandIcon:     brandIcon(avatarEmail),
		Subject:       mm.Subject,
		Body:          mm.BodyText,
		BodyHTML:      mm.BodyHTML,
		Date:          mm.ReceivedAt.Format("2. Jan 2006, 15:04"),
		HasAttachment: mm.HasAttachment,
		Folder:        string(mm.Folder),
		Recipients:    splitRecipients(mm.Recipients),
		Attachments:   attachmentViews,
	}, nil
}

// SaveAttachment öffnet einen nativen "Speichern unter"-Dialog und
// schreibt den Anhang an die gewählte Stelle. Bewusst kein automatisches
// Öffnen der Datei — der Nutzer entscheidet aktiv, wo sie landet, statt
// dass f:mail sie irgendwo (z. B. temp) ablegt und einen Öffnen-Handler
// dafür aufruft. Ein leerer Rückgabewert bedeutet "abgebrochen", kein Fehler.
func (m *MailService) SaveAttachment(attachmentID int64) (string, error) {
	a, err := m.store.GetAttachment(attachmentID)
	if err != nil {
		return "", fmt.Errorf("Anhang nicht gefunden: %w", err)
	}

	path, err := application.Get().Dialog.SaveFile().
		SetFilename(a.Filename).
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("Speicherdialog fehlgeschlagen: %w", err)
	}
	if path == "" {
		return "", nil // abgebrochen
	}

	if err := os.WriteFile(path, a.Data, 0o600); err != nil {
		return "", fmt.Errorf("Anhang konnte nicht gespeichert werden: %w", err)
	}
	return path, nil
}

// OpenExternalLink öffnet einen Link aus einer Mail im Standardbrowser
// des Systems, nie innerhalb von f:mail selbst — das HTML-Rendering läuft
// in einem sandboxed iframe ohne Navigations-Erlaubnis (siehe
// frontend/src/main.js), Klicks darin werden hierher umgeleitet. Nur
// http/https/mailto zugelassen, alles andere (z. B. file:) wird abgelehnt.
func (m *MailService) OpenExternalLink(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("Link ist ungültig: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
		if u.Host == "" {
			return fmt.Errorf("Web-Link enthält keinen Host")
		}
	case "mailto":
		if u.Opaque == "" && u.Path == "" {
			return fmt.Errorf("Mail-Link enthält keine Adresse")
		}
	default:
		return fmt.Errorf("Link-Schema %q wird nicht geöffnet", u.Scheme)
	}
	return application.Get().Browser.OpenURL(rawURL)
}

// DeleteMail verschiebt eine IMAP-Mail zuerst serverseitig in den
// Papierkorb und entfernt erst danach die lokale Kopie. Rein lokale
// Gesendet-Kopien (ohne Server-UID) können direkt entfernt werden.
func (m *MailService) DeleteMail(mailID int64) error {
	mm, err := m.store.GetMail(mailID)
	if err != nil {
		return fmt.Errorf("Mail nicht gefunden: %w", err)
	}
	legacyLocalSent := mm.Mailbox == "SENT" && mm.ID >= 0 && mm.ID <= math.MaxUint32 && mm.UID == uint32(mm.ID)
	localOnly := mm.Mailbox == "LOCAL_SENT" || legacyLocalSent
	if !localOnly {
		acc, err := m.store.GetAccount(mm.AccountID)
		if err != nil {
			return fmt.Errorf("Konto nicht gefunden: %w", err)
		}
		pw, err := secrets.Get(mm.AccountID, secrets.KindIMAP)
		if err != nil {
			return err
		}
		if err := mail.MoveToTrash(mail.IMAPConfig{
			Host: acc.IMAPHost, Port: acc.IMAPPort, Security: acc.IMAPSecurity,
			Username: acc.IMAPUsername, Password: pw,
		}, mm.Mailbox, mm.UID); err != nil {
			return err
		}
	}
	return m.store.DeleteMail(mailID)
}

// MoveToAblage verschiebt eine Mail von Post nach Ablage.
func (m *MailService) MoveToAblage(mailID int64) error {
	return m.store.MoveMail(mailID, store.FolderAblage)
}

// MoveToPost verschiebt eine Mail von Ablage zurück nach Post.
func (m *MailService) MoveToPost(mailID int64) error {
	return m.store.MoveMail(mailID, store.FolderPost)
}

func (m *MailService) GetPfoertner() ([]PendingView, error) {
	senders, err := m.store.PendingSendersAll()
	if err != nil {
		return nil, err
	}
	labels, err := m.accountLabels()
	if err != nil {
		return nil, err
	}
	out := make([]PendingView, 0, len(senders))
	for _, s := range senders {
		out = append(out, PendingView{
			ID:           pendingID(s.AccountID, s.Email),
			AccountID:    s.AccountID,
			AccountLabel: labels[s.AccountID],
			Name:         s.Name,
			Email:        s.Email,
			Initials:     initials(s.Name),
			AvatarColor:  avatarColor(s.Email),
			BrandIcon:    brandIcon(s.Email),
			Subject:      s.Subject,
			Preview:      s.Preview,
		})
	}
	return out, nil
}

func pendingID(accountID int64, email string) string {
	return fmt.Sprintf("%d:%s", accountID, email)
}

// Entscheiden setzt Freigabe ("post") oder Sperrung. Gesperrte Absender
// erreichen ab dem nächsten Sync weder Post noch den Pförtner — siehe
// internal/mail/sync.go.
func (m *MailService) Entscheiden(accountID int64, email string, freigeben bool) error {
	status := store.StatusBlocked
	if freigeben {
		status = store.StatusApproved
	}
	return m.store.SetSenderStatus(accountID, email, status)
}

// AttachmentInput kommt aus dem Compose-Formular im Frontend — Data ist
// Base64-kodiert, weil FileReader im Browser Dateien so über die
// Wails-Bindings-Grenze (JSON) an Go übergibt.
type AttachmentInput struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Data        string `json:"data"`
}

// maxAttachmentsSize liegt großzügig unter üblichen SMTP-Server-Limits
// (oft 10–25 MB) — lieber hier mit einer klaren Meldung abbrechen als
// den Server das mit einer kryptischen SMTP-Fehlermeldung tun lassen.
const maxAttachmentsSize = 20 * 1024 * 1024

// SendMail verschickt eine Mail über das SMTP-Konto, optional als HTML
// (bodyHTML != "", body dient dann als Klartext-Fallback für
// multipart/alternative) und/oder mit Anhängen.
func (m *MailService) SendMail(accountID int64, to []string, subject, body, bodyHTML string, attachments []AttachmentInput) error {
	acc, err := m.store.GetAccount(accountID)
	if err != nil {
		return err
	}
	pw, err := secrets.Get(accountID, secrets.KindSMTP)
	if err != nil {
		return err
	}

	mailAttachments := make([]mail.Attachment, 0, len(attachments))
	total := 0
	for _, a := range attachments {
		data, err := base64.StdEncoding.DecodeString(a.Data)
		if err != nil {
			return fmt.Errorf("Anhang %q ist nicht gültig kodiert: %w", a.Filename, err)
		}
		total += len(data)
		mailAttachments = append(mailAttachments, mail.Attachment{
			Filename: a.Filename, ContentType: a.ContentType, Data: data,
		})
	}
	if total > maxAttachmentsSize {
		return fmt.Errorf("Anhänge sind zusammen zu groß (%.1f MB, Limit %d MB)", float64(total)/(1024*1024), maxAttachmentsSize/(1024*1024))
	}

	raw, err := mail.Send(mail.SMTPConfig{
		Host: acc.SMTPHost, Port: acc.SMTPPort, Security: acc.SMTPSecurity,
		Username: acc.SMTPUsername, Password: pw,
	}, mail.OutgoingMail{
		From: acc.Email, FromName: acc.DisplayName, To: to, Subject: subject, Body: body,
		BodyHTML: bodyHTML, Attachments: mailAttachments,
	})
	if err != nil {
		return err
	}

	// Ab hier ist die Mail unwiderruflich raus — beide folgenden Schritte
	// (Server-Kopie, lokale Kopie) sind Best-Effort und melden Fehler nur
	// per Log, nicht als Rückgabewert: ein SendMail-Aufruf, der dem Nutzer
	// "Fehlgeschlagen" zeigt, obwohl die Mail beim Empfänger angekommen
	// ist, wäre irreführender als eine fehlende Gesendet-Kopie.
	var sentMailbox string
	var sentUID uint32
	imapPw, err := secrets.Get(accountID, secrets.KindIMAP)
	if err != nil {
		log.Printf("Gesendet-Kopie konnte nicht auf dem Server abgelegt werden (IMAP-Passwort fehlt): %v", err)
	} else {
		sentMailbox, sentUID, err = mail.AppendSentWithUID(mail.IMAPConfig{
			Host: acc.IMAPHost, Port: acc.IMAPPort, Security: acc.IMAPSecurity,
			Username: acc.IMAPUsername, Password: imapPw,
		}, raw, time.Now())
		if err != nil {
			log.Printf("Gesendet-Kopie konnte nicht auf dem Server abgelegt werden: %v", err)
			sentMailbox, sentUID = "", 0
		}
	}
	if err := m.saveSentCopy(acc, to, subject, body, bodyHTML, mailAttachments, sentMailbox, sentUID); err != nil {
		log.Printf("Gesendet-Kopie konnte nicht lokal gespeichert werden: %v", err)
	}
	return nil
}

// saveSentCopy legt die gerade verschickte Mail lokal im Ordner
// FolderGesendet ab (siehe store.InsertSentMail) — inklusive Anhängen,
// damit sie auch aus der "Gesendet"-Ansicht heraus wieder herunterladbar
// sind, genau wie bei empfangenen Mails.
func (m *MailService) saveSentCopy(acc store.Account, to []string, subject, body, bodyHTML string, attachments []mail.Attachment, mailbox string, uid uint32) error {
	stored := store.Mail{
		UID:           uid,
		SenderName:    coalesceNonEmpty(acc.DisplayName, acc.Email),
		SenderEmail:   acc.Email,
		Recipients:    strings.Join(to, ", "),
		Subject:       subject,
		Preview:       mail.Preview(body, 140),
		BodyText:      body,
		BodyHTML:      bodyHTML,
		ReceivedAt:    time.Now(),
		HasAttachment: len(attachments) > 0,
	}
	var sentID int64
	var err error
	if mailbox != "" && uid > 0 {
		sentID, err = m.store.InsertMail(acc.ID, mailbox, stored)
	} else {
		sentID, err = m.store.InsertSentMail(acc.ID, stored)
	}
	if err != nil {
		return err
	}
	for _, a := range attachments {
		if err := m.store.InsertAttachment(sentID, store.Attachment{
			Filename: a.Filename, ContentType: a.ContentType, Size: int64(len(a.Data)), Data: a.Data,
		}); err != nil {
			return err
		}
	}
	return nil
}

func coalesceNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------- Avatar-Darstellung (deterministisch aus Name/E-Mail) ----------

func initials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	fields := strings.Fields(name)
	if len(fields) == 1 {
		r := []rune(fields[0])
		if len(r) >= 2 {
			return strings.ToUpper(string(r[:2]))
		}
		return strings.ToUpper(string(r))
	}
	first := []rune(fields[0])
	last := []rune(fields[len(fields)-1])
	return strings.ToUpper(string(first[0]) + string(last[0]))
}

var avatarPalette = []string{"steel", "moss", "amber", "rust", "slate"}

func avatarColor(email string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(email))) // hash.Hash.Write liefert nie einen Fehler
	switch h.Sum32() % 5 {
	case 0:
		return avatarPalette[0]
	case 1:
		return avatarPalette[1]
	case 2:
		return avatarPalette[2]
	case 3:
		return avatarPalette[3]
	default:
		return avatarPalette[4]
	}
}

// knownBrandDomains ordnet Absender-Domains bekannten Diensten zu, deren
// Logo im Frontend statt der Initialen angezeigt wird (siehe
// frontend/src/main.js, brandIcons — die eigentlichen SVGs liegen dort,
// hier wird nur der Slug bestimmt). Rein kosmetisch, ohne Einfluss auf
// Klassifizierung oder Zustellung.
var knownBrandDomains = map[string]string{
	"github.com": "github", "github.io": "github",
	"cloudflare.com": "cloudflare",
	"tailscale.com":  "tailscale",
	"paypal.com":     "paypal", "paypal.de": "paypal",
	"google.com": "google", "gmail.com": "google", "googlemail.com": "google",
	"microsoft.com": "microsoft", "outlook.com": "microsoft", "live.com": "microsoft", "office.com": "microsoft",
	"apple.com": "apple", "icloud.com": "apple",
	"amazon.com": "amazon", "amazon.de": "amazon", "amazonses.com": "amazon",
	"slack.com":        "slack",
	"stripe.com":       "stripe",
	"linkedin.com":     "linkedin",
	"notion.so":        "notion",
	"figma.com":        "figma",
	"dropbox.com":      "dropbox",
	"zoom.us":          "zoom",
	"spotify.com":      "spotify",
	"discord.com":      "discord",
	"discordapp.com":   "discord",
	"gitlab.com":       "gitlab",
	"digitalocean.com": "digitalocean",
	"vercel.com":       "vercel",
	"netlify.com":      "netlify",
	"atlassian.com":    "atlassian",
	"atlassian.net":    "atlassian",
	"docker.com":       "docker",
	"npmjs.com":        "npm",
}

// brandIcon prüft die Absender-Domain gegen knownBrandDomains — inklusive
// Subdomains (z. B. "notifications.github.com" -> "github"), indem
// probeweise von links Label für Label abgeschnitten wird.
func brandIcon(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	domain := strings.ToLower(email[at+1:])
	labels := strings.Split(domain, ".")
	for i := 0; i < len(labels)-1; i++ {
		if slug, ok := knownBrandDomains[strings.Join(labels[i:], ".")]; ok {
			return slug
		}
	}
	return ""
}
