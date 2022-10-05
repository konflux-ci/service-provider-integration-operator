//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import "time"

// ValueObserver1 is a callback interface that is called with a piece of data and a metric value. This can be used to
// record durations of different metrics based on the supplied value.
type ValueObserver1[T any] interface {
	Observe(val T, metric float64)
}

// ValueObserver2 is a callback interface that is called with a piece of data and a metric value. This can be used to
// record durations of different metrics based on the supplied values.
type ValueObserver2[T any, U any] interface {
	Observe(val1 T, val2 U, metric float64)
}

// ValueObserver3 is a callback interface that is called with a piece of data and a metric value. This can be used to
// record durations of different metrics based on the supplied values.
type ValueObserver3[T any, U any, V any] interface {
	Observe(val1 T, val2 U, val3 V, metric float64)
}

// ValueObserver4 is a callback interface that is called with a piece of data and a metric value. This can be used to
// record durations of different metrics based on the supplied values.
type ValueObserver4[T any, U any, V any, W any] interface {
	Observe(val1 T, val2 U, val3 V, val4 W, metric float64)
}

// ValueObserver5 is a callback interface that is called with a piece of data and a metric value. This can be used to
// record durations of different metrics based on the supplied values.
type ValueObserver5[T any, U any, V any, W any, X any] interface {
	Observe(val1 T, val2 U, val3 V, val4 W, val5 X, metric float64)
}

// ValueObserverFunc1 is a functional implementation of ValueObserver1 interface.
type ValueObserverFunc1[T any] func(val T, metric float64)

// ValueObserverFunc2 is a functional implementation of ValueObserver2 interface.
type ValueObserverFunc2[T any, U any] func(val1 T, val2 U, metric float64)

// ValueObserverFunc3 is a functional implementation of ValueObserver3 interface.
type ValueObserverFunc3[T any, U any, V any] func(val1 T, val2 U, val3 V, metric float64)

// ValueObserverFunc4 is a functional implementation of ValueObserver4 interface.
type ValueObserverFunc4[T any, U any, V any, W any] func(val1 T, val2 U, val3 V, val4 W, metric float64)

// ValueObserverFunc5 is a functional implementation of ValueObserver5 interface.
type ValueObserverFunc5[T any, U any, V any, W any, X any] func(val1 T, val2 U, val3 V, val4 W, val5 X, metric float64)

func (r ValueObserverFunc1[T]) Observe(v T, metric float64) {
	r(v, metric)
}

func (r ValueObserverFunc2[T, U]) Observe(v1 T, v2 U, metric float64) {
	r(v1, v2, metric)
}

func (r ValueObserverFunc3[T, U, V]) Observe(v1 T, v2 U, v3 V, metric float64) {
	r(v1, v2, v3, metric)
}

func (r ValueObserverFunc4[T, U, V, W]) Observe(v1 T, v2 U, v3 V, v4 W, metric float64) {
	r(v1, v2, v3, v4, metric)
}

func (r ValueObserverFunc5[T, U, V, W, X]) Observe(v1 T, v2 U, v3 V, v4 W, v5 X, metric float64) {
	r(v1, v2, v3, v4, v5, metric)
}

// Trying out the implementation is possible only using some concrete generics instantiation
var _ ValueObserver1[int] = (ValueObserverFunc1[int])(nil)
var _ ValueObserver2[int, int] = (ValueObserverFunc2[int, int])(nil)
var _ ValueObserver3[int, int, int] = (ValueObserverFunc3[int, int, int])(nil)
var _ ValueObserver4[int, int, int, int] = (ValueObserverFunc4[int, int, int, int])(nil)
var _ ValueObserver5[int, int, int, int, int] = (ValueObserverFunc5[int, int, int, int, int])(nil)

// ValueTimer1 is a timer that calls the supplied recorder function. When the ObserveValuesAndDuration method is called the supplied
// recorder function is called with the provided data and the time it took since the instantiation of
// the RecordingTimer.
//
// It is similar to prometheus.Timer but unlike it, this can also process the supplied data. This means though that it
// is not possible to defer the execution of the RecordingTimer rather it is expected that is called right before
// the return from a function. See ObserveValuesAndDuration for more details.
type ValueTimer1[T any] struct {
	Observer  ValueObserver1[T]
	startTime time.Time
}

