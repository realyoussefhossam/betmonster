package rates

import "testing"

func TestIsSupportedFiat(t *testing.T) {
	if !IsSupportedFiat("USD") {
		t.Fatal("USD should be supported")
	}
	if !IsSupportedFiat("usd") {
		t.Fatal("lowercase usd should be supported")
	}
	if IsSupportedFiat("XYZ") {
		t.Fatal("XYZ should not be supported")
	}
}
