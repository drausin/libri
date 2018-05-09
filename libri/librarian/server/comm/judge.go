package comm

// Judge determines which peers should be favored over others.
type Judge interface {
	Preferer
	Limiter
	Doctor
}
