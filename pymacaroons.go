// Copyright 2017 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	errgo "gopkg.in/errgo.v1"
)

var pyMacaroonsRunner = []*pyMacaroonsInterp{
	2: newPyMacaroonsInterp(2),
	3: newPyMacaroonsInterp(3),
}

type pyMacaroonsPkg struct {
	version int
}

func (p pyMacaroonsPkg) eval(expr string, result interface{}) error {
	return pyMacaroonsRunner[p.version].eval(expr, result)
}

func (p pyMacaroonsPkg) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m := p.newMacaroon()
	expr := fmt.Sprintf(`%s = pymacaroons.Macaroon(location=%s, identifier=%s, key=%s)`,
		m.name, pyVal(loc), pyVal(id), pyVal(rootKey))
	if err := p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (p pyMacaroonsPkg) newMacaroon() *pyMacaroon {
	return &pyMacaroon{
		p:    p,
		name: newPyName("m"),
	}
}

func (p pyMacaroonsPkg) UnmarshalJSON(data []byte) (Macaroon, error) {
	m := p.newMacaroon()
	expr := fmt.Sprintf(`%s = pymacaroons.Macaroon.deserialize(%s, serializer=pymacaroons.serializers.JsonSerializer())`, m.name, pyVal(string(data)))
	if err := p.eval(expr, nil); err != nil {
		return m, err
	}
	return m, nil
}

func (p pyMacaroonsPkg) UnmarshalBinary(data []byte) (Macaroon, error) {
	return nil, fmt.Errorf("unimplemented")
}

type pyMacaroon struct {
	p    pyMacaroonsPkg
	name string
}

func (m *pyMacaroon) clone() *pyMacaroon {
	newm := m.p.newMacaroon()
	expr := fmt.Sprintf(`%s = %s.copy()`, newm.name, m.name)
	if err := m.p.eval(expr, nil); err != nil {
		panic(err)
	}
	return newm
}

func (m *pyMacaroon) MarshalJSON() ([]byte, error) {
	expr := fmt.Sprintf(`result = %s.serialize(pymacaroons.serializers.JsonSerializer())`,
		m.name)
	var r string
	if err := m.p.eval(expr, &r); err != nil {
		return nil, err
	}
	return []byte(r), nil
}

func (m *pyMacaroon) MarshalBinary() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (m *pyMacaroon) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m = m.clone()
	expr := fmt.Sprintf(`%s.add_first_party_caveat(%s)`,
		m.name, pyVal(caveatId))
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *pyMacaroon) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.clone()
	// Read the nonce explicitly from crypto/rand so that it can
	// be patched by the tests
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	expr := fmt.Sprintf(`%s.add_third_party_caveat(%s, %s, %s, nonce=%s)`,
		m.name, pyVal(loc), pyVal(rootKey), pyVal(caveatId), pyVal(nonce))
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *pyMacaroon) Bind(primary Macaroon) (Macaroon, error) {
	pm := primary.(*pyMacaroon)
	boundM := m.p.newMacaroon()
	expr := fmt.Sprintf(`%s = %s.prepare_for_request(%s)`, boundM.name, pm.name, m.name)
	if err := m.p.eval(expr, nil); err != nil {
		return nil, err
	}
	return boundM, nil
}

func (m *pyMacaroon) Verify(rootKey []byte, check Checker, discharges []Macaroon) error {
	dischargeNames := make([]string, len(discharges))
	for i, m := range discharges {
		dischargeNames[i] = m.(*pyMacaroon).name
	}
	checkData, _ := json.Marshal(check)
	expr := fmt.Sprintf(`bool_verifier(json.loads(%s)).verify(%s, %s, [%s])`, pyVal(string(checkData)), m.name, pyVal(rootKey), strings.Join(dischargeNames, ", "))
	return m.p.eval(expr, nil)
}

func (m *pyMacaroon) Signature() []byte {
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

type pyMacaroonsInterp struct {
	interp *pyInterp
}

func newPyMacaroonsInterp(version int) *pyMacaroonsInterp {
	return &pyMacaroonsInterp{
		interp: newPyInterp(version),
	}
}

func (i *pyMacaroonsInterp) eval(expr string, resultVal interface{}) error {
	if err := i.start(); err != nil {
		return fmt.Errorf("cannot start pyinterp: %v", err)
	}
	return i.interp.eval(expr, resultVal)
}

func (i *pyMacaroonsInterp) start() error {
	if i.interp.started() {
		return nil
	}
	if err := i.interp.start(); err != nil {
		return errgo.Mask(err)
	}
	for _, p := range []string{"pymacaroons", "base64", "pymacaroons.serializers", "json"} {
		if err := i.interp.eval(fmt.Sprintf("global %s; import %s", pyImportSym(p), p), nil); err != nil {
			return errgo.Notef(err, "cannot import %s", p)
		}
	}
	boolVerifierDef := `
global bool_verifier
def bool_verifier(conds):
	v = pymacaroons.Verifier()
	v.satisfy_general(lambda cond: conds.get(cond, None))
	return v
`
	if err := i.interp.eval(boolVerifierDef, nil); err != nil {
		return errgo.Notef(err, "cannot define bool_verifier")
	}
	return nil
}
