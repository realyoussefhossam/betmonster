package rates

import "testing"

func TestMulDecimalStrings(t *testing.T) {
	got, err := MulDecimalStrings("100.00", "1.05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "105.00" {
		t.Fatalf("expected 105.00, got %s", got)
	}
}

func TestMulDecimalStrings_Stablecoin(t *testing.T) {
	got, err := MulDecimalStrings("50.00000000", "1.00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "50.00000000" {
		t.Fatalf("expected 50.00000000, got %s", got)
	}
}
