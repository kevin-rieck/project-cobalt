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
		{name: "SByte min", dataType: "SByte", target: "-128", want: int8(-128), normalized: "-128"},
		{name: "Int16", dataType: "Int16", target: "-42", want: int16(-42), normalized: "-42"},
		{name: "Int32", dataType: "Int32", target: "2147483647", want: int32(2147483647), normalized: "2147483647"},
		{name: "Int64", dataType: "Int64", target: "-9223372036854775808", want: int64(-9223372036854775808), normalized: "-9223372036854775808"},
		{name: "Byte max", dataType: "Byte", target: "255", want: byte(255), normalized: "255"},
		{name: "UInt16", dataType: "UInt16", target: "65535", want: uint16(65535), normalized: "65535"},
		{name: "UInt32", dataType: "UInt32", target: "4294967295", want: uint32(4294967295), normalized: "4294967295"},
		{name: "UInt64", dataType: "UInt64", target: "18446744073709551615", want: uint64(18446744073709551615), normalized: "18446744073709551615"},
		{name: "Float exponent", dataType: "Float", target: "1e-3", want: float32(0.001), normalized: "0.001"},
		{name: "Double", dataType: "Double", target: "3.14", want: float64(3.14), normalized: "3.14"},
		{name: "String preserves whitespace", dataType: "String", target: "  Pump A  ", want: "  Pump A  ", normalized: "  Pump A  "},
		{name: "String empty", dataType: "String", target: "", want: "", normalized: ""},
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
		{dataType: "Float", target: "0x1p2"},
		{dataType: "Double", target: "1_000"},
	}
	for _, tt := range tests {
		if _, err := ParseScalarValue(tt.dataType, tt.target); err == nil {
			t.Fatalf("ParseScalarValue(%q, %q) error = nil, want rejection", tt.dataType, tt.target)
		}
	}
}
