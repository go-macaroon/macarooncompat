// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

type Macaroon interface {
	MarshalJSON() ([]byte, error)
	MarshalBinary() ([]byte, error)
	WithFirstPartyCaveat(caveatId string) (Macaroon, error)
	WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error)
	Bind(discharge Macaroon) (Macaroon, error)
	Verify(rootKey []byte, check func(caveat string) error, discharges []Macaroon) error
	Signature() []byte
}

type Package interface {
	UnmarshalJSON(data []byte) (Macaroon, error)
	UnmarshalBinary(data []byte) (Macaroon, error)
	New(rootKey []byte, id, loc string) (Macaroon, error)
}

var Implementations = []struct {
	Name string
	Pkg  Package
}{{
	Name: "go",
	Pkg:  goMacaroonPackage{},
}, {
	Name: "libmacaroons",
	Pkg:  libMacaroonPkg{},
}}
