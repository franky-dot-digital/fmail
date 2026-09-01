package mail

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

// TestBuildMessage_HTMLAlternative deckt beide HTML-Fälle ab: ohne
// Anhang (äußerste Ebene ist direkt multipart/alternative) und mit
// Anhang (multipart/alternative steckt verschachtelt in multipart/mixed).
func TestBuildMessage_HTMLAlternative(t *testing.T) {
	cases := []struct {
		name        string
		attachments []Attachment
	}{
		{"ohne Anhang", nil},
		{"mit Anhang", []Attachment{{Filename: "bild.png", ContentType: "image/png", Data: []byte("pngdata")}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := buildMessage("frank@example.com", "Frank", []string{"empfaenger@example.com"},
				"Betreff", "Hallo Welt", "<p>Hallo <b>Welt</b></p>", c.attachments)
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

			var sawText, sawHTML, sawAttachment bool
			var walk func(r io.Reader, boundary string)
			walk = func(r io.Reader, boundary string) {
				mr := multipart.NewReader(r, boundary)
				for {
					part, err := mr.NextPart()
					if err == io.EOF {
						return
					}
					if err != nil {
						t.Fatalf("NextPart: %v", err)
					}
					ct := part.Header.Get("Content-Type")
					pMediaType, pParams, err := mime.ParseMediaType(ct)
					if err == nil && strings.HasPrefix(pMediaType, "multipart/") {
						walk(part, pParams["boundary"])
						continue
					}
					body, _ := io.ReadAll(part)
					switch {
					case strings.HasPrefix(ct, "text/plain"):
						sawText = true
						if string(body) != "Hallo Welt" {
							t.Errorf("text/plain = %q, want %q", body, "Hallo Welt")
						}
					case strings.HasPrefix(ct, "text/html"):
						sawHTML = true
						if string(body) != "<p>Hallo <b>Welt</b></p>" {
							t.Errorf("text/html = %q, want %q", body, "<p>Hallo <b>Welt</b></p>")
						}
					case part.FileName() == "bild.png":
						sawAttachment = true
					}
				}
			}
			walk(msg.Body, params["boundary"])

			if !sawText {
				t.Error("text/plain-Part nicht gefunden")
			}
			if !sawHTML {
				t.Error("text/html-Part nicht gefunden")
			}
			if len(c.attachments) > 0 && !sawAttachment {
				t.Error("Anhang nicht gefunden")
			}
			if len(c.attachments) == 0 && !strings.HasPrefix(mediaType, "multipart/alternative") {
				t.Errorf("äußerster Content-Type = %q, want multipart/alternative (kein Anhang, keine mixed-Hülle nötig)", mediaType)
			}
			if len(c.attachments) > 0 && !strings.HasPrefix(mediaType, "multipart/mixed") {
				t.Errorf("äußerster Content-Type = %q, want multipart/mixed", mediaType)
			}
		})
	}
}
