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
	Bind(discharge Macaroon) (Macaroon, error)
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
	ImplGo           Implementation = "go"
	ImplLibMacaroons Implementation = "libmacaroons"
	ImplJSMacaroon   Implementation = "jsmacaroon"
)

var Implementations = []struct {
	Name Implementation
	Pkg  Package
}{{
	Name: ImplGo,
	Pkg:  goMacaroonPackage{},
}, {
	Name: ImplLibMacaroons,
	Pkg:  libMacaroonPkg{},
}, {
	Name: ImplJSMacaroon,
	Pkg:  jsMacaroonPkg{},
}}
