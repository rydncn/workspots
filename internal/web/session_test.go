package web

import "testing"

// A return path is used only if it is one of our own paths. Browsers strip
// tabs and newlines from URLs, so "/\t/evil.example" becomes "//evil.example".
func TestSafeNext(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/account":               "/account",
		"/spots/12?x=1":          "/spots/12?x=1",
		"/\t/evil.example":       "/",
		"//evil.example":         "/",
		"/\\evil.example":        "/",
		"https://evil.example/x": "/",
		"/account\r\nX: y":       "/",
		"/\x7fthing":             "/",
		"javascript:alert(1)":    "/",
		"account":                "/",
		"":                       "/",
		"/%09/evil.example":      "/",
	}
	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
