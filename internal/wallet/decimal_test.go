package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDecimal(t *testing.T) {
	sum, err := addDecimal("100", "50")
	require.NoError(t, err)
	assert.Equal(t, "150", sum)
}

func TestSubDecimal(t *testing.T) {
	diff, err := subDecimal("100", "50")
	require.NoError(t, err)
	assert.Equal(t, "50", diff)
}

func TestAddDecimalInvalidFirstOperand(t *testing.T) {
	_, err := addDecimal("not-a-number", "50")
	assert.Error(t, err)
}

func TestSubDecimalInvalidSecondOperand(t *testing.T) {
	_, err := subDecimal("100", "not-a-number")
	assert.Error(t, err)
}

func TestAddDecimalEmptyString(t *testing.T) {
	_, err := addDecimal("", "50")
	assert.Error(t, err)
}
