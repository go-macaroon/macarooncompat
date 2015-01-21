// Copyright 2015 Canonical Ltd.
// Licensed under the LGPL, see LICENCE file for details.

package macarooncompat_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon.v1"

	mcompat "github.com/go-macaroon/macarooncompat"
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

type conditionTest struct {
	conditions    map[string]bool
	expectFailure []mcompat.Implementation
	expectErr     string
}

var verifyTests = []struct {
	about      string
	macaroons  []macaroonSpec
	conditions []conditionTest
}{{
	about: "single third party caveat without discharge",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful": true,
		},
		expectErr: `cannot find discharge macaroon for caveat "bob-is-great"`,
	}},
}, {
	about: "single third party caveat with discharge",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful": true,
		},
	}, {
		conditions: map[string]bool{
			"wonderful": false,
		},
		expectErr: `condition "wonderful" not met`,
	}},
}, {
	about: "single third party caveat with discharge with mismatching root key",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key-wrong",
		id:       "bob-is-great",
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful": true,
		},
		expectErr: `signature mismatch after caveat verification`,
	}},
}, {
	about: "single third party caveat with two discharges",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
		caveats: []caveat{{
			condition: "splendid",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
		caveats: []caveat{{
			condition: "top of the world",
		}},
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful": true,
		},
		expectErr: `condition "splendid" not met`,
	}, {
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         true,
			"top of the world": true,
		},
		expectFailure: []mcompat.Implementation{mcompat.ImplLibMacaroons},
		expectErr:     `discharge macaroon "bob-is-great" was not used`,
	}, {
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         false,
			"top of the world": true,
		},
		expectFailure: []mcompat.Implementation{mcompat.ImplLibMacaroons},
		expectErr:     `condition "splendid" not met`,
	}, {
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         true,
			"top of the world": false,
		},
		expectErr: `discharge macaroon "bob-is-great" was not used`,
	}},
}, {
	about: "one discharge used for two macaroons",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "somewhere else",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}, {
			condition: "bob-is-great",
			location:  "charlie",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "somewhere else",
		caveats: []caveat{{
			condition: "bob-is-great",
			location:  "charlie",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
	}},
	conditions: []conditionTest{{
		expectFailure: []mcompat.Implementation{mcompat.ImplLibMacaroons},
		expectErr:     `discharge macaroon "bob-is-great" was used more than once`,
	}},
}, {
	about: "recursive third party caveat",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
		caveats: []caveat{{
			condition: "bob-is-great",
			location:  "charlie",
			rootKey:   "bob-caveat-root-key",
		}},
	}},
	conditions: []conditionTest{{
		expectErr: `discharge macaroon "bob-is-great" was used more than once`,
	}},
}, {
	about: "two third party caveats",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}, {
			condition: "charlie-is-great",
			location:  "charlie",
			rootKey:   "charlie-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
		caveats: []caveat{{
			condition: "splendid",
		}},
	}, {
		location: "charlie",
		rootKey:  "charlie-caveat-root-key",
		id:       "charlie-is-great",
		caveats: []caveat{{
			condition: "top of the world",
		}},
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         true,
			"top of the world": true,
		},
	}, {
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         false,
			"top of the world": true,
		},
		expectErr: `condition "splendid" not met`,
	}, {
		conditions: map[string]bool{
			"wonderful":        true,
			"splendid":         true,
			"top of the world": false,
		},
		expectErr: `condition "top of the world" not met`,
	}},
}, {
	about: "third party caveat with undischarged third party caveat",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	}, {
		location: "bob",
		rootKey:  "bob-caveat-root-key",
		id:       "bob-is-great",
		caveats: []caveat{{
			condition: "splendid",
		}, {
			condition: "barbara-is-great",
			location:  "barbara",
			rootKey:   "barbara-caveat-root-key",
		}},
	}},
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful": true,
			"splendid":  true,
		},
		expectErr: `cannot find discharge macaroon for caveat "barbara-is-great"`,
	}},
}, {
	about:     "recursive third party caveats",
	macaroons: recursiveThirdPartyCaveatMacaroons,
	conditions: []conditionTest{{
		conditions: map[string]bool{
			"wonderful":   true,
			"splendid":    true,
			"high-fiving": true,
			"spiffing":    true,
		},
	}, {
		conditions: map[string]bool{
			"wonderful":   true,
			"splendid":    true,
			"high-fiving": false,
			"spiffing":    true,
		},
		expectErr: `condition "high-fiving" not met`,
	}},
}, {
	about: "unused discharge",
	macaroons: []macaroonSpec{{
		rootKey: "root-key",
		id:      "root-id",
	}, {
		rootKey: "other-key",
		id:      "unused",
	}},
	conditions: []conditionTest{{
		expectFailure: []mcompat.Implementation{mcompat.ImplLibMacaroons},
		expectErr:     `discharge macaroon "unused" was not used`,
	}},
}}

