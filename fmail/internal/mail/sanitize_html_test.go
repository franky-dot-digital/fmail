package mail

import (
	"strings"
	"testing"
)

func TestSanitizeHTML_StripsDangerousContent(t *testing.T) {
	raw := `<p>Hallo</p>
<script>alert('xss')</script>
<img src="x" onerror="alert(1)">
<a href="javascript:alert(1)">Klick</a>
<a href="https://example.com" onclick="evil()">Link</a>
<div style="background-image:url(https://tracker.example/pixel.gif)">Text</div>
<iframe src="https://evil.example"></iframe>
<form action="https://evil.example"><input></form>`

	out := SanitizeHTML(raw)

	forbidden := []string{"<script", "onerror", "onclick", "javascript:", "style=", "<iframe", "<form", "<input"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(out), strings.ToLower(f)) {
			t.Errorf("SanitizeHTML output still contains forbidden fragment %q\noutput: %s", f, out)
		}
	}
	if !strings.Contains(out, "Hallo") {
		t.Error("harmless text content was stripped")
	}
	if !strings.Contains(out, `href="https://example.com"`) {
		t.Error("legitimate https link was stripped")
	}
	if !strings.Contains(out, `rel="nofollow"`) {
		t.Error("expected rel=\"nofollow\" to be added to links")
	}
}

func TestSanitizeHTML_AllowsDataURIImages(t *testing.T) {
	out := SanitizeHTML(`<img src="data:image/png;base64,aGVsbG8=" alt="x">`)
	if !strings.Contains(out, "data:image/png;base64,aGVsbG8=") {
		t.Errorf("expected data URI image to survive sanitization, got: %s", out)
	}
}

func TestInlineCIDImages_ReplacesReferenceWithDataURI(t *testing.T) {
	html := `<img src="cid:logo123">`
	out := InlineCIDImages(html, map[string]InlineImage{
		"logo123": {ContentType: "image/png", Data: []byte("hello")},
	})
	want := `<img src="data:image/png;base64,aGVsbG8=">`
	if out != want {
		t.Errorf("InlineCIDImages = %q, want %q", out, want)
	}
}

func TestInlineCIDImages_RejectsInjectedContentType(t *testing.T) {
	html := `<img src="cid:evil">`
	out := InlineCIDImages(html, map[string]InlineImage{
		"evil": {ContentType: `image/png"><script>alert(1)</script>`, Data: []byte("x")},
	})
	if strings.Contains(out, "<script>") {
		t.Errorf("malicious Content-Type broke out of the attribute: %s", out)
	}
	if !strings.Contains(out, "data:image/png;base64,") {
		t.Errorf("expected fallback to image/png, got: %s", out)
	}
}
