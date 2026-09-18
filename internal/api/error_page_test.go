package api

import (
	"strings"
	"testing"
)

func TestInjectBaseHref(t *testing.T) {
	html := `<html><head><title>x</title></head><body>hi</body></html>`
	got := injectBaseHref(html, "https://shop.example/catalog/product")
	if !strings.Contains(got, `<base href="https://shop.example/catalog/">`) {
		t.Fatalf("got %s", got)
	}

	plain := injectBaseHref("just text", "https://shop.example/")
	if !strings.HasPrefix(plain, "<head><base href=") {
		t.Fatalf("got %s", plain)
	}
}
