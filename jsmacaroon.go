// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type jsMacaroonPkg struct{}

func (jsMacaroonPkg) New(rootKey []byte, id, loc string) (Macaroon, error) {
	m := &jsMacaroon{
		name: newName("m"),
	}
	expr := fmt.Sprintf(`%s = state.macaroon.newMacaroon(%s, %s, %s)`, m.name, jsBits(rootKey), jsVal(id), jsVal(loc))
	if err := jsEval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (jsMacaroonPkg) UnmarshalJSON(data []byte) (Macaroon, error) {
	m := &jsMacaroon{
		name: newName("m"),
	}
	expr := fmt.Sprintf(`%s = state.macaroon.import(JSON.parse(%s))`, m.name, jsVal(string(data)))
	if err := jsEval(expr, nil); err != nil {
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
		name: newName("m"),
	}
	expr := fmt.Sprintf(`%s = %s.clone()`, newm.name, m.name)
	if err := jsEval(expr, nil); err != nil {
		panic(err)
	}
	return newm
}

func (m *jsMacaroon) MarshalJSON() ([]byte, error) {
	expr := fmt.Sprintf(`JSON.stringify(state.macaroon.export(%s))`, m.name)
	var r string
	if err := jsEval(expr, &r); err != nil {
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
	if err := jsEval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *jsMacaroon) WithThirdPartyCaveat(rootKey []byte, caveatId string, loc string) (Macaroon, error) {
	m = m.clone()
	expr := fmt.Sprintf(`%s.addThirdPartyCaveat(%s, %s, %s)`,
		m.name, jsBits(rootKey), jsVal(caveatId), jsVal(loc))
	if err := jsEval(expr, nil); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *jsMacaroon) Bind(discharge Macaroon) (Macaroon, error) {
	m = m.clone()
	dm := discharge.(*jsMacaroon)
	expr := fmt.Sprintf(`%s.bind(%s.signature())`, m.name, dm.name)
	if err := jsEval(expr, nil); err != nil {
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
	expr := fmt.Sprintf(`%s.verify(%s, %s, [%s])`, m.name, jsBits(rootKey), checkFunc, strings.Join(dischargeNames, ", "))
	return jsEval(expr, nil)
}

func (m *jsMacaroon) Signature() []byte {
	expr := fmt.Sprintf(`state.sjcl.codec.base64.fromBits(%s.signature())`, m.name)
	var r string
	err := jsEval(expr, &r)
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

func newName(s string) string {
	n := jsNameSeq
	jsNameSeq++
	return fmt.Sprintf("state.%s%d", s, n)
}

func jsBits(data []byte) string {
	return fmt.Sprintf(`state.sjcl.codec.base64.toBits(%q)`, base64.StdEncoding.EncodeToString(data))
}

func jsVal(x interface{}) []byte {
	data, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return data
}

var (
	jsStdin  io.Writer
	jsStdout *bufio.Scanner
)

func jsStart() error {
	if jsStdin != nil {
		return nil
	}
	cmd := exec.Command("js/interp.js")
	var err error
	jsStdin, err = cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	jsStdout = bufio.NewScanner(stdout)
	if err := cmd.Start(); err != nil {
		return err
	}
	// Check that it's working.
	var r bool
	if err := jsEval("true;", &r); err != nil {
		jsStdin = nil
		jsStdout = nil
		return fmt.Errorf("sanity check failed: %v", err)
	}
	if err := jsEval(`state.macaroon = require("macaroon")`, nil); err != nil {
		jsStdin = nil
		jsStdout = nil
		return fmt.Errorf("cannot require macaroon library: %v", err)
	}
	if err := jsEval(`state.sjcl = require("sjcl")`, nil); err != nil {
		jsStdin = nil
		jsStdout = nil
		return fmt.Errorf("cannot require sjcl libary: %v", err)
	}
	return nil
}

func jsEval(expr string, resultVal interface{}) error {
	if err := jsStart(); err != nil {
		return fmt.Errorf("cannot start jsinterp: %v", err)
	}
	data := make([]byte, base64.StdEncoding.EncodedLen(len(expr))+1)
	base64.StdEncoding.Encode(data, []byte(expr))
	data[len(data)-1] = '\n'
	if _, err := jsStdin.Write(data); err != nil {
		return err
	}
	if !jsStdout.Scan() {
		if err := jsStdout.Err(); err != nil {
			return err
		}
		return io.ErrUnexpectedEOF
	}
	line := jsStdout.Bytes()
	resultData := make([]byte, base64.StdEncoding.DecodedLen(len(line)))
	n, err := base64.StdEncoding.Decode(resultData, line)
	if err != nil {
		return err
	}
	resultData = resultData[0:n]
	var result struct {
		Result    json.RawMessage `json:"result"`
		Exception string          `json:"exception"`
	}
	if err := json.Unmarshal(resultData, &result); err != nil {
		return err
	}
	if result.Exception != "" {
		return fmt.Errorf("eval error on %q: %s", expr, result.Exception)
	}
	if resultVal != nil {
		if err := json.Unmarshal(result.Result, resultVal); err != nil {
			return fmt.Errorf("cannot unmarshal return result: %v", err)
		}
	}
	return nil
}
