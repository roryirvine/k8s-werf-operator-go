package values

import "testing"

func TestResolverInterface(t *testing.T) {
	// Verify interface is well-formed by attempting to declare a variable.
	// This ensures the interface compiles and can be satisfied by implementations.
	var _ Resolver = nil
}
