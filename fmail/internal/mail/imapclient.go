// imapclient.go verbindet sich mit dem Posteingang. Sicherheitsregeln,
// die hier absichtlich gelten:
//
//   - Es gibt keinen Codepfad ohne Verschlüsselung. dial() kennt nur
//     "tls" (implizit, Port 993) und "starttls" (Port 143/anderer Port
//     mit Befehl vor AUTH). Ein drittes, unverschlüsseltes Verfahren
//     existiert im Typsystem gar nicht.
//   - TLS-Mindestversion 1.2, kein InsecureSkipVerify, keine eigene
//     Zertifikatsprüfung, die die Standardvalidierung von crypto/tls
//     abschwächt.
//   - IMAP-Befehle laufen über die strukturierte Client-API
//     (Select/Fetch/UIDSet ...), nie über selbst zusammengebaute
//     Kommando-Strings — Command-Injection auf Protokollebene ist damit
//     kategorisch ausgeschlossen.
package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	gomail "github.com/emersion/go-message/mail"
)

type IMAPConfig struct {
	Host     string
	Port     int
	Security string // "tls" | "starttls"
	Username string
	Password string
}

// FetchedAttachment ist ein "echter" Anhang (Content-Disposition:
// attachment) — Inline-Bilder (Content-Disposition: inline mit
// Content-ID) landen stattdessen direkt eingebettet in HTMLBody, siehe
// InlineCIDImages, und tauchen hier nicht auf.
type FetchedAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type FetchedMessage struct {
	UID           uint32
	SenderName    string
	SenderEmail   string
	Recipients    []string
	Subject       string
	Date          time.Time
	PlainText     string
	HTMLBody      string // bereits sanitiert (SanitizeHTML) und mit eingebetteten cid-Bildern; leer, wenn die Mail keine HTML-Variante hatte
	HasAttachment bool
	Attachments   []FetchedAttachment
}

// maxAttachmentBytes begrenzt, wie viel ein einzelner Anhang beim Sync in
// die lokale SQLite-DB wandert. Größere Anhänge werden stillschweigend
// übersprungen (die Mail selbst wird trotzdem importiert) — lieber das
// als eine einzelne große Mail die App/DB aufblähen zu lassen.
const maxAttachmentBytes = 25 * 1024 * 1024

// maxMessageBytes begrenzt den gesamten RFC-822-Rohinhalt. Ohne eine
// partielle FETCH-Anfrage könnte eine einzelne bösartige Nachricht den
// Arbeitsspeicher und anschließend die lokale Datenbank unbeschränkt füllen.
const maxMessageBytes = 50 * 1024 * 1024

func dial(host string, port int, security string) (*imapclient.Client, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	options := &imapclient.Options{TLSConfig: tlsConfig}

	switch security {
	case "starttls":
		return imapclient.DialStartTLS(addr, options)
	case "tls", "":
		return imapclient.DialTLS(addr, options)
	default:
		return nil, fmt.Errorf("imap: unbekannte Sicherheitsstufe %q (erwartet: tls oder starttls)", security)
	}
}

// TestIMAPConnection prüft Verbindung + Login, ohne etwas zu synchronisieren.
func TestIMAPConnection(cfg IMAPConfig) error {
	c, err := dial(cfg.Host, cfg.Port, cfg.Security)
	if err != nil {
		return fmt.Errorf("Verbindung fehlgeschlagen: %w", err)
	}
	defer c.Close()

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return fmt.Errorf("Anmeldung fehlgeschlagen: %w", err)
	}
	return c.Logout().Wait()
}

// commonSentMailboxNames sind Ordnernamen, die verbreitete Server/Clients
// für "Gesendet" verwenden, falls ein Server SPECIAL-USE (RFC 6154) nicht
// unterstützt oder den \Sent-Attribut nicht gesetzt hat — dann bleibt nur
// das Raten anhand bekannter Konventionen.
var commonSentMailboxNames = []string{
	"Sent", "Sent Items", "Sent Messages", "Gesendet", "Gesendete Elemente",
	"INBOX.Sent", "INBOX/Sent", "INBOX.Sent Items", "INBOX.Gesendet",
}

var commonTrashMailboxNames = []string{
	"Trash", "Deleted Items", "Deleted Messages", "Papierkorb", "Gelöscht", "Gelöschte Elemente",
	"INBOX.Trash", "INBOX/Trash", "INBOX.Deleted Items", "INBOX.Papierkorb",
}

