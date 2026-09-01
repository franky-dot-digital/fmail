// sanitize.go regelt, wie roh aus IMAP kommendes Mail-HTML sicher wird,
// bevor es irgendwo landet — als Klartext-Vorschau/-Fallback (PlainText-
// FromHTML, strikte Policy, killt jedes Tag) oder als tatsächlich
// gerendertes HTML (SanitizeHTML, Allowlist-Policy). Kein Skript, kein
// Style-Attribut, kein iframe/form/object entkommt hier — was danach an
// Restrisiko bleibt (v. a. Tracking über <img src="https://...">), fängt
// das Frontend ab, indem es das sanitierte HTML nur in einem sandboxed
// <iframe> ohne Skriptausführung rendert und Fremd-Bilder per CSP
// standardmäßig blockiert (siehe frontend/src/main.js, renderDetail()).
// Rohes HTML landet nie direkt per innerHTML im UI — das gilt weiterhin,
// nur eben über den iframe-Umweg statt komplett verboten.
package mail

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// stripPolicy entfernt ausnahmslos alle Tags (Skripte, Styles, Events,
// iframes, Formulare, eingebettete Bilder eingeschlossen) und liefert
// nur den Text dazwischen. Es ist bewusst *kein* AllowList-Policy mit
// z. B. <b>/<i> — für eine Vorschau/Textansicht reicht reiner Text, und
// jede erlaubte Tag-Art ist potenziell neue Angriffsfläche.
var stripPolicy = bluemonday.StrictPolicy()

var whitespaceRun = regexp.MustCompile(`[ \t\r\f\v]+`)
var blankLineRun = regexp.MustCompile(`\n{3,}`)

// PlainTextFromHTML wandelt einen HTML-Mailkörper in lesbaren Klartext um.
func PlainTextFromHTML(html string) string {
	text := stripPolicy.Sanitize(html)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = whitespaceRun.ReplaceAllString(text, " ")
	text = blankLineRun.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// Preview kürzt Text auf eine Vorschau-Länge (in Runen, nicht Bytes,
// damit Umlaute/Emoji nicht mitten im Zeichen abgeschnitten werden).
func Preview(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(text), " ") // eine Zeile
	r := []rune(text)
	if len(r) <= maxRunes {
		return text
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// htmlPolicy erlaubt einen festen Satz an Formatierungs-Tags — kein
// <script>, <style>, <iframe>, <object>, <form>, keine Event-Handler-
// oder style-Attribute (das schließt u. a. CSS-basiertes Tracking über
// background-image aus). Links nur http/https/mailto, automatisch mit
// rel="nofollow". Bilder nur als data:-URI (image/gif|jpeg|png|webp) —
// echte http(s)-Bild-URLs bleiben zwar im Markup erlaubt (harmlos als
// bloßer String), ob sie tatsächlich geladen werden, entscheidet aber
// erst das Frontend pro Mail, nicht diese Funktion (siehe Kommentar oben).
var htmlPolicy = newHTMLPolicy()

func newHTMLPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements(
		"p", "br", "b", "strong", "i", "em", "u", "s", "strike",
		"blockquote", "span", "div", "hr", "pre", "code",
		"h1", "h2", "h3", "h4", "h5", "h6",
	)
	p.AllowLists()
	p.AllowTables()

	p.AllowStandardURLs()
	p.AllowAttrs("href").OnElements("a")

	p.AllowImages()
	p.AllowDataURIImages()

	return p
}

// SanitizeHTML entfernt aus HTML alles außer der Allowlist aus
// newHTMLPolicy. Läuft sowohl über eingehende Mails (imapclient.go) als
// auch über ausgehendes HTML aus dem Compose-Editor (smtpclient.go) —
// bei Copy-Paste in ein contenteditable-Feld kann durchaus Unerwartetes
// landen, das lieber auch beim Versand nochmal durch dieselbe Policy soll.
func SanitizeHTML(html string) string {
	return htmlPolicy.Sanitize(html)
}

// InlineImage ist ein per Content-ID referenziertes eingebettetes Bild
// (Signaturen, Inline-Grafiken in HTML-Mails).
type InlineImage struct {
	ContentType string
	Data        []byte
}

var validImageContentType = regexp.MustCompile(`^image/[a-zA-Z0-9.+-]+$`)

// InlineCIDImages ersetzt "cid:xxx"-Bildreferenzen durch eingebettete
// data:-URIs — MUSS vor SanitizeHTML laufen, sonst verwirft die Policy
// das ihr unbekannte "cid:"-Schema ohnehin. Der Content-Type wird gegen
// eine strikte Allowlist geprüft, bevor er roh in ein HTML-Attribut
// eingebettet wird — ein Content-Type mit eingeschmuggeltem `"` aus einer
// böswillig gestalteten Mail könnte sonst aus dem src-Attribut ausbrechen.
func InlineCIDImages(html string, images map[string]InlineImage) string {
	for cid, img := range images {
		contentType := img.ContentType
		if !validImageContentType.MatchString(contentType) {
			contentType = "image/png"
		}
		dataURI := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
		html = strings.ReplaceAll(html, "cid:"+cid, dataURI)
	}
	return html
}
