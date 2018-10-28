package controller

import (
	"github.com/thoth-station/solver-operator/pkg/controller/solver"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, solver.Add)
}
