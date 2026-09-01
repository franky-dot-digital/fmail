// smtpclient.go verschickt Mail über net/smtp mit manuell erzwungener
// Verschlüsselung. Zwei Sicherheitsentscheidungen sind hier bewusst und
// nicht Zufall:
//
//  1. STARTTLS wird nur akzeptiert, wenn der Server die Extension
//     tatsächlich anbietet UND das Upgrade erfolgreich war — sonst wird
//     die Verbindung abgebrochen, statt (wie eine naive net/smtp-Nutzung
//     es täte) still auf Klartext zurückzufallen.
//  2. Jeder Wert, der in eine Kopfzeile wandert (From/To/Subject), wird
//     von CR/LF befreit. Ohne das könnte ein Betreff mit eingebettetem
//     "\r\nBcc: angreifer@böse.example" zusätzliche Kopfzeilen einschmuggeln
//     (Header-/SMTP-Injection) — ein klassischer, leicht übersehener Bug
//     in selbstgebauten Mail-Sendern.
package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Security string // "tls" (implizit, i. d. R. Port 465) | "starttls" (i. d. R. Port 587)
	Username string
	Password string
}

type Attachment struct {
	Filename    string
	ContentType string // leer -> application/octet-stream
	Data        []byte
}

type OutgoingMail struct {
	From        string
	FromName    string // Anzeigename fürs From-Feld, z. B. "Frank Lewandowski" — optional
	To          []string
	Subject     string
	Body        string // Klartext-Variante — bei BodyHTML != "" der Alternative-Part fürs Fallback, sonst der einzige Inhalt
	BodyHTML    string // optional; wird vor dem Versand nochmal durch SanitizeHTML gejagt (siehe Send)
	Attachments []Attachment
}

// TestSMTPConnection prüft Verbindung, Verschlüsselung und Login, ohne
// eine Mail zu senden.
func TestSMTPConnection(cfg SMTPConfig) error {
	client, err := dialSMTP(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Quit()
}

// Send verschickt eine Mail über SMTP und liefert die tatsächlich
// gesendeten Rohbytes zurück — der Aufrufer braucht sie, um exakt dieselbe
// Nachricht per IMAP APPEND in den Gesendet-Ordner des Servers zu legen
// (siehe AppendSent), statt sie ein zweites Mal (potenziell abweichend)
// zusammenzubauen.
func Send(cfg SMTPConfig, msg OutgoingMail) ([]byte, error) {
	fromAddr, err := validateAddress(msg.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: Absenderadresse ungültig: %w", err)
	}
	toAddrs := make([]string, 0, len(msg.To))
	for _, to := range msg.To {
		addr, err := validateAddress(to)
		if err != nil {
			return nil, fmt.Errorf("smtp: Empfängeradresse ungültig: %w", err)
		}
		toAddrs = append(toAddrs, addr)
	}
	if len(toAddrs) == 0 {
		return nil, fmt.Errorf("smtp: mindestens ein Empfänger ist erforderlich")
	}

	client, err := dialSMTP(cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Mail(fromAddr); err != nil {
		return nil, fmt.Errorf("smtp: MAIL FROM fehlgeschlagen: %w", err)
	}
	for _, to := range toAddrs {
		if err := client.Rcpt(to); err != nil {
			return nil, fmt.Errorf("smtp: RCPT TO %s fehlgeschlagen: %w", to, err)
		}
	}

	// Auch selbst verfasstes HTML läuft nochmal durch SanitizeHTML — der
	// Compose-Editor ist ein contenteditable-Feld, und was beim
	// Reinkopieren aus einer Webseite alles an Markup mitkommen kann, ist
	// nicht vorhersehbar. Kostet nichts und schließt aus, dass eine
	// verunglückte Paste-Aktion etwas Unerwartetes verschickt.
	bodyHTML := msg.BodyHTML
	if bodyHTML != "" {
		bodyHTML = SanitizeHTML(bodyHTML)
	}

	raw, err := buildMessage(fromAddr, msg.FromName, toAddrs, msg.Subject, msg.Body, bodyHTML, msg.Attachments)
	if err != nil {
		return nil, fmt.Errorf("smtp: Nachricht konnte nicht zusammengestellt werden: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return nil, fmt.Errorf("smtp: DATA fehlgeschlagen: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("smtp: Senden fehlgeschlagen: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("smtp: Senden fehlgeschlagen: %w", err)
	}

	if err := client.Quit(); err != nil {
		return nil, fmt.Errorf("smtp: Senden fehlgeschlagen: %w", err)
	}
	return raw, nil
}

func dialSMTP(cfg SMTPConfig) (*smtp.Client, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	var client *smtp.Client

	switch cfg.Security {
	case "tls", "":
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("smtp: TLS-Verbindung fehlgeschlagen: %w", err)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp: Client-Initialisierung fehlgeschlagen: %w", err)
		}
	case "starttls":
		conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("smtp: Verbindung fehlgeschlagen: %w", err)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp: Client-Initialisierung fehlgeschlagen: %w", err)
		}
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, fmt.Errorf("smtp: Server bietet kein STARTTLS an — Verbindung wird ohne Verschlüsselung verweigert")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("smtp: STARTTLS fehlgeschlagen: %w", err)
		}
	default:
		return nil, fmt.Errorf("smtp: unbekannte Sicherheitsstufe %q (erwartet: tls oder starttls)", cfg.Security)
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("smtp: Anmeldung fehlgeschlagen: %w", err)
		}
	}

	return client, nil
}

