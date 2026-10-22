package admin

func testPointer[T any](value T) *T {
	return &value
}
