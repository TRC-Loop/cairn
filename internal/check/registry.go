// SPDX-License-Identifier: AGPL-3.0-or-later

// Register pre-start only; reads are unsynchronized.
package check

type Registry struct {
	checkers map[Type]Checker
}

func NewRegistry() *Registry {
	return &Registry{checkers: make(map[Type]Checker)}
}

func (r *Registry) Register(t Type, c Checker) {
	r.checkers[t] = c
}

func (r *Registry) Get(t Type) (Checker, bool) {
	c, ok := r.checkers[t]
	return c, ok
}
