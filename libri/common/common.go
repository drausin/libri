package common

// MaybePanic panics if the error is nil. This function is mostly used in wrapping defer statements.
func MaybePanic(err error) {
	if err != nil {
		panic(err)
	}
}
