// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	mcompat "github.com/go-macaroon/macarooncompat"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon.v1"
)

type suite struct {
	origRandReader io.Reader
}

var _ = gc.Suite(&suite{})

func TestPackage(t *testing.T) {
	gc.TestingT(t)
}

func (s *suite) SetUpSuite(c *gc.C) {
	s.origRandReader = rand.Reader
}

func (s *suite) TearDownSuite(c *gc.C) {
	rand.Reader = s.origRandReader
}

func (s *suite) SetUpTest(c *gc.C) {
	// Patch rand.Reader so that the encryption used by the
	// macaroon package will be deterministic. Without this,
	// signatures produced when adding third party caveats will
	// not be deterministic, because they include a random nonce.
	//
	// When libmacaroons is changed to use a random
	// source for encryption, that will need patching too.
	rand.Reader = zeroReader{}
}

func checkConsistency(c *gc.C, f func(mcompat.Package) (interface{}, error)) {
	impls := mcompat.Implementations
	var firstVal interface{}
	var firstErr error
	for i, impl := range impls {
		c.Logf("consistency check %d: %s", i, impl.Name)
		val, err := f(impl.Pkg)
		if i == 0 {
			firstVal, firstErr = val, err
			continue
		}
		if firstErr != nil {
			if err != nil {
				continue
			}
			c.Errorf("%s succeeded without expected error %s; value %#v", impls[i].Name, firstErr, val)
			continue
		}
		if err != nil {
			c.Errorf("%s failed unexpectedly with error %#v", impls[i].Name, err)
		} else {
			c.Check(val, jc.DeepEquals, firstVal)
		}
	}
}

var signatureTests = []struct {
	about           string
	macaroon        macaroonSpec
	expectSignature string
}{{
	about: "no caveats, from libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is our super secret key; only we should know it",
		id:       "we used our secret key",
		location: "http://mybank",
	},
	expectSignature: "e3d9e02908526c4c0039ae15114115d97fdd68bf2ba379b342aaf0f617d0552f",
}, {
	about: "one caveat, from libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is our super secret key; only we should know it",
		id:       "we used our secret key",
		location: "http://mybank",
		caveats: []caveat{{
			condition: "account = 3735928559",
		}},
	},
	expectSignature: "1efe4763f290dbce0c1d08477367e11f4eee456a64933cf662d79772dbb82128",
}, {
	about: "two caveats, from libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is our super secret key; only we should know it",
		id:       "we used our secret key",
		location: "http://mybank",
		caveats: []caveat{{
			condition: "account = 3735928559",
		}, {
			condition: "time < 2015-01-01T00:00",
		}},
	},
	expectSignature: "696665d0229f9f801b588bb3f68bbdb806b26d1fbcd40ca22d9017bce4a075f1",
}, {
	about: "three caveats, from libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is our super secret key; only we should know it",
		id:       "we used our secret key",
		location: "http://mybank",
		caveats: []caveat{{
			condition: "account = 3735928559",
		}, {
			condition: "time < 2015-01-01T00:00",
		}, {
			condition: "email = alice@example.org",
		}},
	},
	expectSignature: "882e6d59496ed5245edb7ab5b8839ecd63e5d504e54839804f164070d8eed952",
}, {
	about: "one caveat, from second libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is a different super-secret key; never use the same secret twice",
		id:       "we used our other secret key",
		location: "http://mybank",
		caveats: []caveat{{
			condition: "account = 3735928559",
		}},
	},
	expectSignature: "1434e674ad84fdfdc9bc1aa00785325c8b6d57341fc7ce200ba4680c80786dda",
}, {
	about: "one 3rd party caveat, from second libmacaroons example",
	macaroon: macaroonSpec{
		rootKey:  "this is a different super-secret key; never use the same secret twice",
		id:       "we used our other secret key",
		location: "http://mybank",
		caveats: []caveat{{
			condition: "account = 3735928559",
		}, {
			rootKey:   "4; guaranteed random by a fair toss of the dice",
			condition: "this was how we remind auth of key/pred",
			location:  "http://auth.mybank/",
		}},
	},
	expectSignature: "d27db2fd1f22760e4c3dae8137e2d8fc1df6c0741c18aed4b97256bf78d1f55c",
}}

func (*suite) TestSignature(c *gc.C) {
	for i, test := range signatureTests {
		c.Logf("test %d: %s", i, test.about)
		checkConsistency(c, func(pkg mcompat.Package) (interface{}, error) {
			m := makeMacaroon(pkg, test.macaroon)
			if test.expectSignature != "" {
				c.Check(fmt.Sprintf("%x", m.Signature()), gc.Equals, test.expectSignature)
			}
			return m.Signature(), nil
		})
	}
}

func (*suite) TestBind(c *gc.C) {
	checkConsistency(c, func(pkg mcompat.Package) (interface{}, error) {
		_, macaroons := makeMacaroons(pkg, []macaroonSpec{{
			rootKey:  "this is a different super-secret key; never use the same secret twice",
			id:       "we used our other secret key",
			location: "http://mybank",
			caveats: []caveat{{
				condition: "account = 3735928559",
			}, {
				rootKey:   "4; guaranteed random by a fair toss of the dice",
				condition: "this was how we remind auth of key/pred",
				location:  "http://auth.mybank/",
			}},
		}, {
			rootKey: "4; guaranteed random by a fair toss of the dice",
			id:      "this was how we remind auth of key/pred",
			caveats: []caveat{{
				condition: "time < 2015-01-01T00:00",
			}},
		}})
		sig := macaroons[1].Signature()
		// example 2 from libmacaroons README
		c.Check(fmt.Sprintf("%x", sig), gc.Equals, "2eb01d0dd2b4475330739140188648cf25dda0425ea9f661f1574ca0a9eac54e")
		return sig, nil
	})
}

func macStr(m mcompat.Macaroon) string {
	data, err := m.MarshalBinary()
	if err != nil {
		panic(err)
	}
	var m1 macaroon.Macaroon
	if err := m1.UnmarshalBinary(data); err != nil {
		panic(err)
	}
	data, err = m1.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return string(data)
}

type caveat struct {
	rootKey   string
	location  string
	condition string
}

type macaroonSpec struct {
	rootKey  string
	id       string
	caveats  []caveat
	location string
}

func makeMacaroons(pkg mcompat.Package, mspecs []macaroonSpec) (
	rootKey []byte,
	macaroons []mcompat.Macaroon,
) {
	for _, mspec := range mspecs {
		macaroons = append(macaroons, makeMacaroon(pkg, mspec))
	}
	primary := macaroons[0]
	discharges := macaroons[1:]
	for i := range discharges {
		var err error
		discharges[i], err = discharges[i].Bind(primary)
		if err != nil {
			panic(err)
		}
	}
	return []byte(mspecs[0].rootKey), macaroons
}

func makeMacaroon(pkg mcompat.Package, mspec macaroonSpec) mcompat.Macaroon {
	m, err := pkg.New([]byte(mspec.rootKey), mspec.id, mspec.location)
	if err != nil {
		panic(err)
	}
	for _, cav := range mspec.caveats {
		if cav.location != "" {
			m, err = m.WithThirdPartyCaveat([]byte(cav.rootKey), cav.condition, cav.location)
		} else {
			m, err = m.WithFirstPartyCaveat(cav.condition)
		}
		if err != nil {
			panic(err)
		}
	}
	return m
}

type zeroReader struct{}

func (r zeroReader) Read(buf []byte) (int, error) {
	for i := range buf {
		buf[i] = 0
	}
	return len(buf), nil
}
