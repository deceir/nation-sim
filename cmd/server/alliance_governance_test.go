package main

import (
	"encoding/json"
	"testing"
)

func TestTaxBracketInputAcceptsNumericAndFormEncodedRates(t *testing.T) {
	tests := []string{
		`{"name":"Standard","cashRate":12.5,"resourceRate":7}`,
		`{"name":"Standard","cashRate":"12.5","resourceRate":"7"}`,
	}
	for _, body := range tests {
		var input taxBracketInput
		if err := json.Unmarshal([]byte(body), &input); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
		if !validBracket(input) || float64(input.CashRate) != 12.5 || float64(input.ResourceRate) != 7 {
			t.Fatalf("unexpected bracket decoded from %s: %+v", body, input)
		}
	}
}
