package common

import (
	"strings"
	"testing"
)

func TestTruncateTextToWidth(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth float64
		wantSame bool
	}{
		{name: "short text is unchanged", text: "Short", maxWidth: 400, wantSame: true},
		{name: "zero width is unconstrained", text: "Anything at all", maxWidth: 0, wantSame: true},
		{name: "long text is truncated", text: strings.Repeat("wide ", 30), maxWidth: 100},
		{name: "encoded apostrophe is measured decoded", text: "Octo&#39;s extremely long repository name that never ends", maxWidth: 120},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := TruncateTextToWidth(test.text, test.maxWidth, 13)
			if test.wantSame {
				if got != test.text {
					t.Errorf("got %q, want unchanged %q", got, test.text)
				}
				return
			}
			if !strings.HasSuffix(got, "...") {
				t.Errorf("got %q, want ellipsis suffix", got)
			}
			// The visible length must fit the requested width.
			visible := strings.TrimSuffix(got, "...")
			if MeasureText(visible, 13) > test.maxWidth {
				t.Errorf("visible part %q exceeds %v px", visible, test.maxWidth)
			}
		})
	}
}

func TestTruncateTextToWidthNeverSplitsRunes(t *testing.T) {
	text := strings.Repeat("数据", 40)
	got := TruncateTextToWidth(text, 40, 13)
	if strings.ContainsRune(got, '�') {
		t.Fatalf("truncation produced an invalid rune: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis, got %q", got)
	}
}

func TestSanitizeFontFamily(t *testing.T) {
	valid := []string{"Fira Code", "JetBrains Mono", "Segoe UI", "Noto_Sans", "Open-Sans 2.0", "思源黑体"}
	for _, name := range valid {
		if _, ok := SanitizeFontFamily(name); !ok {
			t.Errorf("expected %q to be accepted", name)
		}
	}

	invalid := []string{
		"Arial';fill:red;x:'",
		"<script>",
		"font; color: red",
		`"quoted"`,
		strings.Repeat("A", 65),
	}
	for _, name := range invalid {
		if _, ok := SanitizeFontFamily(name); ok {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}
