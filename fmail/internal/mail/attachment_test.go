package mail

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

// Regressionstest für Anhänge: buildMessage muss eine gültige
// multipart/mixed-Nachricht erzeugen, die sich mit Go's eigenem
// MIME-Parser wieder in Text-Body und Anhang zerlegen lässt.
func TestBuildMessage_WithAttachment(t *testing.T) {
	raw, err := buildMessage("frank@example.com", "Frank Lewandowski", []string{"empfaenger@example.com"},
		"Testbetreff", "Hallo Welt", "", []Attachment{
			{Filename: "rechnung.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4 fake content")},
		})
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("Nachricht konnte nicht geparst werden: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type ungültig: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/mixed") {
		t.Fatalf("Content-Type = %q, want multipart/mixed", mediaType)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var sawText, sawAttachment bool
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("Part lesen: %v", err)
		}
		switch {
		case strings.HasPrefix(part.Header.Get("Content-Type"), "text/plain"):
			sawText = true
			if string(body) != "Hallo Welt" {
				t.Errorf("Text-Body = %q, want %q", body, "Hallo Welt")
			}
		case part.FileName() == "rechnung.pdf":
			sawAttachment = true
			// multipart.Reader entkodiert Content-Transfer-Encoding nicht
			// automatisch (nur die MIME-Struktur) — das übernimmt hier der
			// Test selbst, so wie es jeder echte Mailclient auch tut.
			if part.Header.Get("Content-Transfer-Encoding") != "base64" {
				t.Fatalf("Content-Transfer-Encoding = %q, want base64", part.Header.Get("Content-Transfer-Encoding"))
			}
			decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(body), "\r\n", ""))
			if err != nil {
				t.Fatalf("Anhang konnte nicht Base64-dekodiert werden: %v", err)
			}
			if string(decoded) != "%PDF-1.4 fake content" {
				t.Errorf("Anhang-Inhalt = %q, want %q", decoded, "%PDF-1.4 fake content")
			}
		}
	}
	if !sawText {
		t.Error("Text-Part nicht gefunden")
	}
	if !sawAttachment {
		t.Error("Anhang-Part nicht gefunden")
	}
}