// findSentMailbox fragt die Ordnerliste vom Server ab und delegiert die
// eigentliche Auswahl an pickSentMailbox (siehe dort) — als eigene,
// von der Netzwerkschicht getrennte Funktion, damit die Auswahllogik ohne
// einen echten/simulierten IMAP-Server testbar ist.
func findSentMailbox(c *imapclient.Client) (string, error) {
	entries, err := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true}).Collect()
	if err != nil {
		return "", fmt.Errorf("Ordnerliste konnte nicht gelesen werden: %w", err)
	}
	return pickSentMailbox(entries)
}

// FindSentMailbox ermittelt den Gesendet-Ordner eines Kontos. Der Name ist
// serverabhängig (z. B. "Sent", "Gesendet" oder ein SPECIAL-USE-Ordner).
func FindSentMailbox(cfg IMAPConfig) (string, error) {
	c, err := dial(cfg.Host, cfg.Port, cfg.Security)
	if err != nil {
		return "", fmt.Errorf("Verbindung fehlgeschlagen: %w", err)
	}
	defer c.Close()
	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return "", fmt.Errorf("Anmeldung fehlgeschlagen: %w", err)
	}
	defer c.Logout()
	return findSentMailbox(c)
}

// pickSentMailbox ermittelt den Gesendet-Ordner aus einer Ordnerliste:
// zuerst über das \Sent-Attribut (SPECIAL-USE), sonst per Namensabgleich
// gegen commonSentMailboxNames — case-insensitiv, da Server "Sent" wie
// "SENT" oder "sent" benennen können. Das \Sent-Attribut hat immer Vorrang
// vor einem Namenstreffer, falls beides zufällig zutrifft.
func pickSentMailbox(entries []*imap.ListData) (string, error) {
	for _, e := range entries {
		for _, a := range e.Attrs {
			if a == imap.MailboxAttrSent {
				return e.Mailbox, nil
			}
		}
	}

	byLowerName := make(map[string]string, len(entries))
	for _, e := range entries {
		byLowerName[strings.ToLower(e.Mailbox)] = e.Mailbox
	}
	for _, candidate := range commonSentMailboxNames {
		if name, ok := byLowerName[strings.ToLower(candidate)]; ok {
			return name, nil
		}
	}

	return "", fmt.Errorf("kein Gesendet-Ordner auf dem Server gefunden (weder \\Sent-Attribut noch bekannter Name)")
}

func pickTrashMailbox(entries []*imap.ListData) (string, error) {
	for _, e := range entries {
		for _, a := range e.Attrs {
			if a == imap.MailboxAttrTrash {
				return e.Mailbox, nil
			}
		}
	}
	byLowerName := make(map[string]string, len(entries))
	for _, e := range entries {
		byLowerName[strings.ToLower(e.Mailbox)] = e.Mailbox
	}
	for _, candidate := range commonTrashMailboxNames {
		if name, ok := byLowerName[strings.ToLower(candidate)]; ok {
			return name, nil
		}
	}
	return "", fmt.Errorf("kein Papierkorb auf dem Server gefunden (weder \\Trash-Attribut noch bekannter Name)")
}

// MoveToTrash verschiebt eine Nachricht anhand ihrer stabilen IMAP-UID in
// den Papierkorb. Der Client fällt für ältere Server automatisch von MOVE
// auf COPY + \\Deleted + EXPUNGE zurück.
func MoveToTrash(cfg IMAPConfig, mailbox string, uid uint32) error {
	c, err := dial(cfg.Host, cfg.Port, cfg.Security)
	if err != nil {
		return fmt.Errorf("Verbindung fehlgeschlagen: %w", err)
	}
	defer c.Close()
	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return fmt.Errorf("Anmeldung fehlgeschlagen: %w", err)
	}
	defer c.Logout()
	return moveToTrashOnConn(c, mailbox, uid)
}

func moveToTrashOnConn(c *imapclient.Client, mailbox string, uid uint32) error {
	entries, err := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true}).Collect()
	if err != nil {
		return fmt.Errorf("Ordnerliste konnte nicht gelesen werden: %w", err)
	}
	trash, err := pickTrashMailbox(entries)
	if err != nil {
		return err
	}
	if _, err := c.Select(mailbox, nil).Wait(); err != nil {
		return fmt.Errorf("Postfach %q konnte nicht geöffnet werden: %w", mailbox, err)
	}
	var uidSet imap.UIDSet
	uidSet.AddNum(imap.UID(uid))
	if _, err := c.Move(uidSet, trash).Wait(); err != nil {
		return fmt.Errorf("Mail konnte nicht in %q verschoben werden: %w", trash, err)
	}
	return nil
}

