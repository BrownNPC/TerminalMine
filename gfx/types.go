package gfx

type Vector2 struct {
	X, Y int
}

// new vector 2
func Vec2(x, y int) Vector2 {
	return Vector2{x, y}
}

type RingBuffer[T any] struct {
	data        []T
	size        int
	read, write int
	full        bool
}

func NewRingBuffer[T any](capacity int) RingBuffer[T] {
	return RingBuffer[T]{
		data: make([]T, capacity),
		size: capacity,
	}
}
func (r *RingBuffer[T]) Append(value T) {
	r.data[r.write] = value
	if r.full {
		r.read = (r.read + 1) % r.size // Overwrite oldest
	}
	r.write = (r.write + 1) % r.size
	r.full = r.write == r.read
}
func (r *RingBuffer[T]) Pop() (value T, ok bool) {
	if r.Empty() {
		return value, false
	}
	val := r.data[r.read]
	r.full = false
	r.read = (r.read + 1) % r.size
	return val, true
}

func (r *RingBuffer[T]) Empty() bool {
	return !r.full && r.read == r.write
}

func (r *RingBuffer[T]) Full() bool {
	return r.full
}

func (r *RingBuffer[T]) Len() int {
	if r.full {
		return r.size
	}
	if r.write >= r.read {
		return r.write - r.read
	}
	return r.size - r.read + r.write
}