// validateAddress lehnt alles ab, was net/mail nicht als eine einzelne,
// saubere Adresse parst — insbesondere Steuerzeichen, die für
// SMTP-Command-Injection über RCPT/MAIL missbraucht werden könnten.
func validateAddress(addr string) (string, error) {
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		return "", fmt.Errorf("%q ist keine gültige Adresse: %w", addr, err)
	}
	return parsed.Address, nil
}

// sanitizeHeaderValue entfernt CR/LF aus Werten, die in eine Kopfzeile
// eingebettet werden. Das verhindert Header-Injection über Betreffzeilen
// mit eingebetteten Zeilenumbrüchen.
func sanitizeHeaderValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// fromHeader formatiert den From-Header über mail.Address, statt den
// Anzeigenamen roh in den String zu verketten — das übernimmt Quoting
// und RFC-2047-Encoding (Umlaute etc.) korrekt und schließt nebenbei
// Header-Injection über den Anzeigenamen aus.
func fromHeader(addr, name string) string {
	return (&mail.Address{Name: sanitizeHeaderValue(name), Address: addr}).String()
}

// buildMessage baut je nach dem, was da ist, eine von vier MIME-Formen:
//
//	nur Text                          -> text/plain
//	Text + Anhänge                    -> multipart/mixed(text/plain, Anhang, ...)
//	Text + HTML                       -> multipart/alternative(text/plain, text/html)
//	Text + HTML + Anhänge             -> multipart/mixed(multipart/alternative(...), Anhang, ...)
//
// mime/multipart übernimmt Boundary-Erzeugung und -Quoting — kein
// selbstgebautes Trennzeichen, das sich mit Inhalt überschneiden könnte.
func buildMessage(from, fromName string, to []string, subject, body, bodyHTML string, attachments []Attachment) ([]byte, error) {
	var header strings.Builder
	fmt.Fprintf(&header, "From: %s\r\n", fromHeader(from, fromName))
	fmt.Fprintf(&header, "To: %s\r\n", sanitizeHeaderValue(strings.Join(to, ", ")))
	fmt.Fprintf(&header, "Subject: %s\r\n", sanitizeHeaderValue(subject))
	header.WriteString("MIME-Version: 1.0\r\n")

	if bodyHTML == "" && len(attachments) == 0 {
		header.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		header.WriteString(body)
		return []byte(header.String()), nil
	}

	if len(attachments) == 0 {
		altBytes, altBoundary, err := buildAlternativeBody(body, bodyHTML)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&header, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary)
		header.Write(altBytes)
		return []byte(header.String()), nil
	}

	var mixed bytes.Buffer
	mw := multipart.NewWriter(&mixed)

	if bodyHTML == "" {
		textPart, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}})
		if err != nil {
			return nil, err
		}
		if _, err := textPart.Write([]byte(body)); err != nil {
			return nil, err
		}
	} else {
		altBytes, altBoundary, err := buildAlternativeBody(body, bodyHTML)
		if err != nil {
			return nil, err
		}
		altPart, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type": {fmt.Sprintf(`multipart/alternative; boundary=%q`, altBoundary)},
		})
		if err != nil {
			return nil, err
		}
		if _, err := altPart.Write(altBytes); err != nil {
			return nil, err
		}
	}

	for _, a := range attachments {
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		part, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {contentType},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {fmt.Sprintf("attachment; filename=%q", sanitizeHeaderValue(a.Filename))},
		})
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(a.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := min(i+76, len(encoded))
			if _, err := part.Write([]byte(encoded[i:end] + "\r\n")); err != nil {
				return nil, err
			}
		}
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	fmt.Fprintf(&header, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mw.Boundary())
	header.Write(mixed.Bytes())
	return []byte(header.String()), nil
}

// buildAlternativeBody baut den Inhalt eines multipart/alternative-Teils
// (Klartext + HTML) und liefert dessen Bytes plus die verwendete
// Boundary, damit der Aufrufer den passenden Content-Type-Header setzen
// kann — egal ob das direkt die äußerste Ebene ist oder verschachtelt in
// einem multipart/mixed für Anhänge steckt.
func buildAlternativeBody(body, bodyHTML string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	textPart, err := w.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}})
	if err != nil {
		return nil, "", err
	}
	if _, err := textPart.Write([]byte(body)); err != nil {
		return nil, "", err
	}

	htmlPart, err := w.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/html; charset=utf-8"}})
	if err != nil {
		return nil, "", err
	}
	if _, err := htmlPart.Write([]byte(bodyHTML)); err != nil {
		return nil, "", err
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.Boundary(), nil
}
