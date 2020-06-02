package controller

import (
	"github.com/pwegrzyn/kubernetes-operators-project/pkg/controller/h2database"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, h2database.Add)
}
