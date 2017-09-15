// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"gopkg.in/macaroon.v1"
)

type goMacaroonV1 struct {
	*macaroon.Macaroon
}

func (m goMacaroonV1) clone() goMacaroonV1 {
	return goMacaroonV1{m.Macaroon.Clone()}
}

func (m goMacaroonV1) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m = m.clone()
	if err := m.Macaroon.AddFirstPartyCaveat(caveatId); err != nil {
		return nil, err
	}
	return m, nil
}

func (m goMacaroonV1) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.clone()
	if err := m.Macaroon.AddThirdPartyCaveat(rootKey, caveatId, loc); err != nil {
		return nil, err
	}
	return m, nil
}

func (m goMacaroonV1) Bind(primary Macaroon) (Macaroon, error) {
	m = m.clone()
	m.Macaroon.Bind(primary.Signature())
	return m, nil
}

func (m goMacaroonV1) Verify(rootKey []byte, check Checker, discharges []Macaroon) error {
	discharges1 := make([]*macaroon.Macaroon, len(discharges))
	for i, m := range discharges {
		discharges1[i] = m.(goMacaroonV1).Macaroon
	}
	return m.Macaroon.Verify(rootKey, check.Check, discharges1)
}

type goMacaroonV1Package struct{}

func (goMacaroonV1Package) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m, err := macaroon.New(rootKey, id, loc)
	if err != nil {
		return nil, err
	}
	return goMacaroonV1{m}, nil
}

func (goMacaroonV1Package) UnmarshalJSON(data []byte) (Macaroon, error) {
	var m macaroon.Macaroon
	if err := m.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return goMacaroonV1{&m}, nil
}

func (goMacaroonV1Package) UnmarshalBinary(data []byte) (Macaroon, error) {
	var m macaroon.Macaroon
	if err := m.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return goMacaroonV1{&m}, nil
}
