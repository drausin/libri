package api

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestEndpoint_String(t *testing.T) {
	cases := map[string]struct {
		value    Endpoint
		expected string
	}{
		"all":       {value: All, expected: "All"},
		"intro":     {value: Introduce, expected: "Introduce"},
		"find":      {value: Find, expected: "Find"},
		"store":     {value: Store, expected: "Store"},
		"verify":    {value: Verify, expected: "Verify"},
		"get":       {value: Get, expected: "Get"},
		"put":       {value: Put, expected: "Put"},
		"subscribe": {value: Subscribe, expected: "Subscribe"},
	}
	for desc, c := range cases {
		assert.Equal(t, c.expected, c.value.String(), desc)
	}
}