var recursiveThirdPartyCaveatMacaroons = []macaroonSpec{{
	rootKey: "root-key",
	id:      "root-id",
	caveats: []caveat{{
		condition: "wonderful",
	}, {
		condition: "bob-is-great",
		location:  "bob",
		rootKey:   "bob-caveat-root-key",
	}, {
		condition: "charlie-is-great",
		location:  "charlie",
		rootKey:   "charlie-caveat-root-key",
	}},
}, {
	location: "bob",
	rootKey:  "bob-caveat-root-key",
	id:       "bob-is-great",
	caveats: []caveat{{
		condition: "splendid",
	}, {
		condition: "barbara-is-great",
		location:  "barbara",
		rootKey:   "barbara-caveat-root-key",
	}},
}, {
	location: "charlie",
	rootKey:  "charlie-caveat-root-key",
	id:       "charlie-is-great",
	caveats: []caveat{{
		condition: "splendid",
	}, {
		condition: "celine-is-great",
		location:  "celine",
		rootKey:   "celine-caveat-root-key",
	}},
}, {
	location: "barbara",
	rootKey:  "barbara-caveat-root-key",
	id:       "barbara-is-great",
	caveats: []caveat{{
		condition: "spiffing",
	}, {
		condition: "ben-is-great",
		location:  "ben",
		rootKey:   "ben-caveat-root-key",
	}},
}, {
	location: "ben",
	rootKey:  "ben-caveat-root-key",
	id:       "ben-is-great",
}, {
	location: "celine",
	rootKey:  "celine-caveat-root-key",
	id:       "celine-is-great",
	caveats: []caveat{{
		condition: "high-fiving",
	}},
}}

func (*suite) TestVerify(c *gc.C) {
	for i, test := range verifyTests {
		c.Logf("test %d: %s", i, test.about)

		for _, impl := range mcompat.Implementations {
			c.Logf("- implementation: %s", impl.Name)
			rootKey, macaroons := makeMacaroons(impl.Pkg, test.macaroons)
			for _, cond := range test.conditions {
				c.Logf("-- conditions %#v", cond.conditions)
				err := macaroons[0].Verify(
					rootKey,
					cond.conditions,
					macaroons[1:],
				)
				expectFail := false
				for _, fimpl := range cond.expectFailure {
					if fimpl == impl.Name {
						expectFail = true
					}
				}
				if expectFail {
					if cond.expectErr != "" {
						c.Assert(err, gc.IsNil, gc.Commentf("unexpected success"))
					} else {
						c.Assert(err, gc.NotNil, gc.Commentf("unexpected success"))
					}
				} else {
					if cond.expectErr != "" {
						c.Assert(err, gc.NotNil)
					} else {
						c.Assert(err, gc.IsNil)
					}
				}
			}
		}
	}
}

type serializationTest struct {
	about    string
	macaroon macaroonSpec
}

var serializationTests = []serializationTest{{
	about: "vanilla macaroon",
	macaroon: macaroonSpec{
		rootKey: "root-key",
		id:      "root-id",
		caveats: []caveat{{
			condition: "wonderful",
		}, {
			condition: "bob-is-great",
			location:  "bob",
			rootKey:   "bob-caveat-root-key",
		}},
	},
}, {
	about: "macaroon with non-ascii text",
	macaroon: macaroonSpec{
		rootKey:  "root-key-♔",
		id:       "root-γ-♔",
		location: "Москва",
		caveats: []caveat{{
			condition: "π > 3",
		}, {
			condition: "∃χ: ∀ι∈χ: ι≠∅",
			location:  "Αθήνα",
			rootKey:   "root-key-ζ",
		}},
	},
}}

func (*suite) TestSerialization(c *gc.C) {
	tests := serializationTests
	// Add all the macaroons from verifyTests just to make sure.
	for i, vtest := range verifyTests {
		for j, m := range vtest.macaroons {
			tests = append(tests, serializationTest{
				about:    fmt.Sprintf("verify test %d.%d: %s", i, j, vtest.about),
				macaroon: m,
			})
		}
	}
	for i, test := range serializationTests {
		c.Logf("\ntest %d: %s", i, test.about)
		for _, impl := range mcompat.Implementations {
			c.Logf("check %s", impl.Name)
			pkg := impl.Pkg
			m := makeMacaroon(pkg, test.macaroon)
			data, err := m.MarshalJSON()
			c.Assert(err, gc.IsNil)
			c.Logf("macaroon data:\n%s", data)
			c.Logf("---- unmarshal checks {")
			// Check the marshaled form can be unmarshaled by all the other packages.
			// and that it can be marshaled back and eventually produces the
			// same representation for all packages.
			checkConsistency(c, func(pkg mcompat.Package) (interface{}, error) {
				m, err := pkg.UnmarshalJSON(data)
				c.Assert(err, gc.IsNil, gc.Commentf("data: %s", data))
				data, err := m.MarshalJSON()
				c.Assert(err, gc.IsNil)

				var gom *macaroon.Macaroon
				err = json.Unmarshal(data, &gom)
				c.Assert(err, gc.IsNil)
				data, err = gom.MarshalJSON()
				c.Assert(err, gc.IsNil)
				return string(data), nil
			})
			c.Logf("}")
		}
	}
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
