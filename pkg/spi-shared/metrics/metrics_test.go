package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_R1(t *testing.T) {
	recorderCalled := false

	r := NewValueTimer1[int](ValueObserverFunc1[int](func(d1 int, m float64) {
		recorderCalled = true
		assert.Equal(t, 42, d1)
		assert.True(t, m >= 0.02)
	}))

	fn := func() int {
		return 42
	}

	assert.False(t, recorderCalled)
	time.Sleep(20 * time.Millisecond)
	i := r.ObserveValuesAndDuration(fn())
	assert.True(t, recorderCalled)
	assert.Equal(t, 42, i)
}

func Test_R2(t *testing.T) {
	recorderCalled := false

	r := NewValueTimer2[int, error](ValueObserverFunc2[int, error](func(d1 int, d2 error, m float64) {
		recorderCalled = true
		assert.Equal(t, 42, d1)
		assert.Equal(t, "yay", d2.Error())
		assert.True(t, m >= 0.02)
	}))

	fn := func() (int, error) {
		return 42, errors.New("yay")
	}

	assert.False(t, recorderCalled)
	time.Sleep(20 * time.Millisecond)
	i, err := r.ObserveValuesAndDuration(fn())
	assert.True(t, recorderCalled)
	assert.Equal(t, 42, i)
	assert.Equal(t, "yay", err.Error())
}

func Test_R3(t *testing.T) {
	recorderCalled := false

	r := NewValueTimer3[int, int, error](ValueObserverFunc3[int, int, error](func(d1 int, d2 int, d3 error, m float64) {
		recorderCalled = true
		assert.Equal(t, 42, d1)
		assert.Equal(t, 43, d2)
		assert.Equal(t, "yay", d3.Error())
		assert.True(t, m >= 0.02)
	}))

	fn := func() (int, int, error) {
		return 42, 43, errors.New("yay")
	}

	assert.False(t, recorderCalled)
	time.Sleep(20 * time.Millisecond)
	i, j, err := r.ObserveValuesAndDuration(fn())
	assert.True(t, recorderCalled)
	assert.Equal(t, 42, i)
	assert.Equal(t, 43, j)
	assert.Equal(t, "yay", err.Error())
}

func Test_R4(t *testing.T) {
	recorderCalled := false

	r := NewValueTimer4[int, int, string, error](ValueObserverFunc4[int, int, string, error](func(d1 int, d2 int, d3 string, d4 error, m float64) {
		recorderCalled = true
		assert.Equal(t, 42, d1)
		assert.Equal(t, 43, d2)
		assert.Equal(t, "kachny", d3)
		assert.Equal(t, "yay", d4.Error())
		assert.True(t, m >= 0.02)
	}))

	fn := func() (int, int, string, error) {
		return 42, 43, "kachny", errors.New("yay")
	}

	assert.False(t, recorderCalled)
	time.Sleep(20 * time.Millisecond)
	i, j, k, err := r.ObserveValuesAndDuration(fn())
	assert.True(t, recorderCalled)
	assert.Equal(t, 42, i)
	assert.Equal(t, 43, j)
	assert.Equal(t, "kachny", k)
	assert.Equal(t, "yay", err.Error())
}

func Test_R5(t *testing.T) {
	recorderCalled := false

	r := NewValueTimer5[int, int, string, *bool, error](ValueObserverFunc5[int, int, string, *bool, error](func(d1 int, d2 int, d3 string, d4 *bool, d5 error, m float64) {
		recorderCalled = true
		assert.Equal(t, 42, d1)
		assert.Equal(t, 43, d2)
		assert.Equal(t, "kachny", d3)
		assert.Equal(t, true, *d4)
		assert.Equal(t, "yay", d5.Error())
		assert.True(t, m >= 0.02)
	}))

	fn := func() (int, int, string, *bool, error) {
		b := true
		return 42, 43, "kachny", &b, errors.New("yay")
	}

	assert.False(t, recorderCalled)
	time.Sleep(20 * time.Millisecond)
	i, j, k, l, err := r.ObserveValuesAndDuration(fn())
	assert.True(t, recorderCalled)
	assert.Equal(t, 42, i)
	assert.Equal(t, 43, j)
	assert.Equal(t, "kachny", k)
	assert.True(t, *l)
	assert.Equal(t, "yay", err.Error())
}