// AppendSent legt eine bereits per SMTP verschickte Nachricht zusätzlich
// per IMAP APPEND im Gesendet-Ordner des Servers ab — SMTP allein
// speichert nirgendwo eine Kopie, das übernehmen normalerweise die
// Mail-Clients selbst (Thunderbird, Outlook, Webmail tun das genauso).
// raw MUSS exakt das sein, was tatsächlich per SMTP gesendet wurde (siehe
// Send in smtpclient.go), damit die Serverkopie nicht vom real
// Zugestellten abweicht. Mit \Seen markiert, weil man den eigenen
// Versand ja bereits "gelesen" hat.
func AppendSent(cfg IMAPConfig, raw []byte, sentAt time.Time) error {
	_, _, err := AppendSentWithUID(cfg, raw, sentAt)
	return err
}

// AppendSentWithUID liefert zusätzlich Ordner und UID der Serverkopie. Damit
// kann die lokale Sofortkopie später beim Sync eindeutig wiedererkannt werden.
func AppendSentWithUID(cfg IMAPConfig, raw []byte, sentAt time.Time) (string, uint32, error) {
	c, err := dial(cfg.Host, cfg.Port, cfg.Security)
	if err != nil {
		return "", 0, fmt.Errorf("Verbindung fehlgeschlagen: %w", err)
	}
	defer c.Close()

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return "", 0, fmt.Errorf("Anmeldung fehlgeschlagen: %w", err)
	}
	defer c.Logout()

	mailbox, err := findSentMailbox(c)
	if err != nil {
		return "", 0, err
	}
	uid, err := appendSentToMailbox(c, mailbox, raw, sentAt)
	return mailbox, uid, err
}

// appendSentToConn führt die eigentlichen LIST/APPEND-Befehle auf einer
// bereits verbundenen und eingeloggten Session aus — getrennt von
// AppendSent (die den sicheren dial()-Aufbau übernimmt), damit dieser Teil
// gegen einen simulierten IMAP-Server getestet werden kann, ohne den
// TLS-Verbindungsaufbau nachbilden zu müssen.
func appendSentToConn(c *imapclient.Client, raw []byte, sentAt time.Time) error {
	mailbox, err := findSentMailbox(c)
	if err != nil {
		return err
	}
	_, err = appendSentToMailbox(c, mailbox, raw, sentAt)
	return err
}

func appendSentToMailbox(c *imapclient.Client, mailbox string, raw []byte, sentAt time.Time) (uint32, error) {
	appendCmd := c.Append(mailbox, int64(len(raw)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
		Time:  sentAt,
	})
	if _, err := appendCmd.Write(raw); err != nil {
		_ = appendCmd.Close()
		return 0, fmt.Errorf("Mail konnte nicht in %q geschrieben werden: %w", mailbox, err)
	}
	if err := appendCmd.Close(); err != nil {
		return 0, fmt.Errorf("APPEND nach %q fehlgeschlagen: %w", mailbox, err)
	}
	data, err := appendCmd.Wait()
	if err != nil {
		return 0, fmt.Errorf("APPEND nach %q fehlgeschlagen: %w", mailbox, err)
	}
	return uint32(data.UID), nil
}

// FetchNew holt alle Nachrichten mit UID > sinceUID aus der angegebenen
// Mailbox (read-only geöffnet) und liefert sie zusammen mit der höchsten
// gesehenen UID zurück, damit der Aufrufer sync_state fortschreiben kann.
func FetchNew(cfg IMAPConfig, mailbox string, sinceUID uint32) ([]FetchedMessage, uint32, error) {
	c, err := dial(cfg.Host, cfg.Port, cfg.Security)
	if err != nil {
		return nil, sinceUID, fmt.Errorf("Verbindung fehlgeschlagen: %w", err)
	}
	defer c.Close()

	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return nil, sinceUID, fmt.Errorf("Anmeldung fehlgeschlagen: %w", err)
	}
	defer c.Logout()

	selectData, err := c.Select(mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, sinceUID, fmt.Errorf("Postfach %q konnte nicht geöffnet werden: %w", mailbox, err)
	}
	if selectData.NumMessages == 0 {
		return nil, sinceUID, nil
	}

	var uidSet imap.UIDSet
	uidSet.AddRange(imap.UID(sinceUID+1), 0) // 0 == "*" (offenes Ende)

	bodySection := &imap.FetchItemBodySection{Partial: &imap.SectionPartial{Offset: 0, Size: maxMessageBytes + 1}}
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Envelope:    true,
		RFC822Size:  true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	fetchCmd := c.Fetch(uidSet, fetchOptions)
	defer fetchCmd.Close()

	var out []FetchedMessage
	highest := sinceUID

	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}
		buf, err := msg.Collect()
		if err != nil {
			return nil, highest, fmt.Errorf("Nachricht konnte nicht gelesen werden: %w", err)
		}
		if uint32(buf.UID) > highest {
			highest = uint32(buf.UID)
		}
		raw := buf.FindBodySection(bodySection)
		if buf.RFC822Size > maxMessageBytes || len(raw) > maxMessageBytes {
			continue
		}
		out = append(out, parseMessage(uint32(buf.UID), buf.Envelope, raw))
	}
	if err := fetchCmd.Close(); err != nil {
		return nil, highest, fmt.Errorf("FETCH fehlgeschlagen: %w", err)
	}

	return out, highest, nil
}