// ValueTimer2 is similar to ValueTimer1 but can be used with functions returning two values.
type ValueTimer2[T any, U any] struct {
	Recorder  ValueObserver2[T, U]
	startTime time.Time
}

// ValueTimer3 is similar to ValueTimer1 but can be used with functions returning three values.
type ValueTimer3[T any, U any, V any] struct {
	Recorder  ValueObserver3[T, U, V]
	startTime time.Time
}

// ValueTimer4 is similar to ValueTimer1 but can be used with functions returning four values.
type ValueTimer4[T any, U any, V any, W any] struct {
	Recorder  ValueObserver4[T, U, V, W]
	startTime time.Time
}

// ValueTimer5 is similar to ValueTimer1 but can be used with functions returning five values.
type ValueTimer5[T any, U any, V any, W any, X any] struct {
	Recorder  ValueObserver5[T, U, V, W, X]
	startTime time.Time
}

func NewValueTimer1[T any](recorder ValueObserver1[T]) ValueTimer1[T] {
	return ValueTimer1[T]{
		Observer:  recorder,
		startTime: time.Now(),
	}
}

func NewValueTimer2[T any, U any](recorder ValueObserver2[T, U]) ValueTimer2[T, U] {
	return ValueTimer2[T, U]{
		Recorder:  recorder,
		startTime: time.Now(),
	}
}

func NewValueTimer3[T any, U any, V any](recorder ValueObserver3[T, U, V]) ValueTimer3[T, U, V] {
	return ValueTimer3[T, U, V]{
		Recorder:  recorder,
		startTime: time.Now(),
	}
}

func NewValueTimer4[T any, U any, V any, W any](recorder ValueObserver4[T, U, V, W]) ValueTimer4[T, U, V, W] {
	return ValueTimer4[T, U, V, W]{
		Recorder:  recorder,
		startTime: time.Now(),
	}
}

func NewValueTimer5[T any, U any, V any, W any, X any](recorder ValueObserver5[T, U, V, W, X]) ValueTimer5[T, U, V, W, X] {
	return ValueTimer5[T, U, V, W, X]{
		Recorder:  recorder,
		startTime: time.Now(),
	}
}

// ObserveValuesAndDuration calls the stored observer with the supplied data and the time it took since the instantiation of
// the ValueTimer1 since this call. It also returns the supplied data.
func (o ValueTimer1[T]) ObserveValuesAndDuration(val T) T {
	o.Observer.Observe(val, time.Now().Sub(o.startTime).Seconds())
	return val
}

// ObserveValuesAndDuration calls the stored observer with the supplied data and the time it took since the instantiation of
// the ValueTimer2 since this call. It also returns the supplied data.
func (o ValueTimer2[T, U]) ObserveValuesAndDuration(v1 T, v2 U) (T, U) {
	o.Recorder.Observe(v1, v2, time.Now().Sub(o.startTime).Seconds())
	return v1, v2
}

// ObserveValuesAndDuration calls the stored observer with the supplied data and the time it took since the instantiation of
// the ValueTimer3 since this call. It also returns the supplied data.
func (o ValueTimer3[T, U, V]) ObserveValuesAndDuration(v1 T, v2 U, v3 V) (T, U, V) {
	o.Recorder.Observe(v1, v2, v3, time.Now().Sub(o.startTime).Seconds())
	return v1, v2, v3
}

// ObserveValuesAndDuration calls the stored observer with the supplied data and the time it took since the instantiation of
// the ValueTimer4 since this call. It also returns the supplied data.
func (o ValueTimer4[T, U, V, W]) ObserveValuesAndDuration(v1 T, v2 U, v3 V, v4 W) (T, U, V, W) {
	o.Recorder.Observe(v1, v2, v3, v4, time.Now().Sub(o.startTime).Seconds())
	return v1, v2, v3, v4
}

// ObserveValuesAndDuration calls the stored observer with the supplied data and the time it took since the instantiation of
// the ValueTimer5 since this call. It also returns the supplied data.
func (o ValueTimer5[T, U, V, W, X]) ObserveValuesAndDuration(v1 T, v2 U, v3 V, v4 W, v5 X) (T, U, V, W, X) {
	o.Recorder.Observe(v1, v2, v3, v4, v5, time.Now().Sub(o.startTime).Seconds())
	return v1, v2, v3, v4, v5
}
