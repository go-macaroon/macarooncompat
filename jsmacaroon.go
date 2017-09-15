// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	errgo "gopkg.in/errgo.v1"
)

var jsRunner = newJSInterp()

type jsMacaroonPkg struct{}

func (jsMacaroonPkg) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m := &jsMacaroon{
		name: newJSName("m"),
	}
	expr := fmt.Sprintf(`%s = state.macaroon.newMacaroon({
		rootKey: %s,
		identifier: %s,
		location: %s,
	})`, m.name, jsVal([]byte(rootKey)), jsVal(id), jsVal(loc))
	if err := jsRunner.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (jsMacaroonPkg) UnmarshalJSON(data []byte) (Macaroon, error) {
	m := &jsMacaroon{
		name: newJSName("m"),
	}
	expr := fmt.Sprintf(`%s = state.macaroon.importFromJSONObject(JSON.parse(%s))`, m.name, jsVal(string(data)))
	if err := jsRunner.eval(expr, nil); err != nil {
		return m, err
	}
	return m, nil
}

func (jsMacaroonPkg) UnmarshalBinary(data []byte) (Macaroon, error) {
	return nil, fmt.Errorf("unimplemented")
}

type jsMacaroon struct {
	name string
}

func (m *jsMacaroon) clone() *jsMacaroon {
	newm := &jsMacaroon{
		name: newJSName("m"),
	}
	expr := fmt.Sprintf(`%s = %s.clone()`, newm.name, m.name)
	if err := jsRunner.eval(expr, nil); err != nil {
		panic(err)
	}
	return newm
}

func (m *jsMacaroon) MarshalJSON() ([]byte, error) {
	expr := fmt.Sprintf(`JSON.stringify(%s.exportAsJSONObject())`, m.name)
	var r string
	if err := jsRunner.eval(expr, &r); err != nil {
		return nil, err
	}
	return []byte(r), nil
}

func (m *jsMacaroon) MarshalBinary() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (m *jsMacaroon) WithFirstPartyCaveat(caveatId string) (Macaroon, error) {
	m = m.clone()
	expr := fmt.Sprintf(`%s.addFirstPartyCaveat(%s)`,
		m.name, jsVal(caveatId))
	if err := jsRunner.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *jsMacaroon) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.clone()
	expr := fmt.Sprintf(`%s.addThirdPartyCaveat(%s, %s, %s)`,
		m.name, jsVal(rootKey), jsVal(caveatId), jsVal(loc))
	if err := jsRunner.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *jsMacaroon) Bind(primary Macaroon) (Macaroon, error) {
	m = m.clone()
	pm := primary.(*jsMacaroon)
	expr := fmt.Sprintf(`%s.bind(%s.signature)`, m.name, pm.name)
	if err := jsRunner.eval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *jsMacaroon) Verify(rootKey []byte, check Checker, discharges []Macaroon) error {
	dischargeNames := make([]string, len(discharges))
	for i, m := range discharges {
		dischargeNames[i] = m.(*jsMacaroon).name
	}
	checkFunc := fmt.Sprintf(`function(cav) {
		var table = %s;
		if(table[cav] === true){
			return null;
		}
		return new Error("condition not satisfied")
	}`, jsVal(check))
	expr := fmt.Sprintf(`%s.verify(%s, %s, [%s])`, m.name, jsVal(rootKey), checkFunc, strings.Join(dischargeNames, ", "))
	return jsRunner.eval(expr, nil)
}

func (m *jsMacaroon) Signature() []byte {
	expr := fmt.Sprintf(`state.btoa(String.fromCharCode.apply(null, %s.signature))`, m.name)
	var r string
	err := jsRunner.eval(expr, &r)
	if err != nil {
		panic(fmt.Errorf("cannot get signature: %v", err))
	}
	data, err := base64.StdEncoding.DecodeString(r)
	if err != nil {
		panic(fmt.Errorf("cannot decode base64 signature: %v", err))
	}
	return data
}

var jsNameSeq = 0

func newJSName(s string) string {
	n := jsNameSeq
	jsNameSeq++
	return fmt.Sprintf("state.%s%d", s, n)
}

func jsVal(v interface{}) string {
	switch v := v.(type) {
	case []byte:
		return fmt.Sprintf(`state.b64toUint8array(%q)`, base64.StdEncoding.EncodeToString(v))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return string(data)
	}
}

type jsInterp struct {
	interp *interp
}

func newJSInterp() *jsInterp {
	return &jsInterp{
		interp: newInterp("js/interp.js"),
	}
}

func (i *jsInterp) start() error {
	if i.interp.started() {
		return nil
	}
	if err := i.interp.start(); err != nil {
		return errgo.Mask(err)
	}
	// Check that it's working.
	var r bool
	if err := i.interp.eval("true;", &r); err != nil {
		return fmt.Errorf("sanity check failed: %v", err)
	}
	for _, r := range []string{"macaroon", "atob", "btoa"} {
		if err := i.interp.eval(fmt.Sprintf(`state.%s = require(%q)`, r, r), nil); err != nil {
			return fmt.Errorf("cannot require %s: %v", r, err)
		}
	}
	if err := i.interp.eval(`state.b64toUint8array = function(b64) {
		return new Uint8Array(state.atob(b64).split("").map(function(c) {
			return c.charCodeAt(0);
		}));
	}`, nil); err != nil {
		return fmt.Errorf("cannot define b64toUint8array")
	}
	return nil
}

func (i *jsInterp) eval(expr string, resultVal interface{}) error {
	if err := i.start(); err != nil {
		return fmt.Errorf("cannot start jsinterp: %v", err)
	}
	return i.interp.eval(expr, resultVal)
}
