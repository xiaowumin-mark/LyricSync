package ui

import "testing"

func TestParseDurationInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "seconds", input: "225", want: 225000},
		{name: "milliseconds", input: "225000", want: 225000},
		{name: "minutes seconds", input: "03:45", want: 225000},
		{name: "hours minutes seconds", input: "1:02:03", want: 3723000},
		{name: "invalid", input: "3:xx", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("parseDurationInput(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLyricDelayInput(t *testing.T) {
	got, err := parseLyricDelayInput("-320")
	if err != nil {
		t.Fatal(err)
	}
	if got != -320 {
		t.Fatalf("delay = %d, want -320", got)
	}
	got, err = parseLyricDelayInput("")
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("empty delay = %d, want 0", got)
	}
	if _, err := parseLyricDelayInput("1.2"); err == nil {
		t.Fatal("expected invalid delay to fail")
	}
}

func TestLyricPreviewTextSkipsEmptyAndPlaceholderTranslation(t *testing.T) {
	got := lyricPreviewText("\n//\n[00:01.00]第一行\n[00:02.00]第二行\n")
	want := "第一行\n第二行"
	if got != want {
		t.Fatalf("preview = %q, want %q", got, want)
	}
}
