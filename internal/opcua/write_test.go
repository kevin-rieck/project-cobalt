package opcua

import "testing"

func TestParseScalarValueUsesSupportedTypeRules(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		target     string
		want       any
		normalized string
	}{
		{name: "Boolean yes", dataType: "Boolean", target: "yes", want: true, normalized: "true"},
		{name: "Byte max", dataType: "Byte", target: "255", want: byte(255), normalized: "255"},
		{name: "Int16", dataType: "Int16", target: "-42", want: int16(-42), normalized: "-42"},
		{name: "Float exponent", dataType: "Float", target: "1e-3", want: float32(0.001), normalized: "0.001"},
		{name: "String preserves whitespace", dataType: "String", target: "  Pump A  ", want: "  Pump A  ", normalized: "  Pump A  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScalarValue(tt.dataType, tt.target)
			if err != nil {
				t.Fatalf("ParseScalarValue() error = %v", err)
			}
			if got.Value != tt.want || got.Normalized != tt.normalized {
				t.Fatalf("ParseScalarValue() = %#v, want value %#v normalized %q", got, tt.want, tt.normalized)
			}
		})
	}
}

func TestParseScalarValueRejectsUnsupportedAndInvalidValues(t *testing.T) {
	tests := []struct {
		dataType string
		target   string
	}{
		{dataType: "DateTime", target: "2026-01-01T00:00:00Z"},
		{dataType: "Int32", target: "0x2A"},
		{dataType: "Int32", target: "1_000"},
		{dataType: "Int32", target: "1.0"},
		{dataType: "Byte", target: "256"},
	}
	for _, tt := range tests {
		if _, err := ParseScalarValue(tt.dataType, tt.target); err == nil {
			t.Fatalf("ParseScalarValue(%q, %q) error = nil, want rejection", tt.dataType, tt.target)
		}
	}
}
