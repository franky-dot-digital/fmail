package mail

import (
	"strings"
	"testing"
)

// Regressionstest für den Bug: eine eingebettete Grafik VOR dem eigentlichen
// Text (üblich bei Signaturen/Newslettern) durfte nicht mehr als "Text"
// durchgehen und den echten Body verdrängen.
func TestParseMessage_ImageBeforeText(t *testing.T) {
	raw := strings.ReplaceAll(`Content-Type: multipart/mixed; boundary="MIXED"

--MIXED
Content-Type: image/png
Content-Disposition: inline
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=
--MIXED
Content-Type: multipart/alternative; boundary="ALT"

--ALT
Content-Type: text/plain; charset=utf-8

Hallo, das ist der eigentliche Text.
--ALT
Content-Type: text/html; charset=utf-8

<html><body>Hallo, das ist der eigentliche Text.</body></html>
--ALT--
--MIXED--
`, "\n", "\r\n")

	fm := parseMessage(1, nil, []byte(raw))

	if fm.PlainText != "Hallo, das ist der eigentliche Text." {
		t.Fatalf("PlainText = %q, want the real text (image must not win)", fm.PlainText)
	}
}
