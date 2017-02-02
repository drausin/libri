package storage

var (
	// Server namespace contains values relevant to the server itself.
	Server Namespace = []byte("server")

	// Records namespace contains all p2p stored values.
	Records Namespace = []byte("records")
)

// Namespace denotes a storage namespace, which reduces to a key prefix.
type Namespace []byte

// Bytes returns the namespace byte encoding.
func (n Namespace) Bytes() []byte {
	return []byte(n)
}

// NamespaceStorer stores a value to durable storage under the configured namespace.
type NamespaceStorer interface {
	// Store a value for the key in the configured namespace.
	Store(key []byte, value []byte) error
}

// NamespaceLoader loads a value in the configured namespace from the durable storage.
type NamespaceLoader interface {
	// Load the value for the key in the configured namespace.
	Load(key []byte) ([]byte, error)
}

// NamespaceStorerLoader both stores and loads values in a configured namespace.
type NamespaceStorerLoader interface {
	NamespaceStorer
	NamespaceLoader
}

type namespaceStorerLoader struct {
	ns Namespace
	sl StorerLoader
}

func (nsl *namespaceStorerLoader) Store(key []byte, value []byte) error {
	return nsl.sl.Store(nsl.ns.Bytes(), key, value)
}

func (nsl *namespaceStorerLoader) Load(key []byte) ([]byte, error) {
	return nsl.sl.Load(nsl.ns.Bytes(), key)
}

// NewServerStorerLoader creates a new NamespaceStorerLoader for the "server" namespace.
func NewServerStorerLoader(sl StorerLoader) NamespaceStorerLoader {
	return &namespaceStorerLoader{ns: Server, sl: sl}
}

// NewRecordsStorerLoader creates a new NamespaceStorerLoader for the "records" namespace.
func NewRecordsStorerLoader(sl StorerLoader) NamespaceStorerLoader {
	return &namespaceStorerLoader{ns: Records, sl: sl}
}
