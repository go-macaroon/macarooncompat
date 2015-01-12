// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"github.com/rescrv/libmacaroons/bindings/go/macaroons"
)

type libMacaroon struct {
	*macaroons.Macaroon
}

func (m libMacaroon) Clone() libMacaroon {
	newm, err := m.Macaroon.Copy()
	if err != nil {
		panic(err)
	}
	return libMacaroon{newm}
}

func (m libMacaroon) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m = m.Clone()
	if err := m.Macaroon.WithFirstPartyCaveat(caveatId); err != nil {
		return nil, err
	}
	return m, nil
}

func (m libMacaroon) Signature() []byte {
	return []byte(m.Macaroon.Signature())
}

func (m libMacaroon) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.Clone()
	if err := m.Macaroon.WithThirdPartyCaveat(loc, string(rootKey), caveatId); err != nil {
		return nil, err
	}
	return m, nil
}

func (m libMacaroon) Bind(discharge Macaroon) (Macaroon, error) {
	m1, err := m.PrepareForRequest(discharge.(libMacaroon).Macaroon)
	if err != nil {
		return nil, err
	}
	return libMacaroon{m1}, nil
}

func (m libMacaroon) Verify(rootKey []byte, check func(caveat string) error, discharges []Macaroon) error {
	discharges1 := make([]*macaroons.Macaroon, len(discharges))
	for i, m := range discharges {
		discharges1[i] = m.(libMacaroon).Macaroon
	}
	v := macaroons.NewVerifier()
	if err := v.SatisfyGeneral(func(caveat string) bool {
		return check(caveat) == nil
	}); err != nil {
		return err
	}
	return v.Verify(m.Macaroon, string(rootKey), discharges1...)
}

type libMacaroonPkg struct{}

func (libMacaroonPkg) UnmarshalJSON(data []byte) (Macaroon, error) {
	var m macaroons.Macaroon
	if err := m.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return libMacaroon{&m}, nil
}

func (libMacaroonPkg) UnmarshalBinary(data []byte) (Macaroon, error) {
	var m macaroons.Macaroon
	if err := m.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return libMacaroon{&m}, nil
}

func (libMacaroonPkg) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m, err := macaroons.NewMacaroon(loc, string(rootKey), id)
	if err != nil {
		return nil, err
	}
	return libMacaroon{m}, nil
}
