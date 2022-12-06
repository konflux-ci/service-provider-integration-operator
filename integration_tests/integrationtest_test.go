package integrationtests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type A struct {
	A int
	B string
}

func TestToPointerArray(t *testing.T) {
	arr := []A{{}, {A: 1, B: "2"}}

	ptrs := toPointerArray(arr)

	assert.Equal(t, 0, ptrs[0].A)
	assert.Equal(t, 1, ptrs[1].A)
	assert.Equal(t, "", ptrs[0].B)
	assert.Equal(t, "2", ptrs[1].B)
}

func TestFromPointerArray(t *testing.T) {
	ptrs := []*A{{}, {A: 1, B: "2"}}

	arr := fromPointerArray(ptrs)

	assert.Equal(t, 0, arr[0].A)
	assert.Equal(t, 1, arr[1].A)
	assert.Equal(t, "", arr[0].B)
	assert.Equal(t, "2", arr[1].B)
}
