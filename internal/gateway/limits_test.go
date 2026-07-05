package gateway

import (
	"testing"
)

func TestLimits_ValidateWithdrawal_AllowsValidAmount(t *testing.T) {
	l := Limits{
		MinWithdrawal: "10",
		MaxWithdrawal: "1000",
	}
	if err := l.ValidateWithdrawal("100"); err != nil {
		t.Fatalf("expected withdrawal of 100 to be valid, got error: %v", err)
	}
}

func TestLimits_ValidateWithdrawal_RejectsBelowMinimum(t *testing.T) {
	l := Limits{
		MinWithdrawal: "10",
		MaxWithdrawal: "1000",
	}
	err := l.ValidateWithdrawal("5")
	if err == nil {
		t.Fatal("expected error for withdrawal below minimum")
	}
	if err.Error() != "withdrawal below minimum" {
		t.Fatalf("expected 'withdrawal below minimum', got: %v", err)
	}
}

func TestLimits_ValidateWithdrawal_RejectsAboveMaximum(t *testing.T) {
	l := Limits{
		MinWithdrawal: "10",
		MaxWithdrawal: "1000",
	}
	err := l.ValidateWithdrawal("2000")
	if err == nil {
		t.Fatal("expected error for withdrawal above maximum")
	}
	if err.Error() != "withdrawal above maximum" {
		t.Fatalf("expected 'withdrawal above maximum', got: %v", err)
	}
}

func TestLimits_ValidateWithdrawal_RespectsEmptyLimits(t *testing.T) {
	l := Limits{}
	if err := l.ValidateWithdrawal("1000000"); err != nil {
		t.Fatalf("expected no limits to allow any amount, got error: %v", err)
	}
}

func TestLimits_ValidateDeposit_AllowsValidAmount(t *testing.T) {
	l := Limits{
		MinDeposit: "5",
		MaxDeposit: "10000",
	}
	if err := l.ValidateDeposit("500"); err != nil {
		t.Fatalf("expected deposit of 500 to be valid, got error: %v", err)
	}
}

func TestLimits_ValidateDeposit_RejectsBelowMinimum(t *testing.T) {
	l := Limits{
		MinDeposit: "5",
		MaxDeposit: "10000",
	}
	err := l.ValidateDeposit("1")
	if err == nil {
		t.Fatal("expected error for deposit below minimum")
	}
	if err.Error() != "deposit below minimum" {
		t.Fatalf("expected 'deposit below minimum', got: %v", err)
	}
}

func TestLimits_ValidateDeposit_RejectsAboveMaximum(t *testing.T) {
	l := Limits{
		MinDeposit: "5",
		MaxDeposit: "10000",
	}
	err := l.ValidateDeposit("20000")
	if err == nil {
		t.Fatal("expected error for deposit above maximum")
	}
	if err.Error() != "deposit above maximum" {
		t.Fatalf("expected 'deposit above maximum', got: %v", err)
	}
}

func TestLimits_ValidateDeposit_RespectsEmptyLimits(t *testing.T) {
	l := Limits{}
	if err := l.ValidateDeposit("1000000"); err != nil {
		t.Fatalf("expected no limits to allow any amount, got error: %v", err)
	}
}

func TestLimits_ValidateWithdrawal_RejectsInvalidAmount(t *testing.T) {
	l := Limits{
		MinWithdrawal: "10",
		MaxWithdrawal: "1000",
	}
	if err := l.ValidateWithdrawal("not-a-number"); err == nil {
		t.Fatal("expected error for invalid amount")
	}
}

func TestLimits_ValidateDeposit_RejectsInvalidAmount(t *testing.T) {
	l := Limits{
		MinDeposit: "5",
		MaxDeposit: "10000",
	}
	if err := l.ValidateDeposit("not-a-number"); err == nil {
		t.Fatal("expected error for invalid amount")
	}
}
