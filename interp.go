package macarooncompat

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	errgo "gopkg.in/errgo.v1"
)

type pyInterp struct {
	interp *interp
}

// newPyInterp returns an interpreter instance that will run the
// given python version (either 2 or 3).
func newPyInterp(version int) *pyInterp {
	return &pyInterp{
		interp: newInterp(fmt.Sprintf("python%d", version), "./python/interp.py"),
	}
}

func (i *pyInterp) eval(expr string, resultVal interface{}) error {
	if err := i.start(); err != nil {
		return fmt.Errorf("cannot start pyinterp: %v", err)
	}
	return i.interp.eval(expr, resultVal)
}

func (i *pyInterp) started() bool {
	return i.interp.started()
}

func (i *pyInterp) start() error {
	if i.started() {
		return nil
	}
	if err := i.interp.start(); err != nil {
		return errgo.Mask(err)
	}
	var r bool
	if err := i.interp.eval("result=True", &r); err != nil {
		return fmt.Errorf("sanity check failed: %v", err)
	}
	return nil
}

var pyNameSeq = 0

func newPyName(s string) string {
	n := pyNameSeq
	pyNameSeq++
	return fmt.Sprintf("%s%d", s, n)
}

func pyImportSym(imp string) string {
	return strings.Split(imp, ".")[0]
}

func pyVal(v interface{}) string {
	switch v := v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case []byte:
		return fmt.Sprintf(`base64.b64decode(%q)`, base64.StdEncoding.EncodeToString(v))
	case int, float64:
		return fmt.Sprint(v)
	default:
		panic(errgo.Newf("cannot encode value %v of type %T", v, v))
	}
}

type interp struct {
	cmd    string
	args   []string
	stdin  io.Writer
	stdout *bufio.Scanner
}

func newInterp(cmd string, args ...string) *interp {
	return &interp{
		cmd:  cmd,
		args: args,
	}
}

func (i *interp) started() bool {
	return i.stdin != nil
}

func (i *interp) start() error {
	if i.started() {
		return nil
	}
	log.Printf("starting %q %q", i.cmd, i.args)
	cmd := exec.Command(i.cmd, i.args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	i.stdin = stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		i.stdin = nil
		return err
	}
	i.stdout = bufio.NewScanner(stdout)
	i.stdout.Buffer(nil, 1024*1024)
	if err := cmd.Start(); err != nil {
		i.stdin = nil
		return err
	}
	return nil
}

// eval evaluates the expression or statement in expr and unmarshals
// any result into resultVal if resultVal is non-nil.
func (i *interp) eval(expr string, resultVal interface{}) error {
	if err := i.start(); err != nil {
		return fmt.Errorf("cannot start interp: %v", err)
	}
	log.Printf("eval: %s", expr)
	data := make([]byte, base64.StdEncoding.EncodedLen(len(expr))+1)
	base64.StdEncoding.Encode(data, []byte(expr))
	data[len(data)-1] = '\n'
	if _, err := i.stdin.Write(data); err != nil {
		return err
	}
	if !i.stdout.Scan() {
		if err := i.stdout.Err(); err != nil {
			return err
		}
		return io.ErrUnexpectedEOF
	}
	line := i.stdout.Bytes()
	resultData := make([]byte, base64.StdEncoding.DecodedLen(len(line)))
	n, err := base64.StdEncoding.Decode(resultData, line)
	if err != nil {
		return errgo.Notef(err, "cannot decode base64")
	}
	resultData = resultData[0:n]
	var result struct {
		Result    json.RawMessage `json:"result"`
		Exception interface{}     `json:"exception"`
	}
	if err := json.Unmarshal(resultData, &result); err != nil {
		return errgo.Notef(err, "cannot unmarshal result %q", resultData)
	}
	if result.Exception != nil {
		return fmt.Errorf("eval error on %q: %#v", expr, result.Exception)
	}
	if resultVal != nil {
		if err := json.Unmarshal(result.Result, resultVal); err != nil {
			return fmt.Errorf("cannot unmarshal return result: %v", err)
		}
	}
	return nil
}
