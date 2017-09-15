// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	errgo "gopkg.in/errgo.v1"
)

var libMacaroonsRunner = []*libMacaroonsInterp{
	2: newLibMacaroonsInterp(2),
	3: newLibMacaroonsInterp(3),
}

type libMacaroonsPkg struct {
	version int
}

func (p libMacaroonsPkg) eval(expr string, result interface{}) error {
	return libMacaroonsRunner[p.version].eval(expr, result)
}

func (p libMacaroonsPkg) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m := p.newMacaroon()
	expr := fmt.Sprintf(`%s = macaroons.create(%s, %s, %s)`,
		m.name, pyVal(loc), pyVal(rootKey), pyVal(id))
	if err := p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (p libMacaroonsPkg) newMacaroon() *libMacaroon {
	return &libMacaroon{
		p:    p,
		name: newPyName("m"),
	}
}

func (p libMacaroonsPkg) UnmarshalJSON(data []byte) (Macaroon, error) {
	m := p.newMacaroon()
	expr := fmt.Sprintf(`%s = macaroons.deserialize(%s)`, m.name, pyVal(base64.StdEncoding.EncodeToString(data)))
	if err := p.eval(expr, nil); err != nil {
		return m, err
	}
	return m, nil
}

func (p libMacaroonsPkg) UnmarshalBinary(data []byte) (Macaroon, error) {
	return nil, fmt.Errorf("unimplemented")
}

type libMacaroon struct {
	p    libMacaroonsPkg
	name string
}

func (m *libMacaroon) MarshalJSON() ([]byte, error) {
	expr := fmt.Sprintf(`result = %s.serialize(format='2j')`, m.name)
	var r string
	if err := m.p.eval(expr, &r); err != nil {
		return nil, err
	}
	return []byte(r), nil
}

func (m *libMacaroon) MarshalBinary() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (m *libMacaroon) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m1 := m.p.newMacaroon()
	expr := fmt.Sprintf(`%s = %s.add_first_party_caveat(%s)`,
		m1.name, m.name, pyVal(caveatId))
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m1, nil
}

func (m *libMacaroon) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m1 := m.p.newMacaroon()
	// TODO specify nonce explicitly.
	expr := fmt.Sprintf(`%s = %s.add_third_party_caveat(%s, %s, %s)`,
		m1.name, m.name, pyVal(loc), pyVal(rootKey), pyVal(caveatId))
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m1, nil
}

func (m *libMacaroon) Bind(primary Macaroon) (Macaroon, error) {
	pm := primary.(*libMacaroon)
	boundM := m.p.newMacaroon()
	expr := fmt.Sprintf(`%s = %s.prepare_for_request(%s)`, boundM.name, pm.name, m.name)
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return boundM, nil
}

func (m *libMacaroon) Verify(rootKey []byte, check Checker, discharges []Macaroon) error {
	dischargeNames := make([]string, len(discharges))
	for i, m := range discharges {
		dischargeNames[i] = m.(*libMacaroon).name
	}
	checkData, _ := json.Marshal(check)
	expr := fmt.Sprintf(`bool_verifier(json.loads(%s)).verify(%s, %s, [%s])`, pyVal(string(checkData)), m.name, pyVal(rootKey), strings.Join(dischargeNames, ", "))
	return m.p.eval(expr, nil)
}

func (m *libMacaroon) Signature() []byte {
	expr := fmt.Sprintf(`result = %s.signature`, m.name)
	var r string
	err := m.p.eval(expr, &r)
	if err != nil {
		panic(fmt.Errorf("cannot get signature: %v", err))
	}
	data, err := hex.DecodeString(r)
	if err != nil {
		panic(fmt.Errorf("cannot decode base64 signature: %v", err))
	}
	return data
}

type libMacaroonsInterp struct {
	interp *pyInterp
}

func newLibMacaroonsInterp(version int) *libMacaroonsInterp {
	return &libMacaroonsInterp{
		interp: newPyInterp(version),
	}
}

func (i *libMacaroonsInterp) eval(expr string, resultVal interface{}) error {
	if err := i.start(); err != nil {
		return fmt.Errorf("cannot start pyinterp: %v", err)
	}
	return i.interp.eval(expr, resultVal)
}

func (i *libMacaroonsInterp) start() error {
	if i.interp.started() {
		return nil
	}
	if err := i.interp.start(); err != nil {
		return errgo.Mask(err)
	}
	for _, p := range []string{"macaroons", "base64", "json"} {
		if err := i.interp.eval(fmt.Sprintf("global %s; import %s", pyImportSym(p), p), nil); err != nil {
			return errgo.Notef(err, "cannot import %s", p)
		}
	}
	boolVerifierDef := `
global bool_verifier
def bool_verifier(conds):
	v = macaroons.Verifier()
	v.satisfy_general(lambda cond: conds.get(cond, None))
	return v
`
	if err := i.interp.eval(boolVerifierDef, nil); err != nil {
		return errgo.Notef(err, "cannot define bool_verifier")
	}
	return nil
}
