package controllers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregatedError(t *testing.T) {
	e := NewAggregatedError(errors.New("a"), errors.New("b"), errors.New("c"))
	assert.Equal(t, "a, b, c", e.Error())
}
