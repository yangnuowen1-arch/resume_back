package handler

import "testing"

func TestParseOperationLogDate(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantDate bool
		wantErr  bool
	}{
		{name: "empty", value: ""},
		{name: "ISO date", value: "2026-06-20", wantDate: true},
		{name: "picker slash date", value: "2026/06/20", wantDate: true},
		{name: "invalid calendar date", value: "2026-02-30", wantErr: true},
		{name: "invalid format", value: "20-06-2026", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, err := parseOperationLogDate(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOperationLogDate(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if (date != nil) != tt.wantDate {
				t.Fatalf("parseOperationLogDate(%q) date = %v, wantDate %v", tt.value, date, tt.wantDate)
			}
			if date != nil && (date.Year() != 2026 || date.Month() != 6 || date.Day() != 20) {
				t.Fatalf("parseOperationLogDate(%q) = %v, want 2026-06-20", tt.value, date)
			}
		})
	}
}
