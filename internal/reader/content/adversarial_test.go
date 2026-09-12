package content_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/reader/content"
)

// Adversarial fixtures that must be neutralised. Each asserts a hostile construct
// does not survive SanitizeHTML / SanitizeCSS.
func TestAdversarialHTML_NeutralisedPayloads(t *testing.T) {
	cases := []struct {
		name string
		in   string
		bad  []string // substrings that must NOT appear in the output
	}{
		{
			name: "uppercase and whitespace-obfuscated script tag",
			in:   "<SCRIPT\n>alert(1)</SCRIPT\n>",
			bad:  []string{"alert(1)", "<script", "<SCRIPT"},
		},
		{
			name: "svg with foreignObject reintroducing html",
			in:   `<svg><foreignObject><body><img src=x onerror=alert(1)></body></foreignObject></svg>`,
			bad:  []string{"onerror", "foreignObject", "<svg"},
		},
		{
			name: "svg use xlink:href javascript",
			in:   `<svg><use xlink:href="javascript:alert(1)"/></svg>`,
			bad:  []string{"javascript:", "xlink", "<svg", "<use"},
		},
		{
			name: "nested style attribute with external url",
			in:   `<div style="x:url(https://evil.example/p)"><span style="y:url(//evil.example/q)">t</span></div>`,
			bad:  []string{"evil.example", "style="},
		},
		{
			name: "data-uri html smuggled in iframe",
			in:   `<iframe src="data:text/html,<script>alert(1)</script>"></iframe>`,
			bad:  []string{"<iframe", "alert(1)"},
		},
		{
			name: "meta refresh redirect",
			in:   `<meta http-equiv="refresh" content="0;url=http://evil.example">`,
			bad:  []string{"evil.example", "http-equiv", "<meta"},
		},
		{
			name: "base tag rewriting relative urls",
			in:   `<base href="http://evil.example/"><img src="a.png">`,
			bad:  []string{"evil.example", "<base"},
		},
		{
			name: "link prefetch external",
			in:   `<link rel="prefetch" href="http://evil.example/track">`,
			bad:  []string{"evil.example", "prefetch"},
		},
		{
			name: "link stylesheet relative kept but external stripped",
			in:   `<link rel="stylesheet" href="styles/book.css"><link rel="stylesheet" href="https://evil.example/x.css">`,
			bad:  []string{"evil.example"},
		},
		{
			name: "protocol-relative image src",
			in:   `<img src="//evil.example/pixel.gif">`,
			bad:  []string{"evil.example"},
		},
		{
			name: "onload via body",
			in:   `<body onload="steal()">text</body>`,
			bad:  []string{"onload", "steal()"},
		},
		{
			name: "css expression and behavior in style block",
			in:   `<style>.x{background:url(http://evil.example/a)} .y{behavior:url(script.htc)}</style>`,
			bad:  []string{"evil.example"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := content.SanitizeHTML([]byte(tc.in))
			s := strings.ToLower(string(out))
			for _, frag := range tc.bad {
				if strings.Contains(s, strings.ToLower(frag)) {
					t.Fatalf("hostile fragment %q survived: %q", frag, out)
				}
			}
		})
	}
}

func TestAdversarialCSS_NeutralisedPayloads(t *testing.T) {
	cases := []struct {
		name string
		in   string
		bad  []string
	}{
		{"import external", `@import "http://evil.example/x.css"; body{color:red}`, []string{"evil.example"}},
		{"import url external", `@import url(https://evil.example/x.css);`, []string{"evil.example"}},
		{"url with vbscript", `.a{background:url(vbscript:msgbox(1))}`, []string{"vbscript:"}},
		{"url with whitespace and case", `.a{background:URL(  HTTPS://Evil.Example/p  )}`, []string{"evil.example"}},
		{"protocol relative in url", `.a{background:url(//evil.example/p)}`, []string{"evil.example"}},
		{"multiple urls one bad one relative", `.a{background:url(local.png),url(http://evil.example/x)}`, []string{"evil.example"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _ := content.SanitizeCSS([]byte(tc.in))
			s := strings.ToLower(string(out))
			for _, frag := range tc.bad {
				if strings.Contains(s, strings.ToLower(frag)) {
					t.Fatalf("hostile fragment %q survived: %q", frag, out)
				}
			}
		})
	}
}

// Legitimate EPUB-typical markup is not over-stripped.
func TestSanitizeHTML_KeepsLegitimateMarkup(t *testing.T) {
	in := `<section epub:type="chapter"><h1>Chapter One</h1><p class="first">It was a <em>dark</em> and <strong>stormy</strong> night.</p>` +
		`<figure><img src="images/plate1.jpg" alt="A plate"/><figcaption>Plate 1</figcaption></figure>` +
		`<blockquote cite="somewhere">quoted</blockquote><a href="chapter2.xhtml#start">next</a></section>`
	out, rep := content.SanitizeHTML([]byte(in))
	s := string(out)

	for _, want := range []string{"Chapter One", "images/plate1.jpg", "chapter2.xhtml#start", "<em>dark</em>", "stormy", "Plate 1"} {
		if !strings.Contains(s, want) {
			t.Fatalf("legitimate content %q was stripped: %q", want, s)
		}
	}
	if rep.StrippedSomething() {
		t.Fatalf("nothing hostile was present but the report says something was stripped: %+v", rep)
	}
}
