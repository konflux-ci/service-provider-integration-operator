package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissions(t *testing.T) {
	var pt PermissionType

	pt = PermissionTypeRead
	assert.True(t, pt.IsRead())
	assert.False(t, pt.IsWrite())

	pt = PermissionTypeReadWrite
	assert.True(t, pt.IsRead())
	assert.True(t, pt.IsWrite())

	pt = PermissionTypeWrite
	assert.False(t, pt.IsRead())
	assert.True(t, pt.IsWrite())
}
