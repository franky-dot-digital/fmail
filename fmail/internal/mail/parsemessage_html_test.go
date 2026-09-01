package mail

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestParseMessage_RecipientsFromEnvelope(t *testing.T) {
	fm := parseMessage(7, &imap.Envelope{To: []imap.Address{
		{Mailbox: "Bob", Host: "Example.COM"},
		{Mailbox: "carol", Host: "example.net"},
	}}, nil)
	if got, want := strings.Join(fm.Recipients, ", "), "bob@example.com, carol@example.net"; got != want {
		t.Fatalf("Recipients = %q, want %q", got, want)
	}
}

// Regressionstest: eine Mail mit HTML-Body, eingebettetem Inline-Bild
// (cid) und einem echten Anhang muss alle drei sauber trennen.
func TestParseMessage_HTMLWithInlineImageAndAttachment(t *testing.T) {
	logoData := "logodata"
	logoB64 := base64.StdEncoding.EncodeToString([]byte(logoData))
	pdfData := "pdfdata"
	pdfB64 := base64.StdEncoding.EncodeToString([]byte(pdfData))

	raw := strings.ReplaceAll(`Content-Type: multipart/mixed; boundary="MIXED"

--MIXED
Content-Type: multipart/related; boundary="REL"

--REL
Content-Type: multipart/alternative; boundary="ALT"

--ALT
Content-Type: text/plain; charset=utf-8

Hallo Welt
--ALT
Content-Type: text/html; charset=utf-8

<html><body><p>Hallo <b>Welt</b></p><img src="cid:logo1"><script>alert(1)</script></body></html>
--ALT--
--REL
Content-Type: image/png
Content-Disposition: inline
Content-ID: <logo1>
Content-Transfer-Encoding: base64

`+logoB64+`
--REL--
--MIXED
Content-Type: application/pdf
Content-Disposition: attachment; filename="rechnung.pdf"
Content-Transfer-Encoding: base64

`+pdfB64+`
--MIXED--
`, "\n", "\r\n")

	fm := parseMessage(1, nil, []byte(raw))

	if fm.PlainText != "Hallo Welt" {
		t.Errorf("PlainText = %q, want %q", fm.PlainText, "Hallo Welt")
	}
	if strings.Contains(fm.HTMLBody, "<script>") {
		t.Errorf("HTMLBody enthält <script>: %s", fm.HTMLBody)
	}
	wantDataURI := "data:image/png;base64," + logoB64
	if !strings.Contains(fm.HTMLBody, wantDataURI) {
		t.Errorf("HTMLBody enthält nicht das eingebettete Inline-Bild %q, got: %s", wantDataURI, fm.HTMLBody)
	}
	if strings.Contains(fm.HTMLBody, "cid:") {
		t.Errorf("HTMLBody enthält noch eine unaufgelöste cid:-Referenz: %s", fm.HTMLBody)
	}

	if !fm.HasAttachment {
		t.Error("HasAttachment = false, want true")
	}
	if len(fm.Attachments) != 1 {
		t.Fatalf("len(Attachments) = %d, want 1 (Inline-Bild darf nicht auch als Anhang auftauchen)", len(fm.Attachments))
	}
	a := fm.Attachments[0]
	if a.Filename != "rechnung.pdf" {
		t.Errorf("Attachment.Filename = %q, want %q", a.Filename, "rechnung.pdf")
	}
	if string(a.Data) != pdfData {
		t.Errorf("Attachment.Data = %q, want %q", a.Data, pdfData)
	}
}
