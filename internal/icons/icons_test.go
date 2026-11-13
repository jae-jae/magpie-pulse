package icons

import "testing"

func TestProviderIconsAndTray(t *testing.T) {
	for _, provider := range []string{"antigravity", "claude", "codex", "gemini-web", "ai-studio", "zcode", "unknown-provider"} {
		svg := ForProvider(provider)
		if svg == nil {
			t.Fatalf("ForProvider(%q) returned nil", provider)
		}
		w, h := svg.Size()
		if w <= 0 || h <= 0 {
			t.Fatalf("ForProvider(%q) invalid size: %v x %v", provider, w, h)
		}
	}
	if len(TrayIconPNG(false)) == 0 || len(TrayIconPNG(true)) == 0 {
		t.Fatal("TrayIconPNG returned empty bytes")
	}
	if len(AppIconPNG()) == 0 {
		t.Fatal("AppIconPNG returned empty bytes")
	}
}
