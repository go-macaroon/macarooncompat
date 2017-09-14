// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"gopkg.in/macaroon.v2-unstable"
)

type goMacaroonV2 struct {
	*macaroon.Macaroon
}

func (m goMacaroonV2) clone() goMacaroonV2 {
	return goMacaroonV2{m.Macaroon.Clone()}
}

func (m goMacaroonV2) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m = m.clone()
	if err := m.Macaroon.AddFirstPartyCaveat(caveatId); err != nil {
		return nil, err
	}
	return m, nil
}

func (m goMacaroonV2) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.clone()
	if err := m.Macaroon.AddThirdPartyCaveat(rootKey, []byte(caveatId), loc); err != nil {
		return nil, err
	}
	return m, nil
}

func (m goMacaroonV2) Bind(primary Macaroon) (Macaroon, error) {
	m = m.clone()
	m.Macaroon.Bind(primary.Signature())
	return m, nil
}

func (m goMacaroonV2) Verify(rootKey []byte, check Checker, discharges []Macaroon) error {
	discharges1 := make([]*macaroon.Macaroon, len(discharges))
	for i, m := range discharges {
		discharges1[i] = m.(goMacaroonV2).Macaroon
	}
	return m.Macaroon.Verify(rootKey, check.Check, discharges1)
}

type goMacaroonV2Package struct{}

func (goMacaroonV2Package) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m, err := macaroon.New(rootKey, []byte(id), loc, macaroon.V1)
	if err != nil {
		return nil, err
	}
	return goMacaroonV2{m}, nil
}

func (goMacaroonV2Package) UnmarshalJSON(data []byte) (Macaroon, error) {
	var m macaroon.Macaroon
	if err := m.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return goMacaroonV2{&m}, nil
}

func (goMacaroonV2Package) UnmarshalBinary(data []byte) (Macaroon, error) {
	var m macaroon.Macaroon
	if err := m.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return goMacaroonV2{&m}, nil
}
