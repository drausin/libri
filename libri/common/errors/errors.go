package errors

// MaybePanic panics if the argument is not nil. It is useful for wrapping error-only return
// functions known to only return nil values.
func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}