func parseMessage(uid uint32, env *imap.Envelope, raw []byte) FetchedMessage {
	fm := FetchedMessage{UID: uid}

	if env != nil {
		fm.Subject = env.Subject
		fm.Date = env.Date
		if len(env.From) > 0 {
			fm.SenderName = env.From[0].Name
			fm.SenderEmail = strings.ToLower(env.From[0].Addr())
		}
		for _, addr := range env.To {
			if email := strings.ToLower(addr.Addr()); email != "" {
				fm.Recipients = append(fm.Recipients, email)
			}
		}
	}
	if fm.SenderName == "" {
		fm.SenderName = fm.SenderEmail
	}

	if raw == nil {
		return fm
	}

	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		// Envelope-Daten (Betreff/Absender) bleiben trotzdem nutzbar,
		// nur der Body bleibt in diesem Fall leer.
		return fm
	}

	// Eine Mail mit eingebetteten Bildern (Signatur-Logo, Inline-Grafiken in
	// HTML-Mails) hat mehrere *gomail.InlineHeader-Parts — nicht nur der
	// Text ist "inline", Bilder mit Content-Disposition: inline sind es
	// genauso. Die werden hier gesammelt und später per Content-ID in den
	// HTML-Body eingebettet (InlineCIDImages), statt als Text durchzugehen
	// oder als eigenständiger Anhang aufzutauchen.
	var plainText, htmlText string
	inlineImages := map[string]InlineImage{}
	var attachments []FetchedAttachment

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			contentType, _, _ := h.ContentType()
			contentType = strings.ToLower(contentType)
			switch {
			case plainText == "" && contentType == "text/plain":
				raw, _ := io.ReadAll(p.Body)
				plainText = strings.TrimSpace(string(raw))
			case htmlText == "" && contentType == "text/html":
				raw, _ := io.ReadAll(p.Body)
				htmlText = string(raw)
			case strings.HasPrefix(contentType, "image/"):
				if cid := strings.Trim(headerText(h, "Content-Id"), "<>"); cid != "" {
					if data, ok := readLimited(p.Body); ok {
						inlineImages[cid] = InlineImage{ContentType: contentType, Data: data}
					}
				}
			}
		case *gomail.AttachmentHeader:
			fm.HasAttachment = true
			filename, _ := h.Filename()
			if filename == "" {
				filename = "Anhang"
			}
			contentType, _, _ := h.ContentType()
			if data, ok := readLimited(p.Body); ok {
				attachments = append(attachments, FetchedAttachment{
					Filename: filename, ContentType: contentType, Data: data,
				})
			}
			// bei Überschreitung von maxAttachmentBytes wird der Anhang
			// stillschweigend übersprungen, siehe Kommentar an der Konstante
		}
	}

	switch {
	case plainText != "":
		fm.PlainText = plainText
	case htmlText != "":
		fm.PlainText = PlainTextFromHTML(htmlText)
	}
	if htmlText != "" {
		fm.HTMLBody = SanitizeHTML(InlineCIDImages(htmlText, inlineImages))
	}
	fm.Attachments = attachments

	return fm
}

// headerText liest einen einzelnen Header-Wert und ignoriert Fehler
// (z. B. wenn der Header fehlt) — für optionale Header wie Content-Id
// reicht das, ein fehlender Wert ist kein Abbruchgrund.
func headerText(h interface{ Text(string) (string, error) }, key string) string {
	v, _ := h.Text(key)
	return v
}

// readLimited liest einen Part bis maxAttachmentBytes+1 und meldet false,
// wenn das Limit überschritten wurde (statt den Rest still abzuschneiden,
// was einen kaputten Anhang erzeugen würde).
func readLimited(r io.Reader) ([]byte, bool) {
	data, err := io.ReadAll(io.LimitReader(r, maxAttachmentBytes+1))
	if err != nil || len(data) > maxAttachmentBytes {
		return nil, false
	}
	return data, true
}
