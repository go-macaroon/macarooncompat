// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"fmt"
)

type Macaroon interface {
	MarshalJSON() ([]byte, error)
	MarshalBinary() ([]byte, error)
	WithFirstPartyCaveat(caveatId string) (Macaroon, error)
	WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error)
	Bind(primary Macaroon) (Macaroon, error)
	Verify(rootKey []byte, check Checker, discharges []Macaroon) error
	Signature() []byte
}

type Checker map[string]bool

func (c Checker) Check(cav string) error {
	if c[cav] {
		return nil
	}
	return fmt.Errorf("condition %q not met", cav)
}

type Package interface {
	UnmarshalJSON(data []byte) (Macaroon, error)
	UnmarshalBinary(data []byte) (Macaroon, error)
	New(rootKey []byte, id, loc string) (Macaroon, error)
}

type Implementation string

const (
	ImplGoV1          Implementation = "gov1"
	ImplGoV2          Implementation = "gov2"
	ImplLibMacaroons2 Implementation = "libmacaroons2"
	ImplJSMacaroon    Implementation = "jsmacaroon"
	ImplPyMacaroons2  Implementation = "pymacaroons2"
	ImplPyMacaroons3  Implementation = "pymacaroons3"
)

var Implementations = []struct {
	Name Implementation
	Pkg  Package
}{{
	Name: ImplGoV1,
	Pkg:  goMacaroonV1Package{},
}, {
	Name: ImplGoV2,
	Pkg:  goMacaroonV2Package{},
}, {
	Name: ImplLibMacaroons2,
	Pkg: libMacaroonsPkg{
		version: 2,
	},
}, {
	Name: ImplJSMacaroon,
	Pkg:  jsMacaroonPkg{},
}, {
	Name: ImplPyMacaroons2,
	Pkg: pyMacaroonsPkg{
		version: 2,
	},
}, {
	Name: ImplPyMacaroons3,
	Pkg: pyMacaroonsPkg{
		version: 3,
	},
}}

// TODO add libmacaroons python 3.
