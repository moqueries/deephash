package deephash_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"moqueries.org/deephash"
)

type testStruct struct {
	S         string
	I         int
	I8        int8
	I16       int16
	I32       int32
	I64       int64
	U8        uint8
	U16       uint16
	U32       uint32
	U64       uint64
	F32       float32
	F64       float64
	Interface interface{}
}

var differentTestCases = []interface{}{
	// simple types
	"dave",
	"foo",
	"foobar",
	" foo",
	1,
	1.0,

	// structs
	testStruct{S: "foo"},
	testStruct{S: "bar"},

	// pointers to structs
	&testStruct{S: "foo1"},
	&testStruct{S: "bar1"},

	// structs with different types of ints
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 43, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 44, I8: 44, I16: 44, I32: 44, I64: 44, U8: 44, U16: 44, U32: 44, U64: 44},
	&testStruct{I: 11, I8: 43, I16: 43, I32: 43, I64: 43, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 11, I16: 43, I32: 43, I64: 43, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 11, I32: 43, I64: 43, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 11, I64: 43, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 11, U8: 43, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 43, U8: 11, U16: 43, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 43, U8: 43, U16: 11, U32: 43, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 43, U8: 43, U16: 43, U32: 11, U64: 43},
	&testStruct{I: 43, I8: 43, I16: 43, I32: 43, I64: 43, U8: 43, U16: 43, U32: 43, U64: 11},

	// structs with different types of floats
	&testStruct{F32: 43.0, F64: 43.0},
	&testStruct{F32: 44.0, F64: 44.0},
	&testStruct{F32: 11.0, F64: 43.0},
	&testStruct{F32: 43.0, F64: 11.0},

	// string maps
	map[string]testStruct{
		"foo": {S: "baz"},
		"bar": {S: "baz"},
	},
	map[string]testStruct{
		"foo": {S: "BAZZER"},
		"bar": {S: "BAZZER"},
	},

	// other maps
	map[testStruct]testStruct{
		testStruct{S: "baz"}: {S: "baz"},
		testStruct{S: "bar"}: {S: "bar"},
	},

	// slices -- ordered here
	[]testStruct{
		{S: "foo"},
		{S: "bar"},
		{S: "baz"},
	},
	[]testStruct{
		{S: "bar"},
		{S: "foo"},
		{S: "baz"},
	},
	[]testStruct{
		{S: "bar"},
		{S: "baz"},
		{S: "foo"},
	},

	// arrays -- we're looking at the contents, so we have to be different to the slices
	[3]testStruct{
		{S: "FOO"},
		{S: "BAR"},
		{S: "BAZ"},
	},
	[3]testStruct{
		{S: "BAR"},
		{S: "FOO"},
		{S: "BAZ"},
	},
	[3]testStruct{
		{S: "BAR"},
		{S: "BAZ"},
		{S: "FOO"},
	},

	// Interface types
	&testStruct{Interface: testStruct{I: 42}},
	&testStruct{Interface: &testStruct{I: 100}},
}

func TestDifferentCases(t *testing.T) {
	seen := make(map[string]bool)
	for n, tc := range differentTestCases {
		t.Run(fmt.Sprintf("[%d] %#v", n, tc), func(t *testing.T) {
			h := deephash.Hash(tc)
			hs := fmt.Sprintf("%x", h)
			if h == 0 {
				t.Errorf("Test case %v yields zero hash", tc)
				return
			}
			if seen[hs] {
				t.Errorf("Test case %v hashes to %v which has already been seen", tc, hs)
			}
			seen[hs] = true
		})
	}
}

func TestSameCases(t *testing.T) {
	for name, tcs := range map[string][]interface{}{
		"simple stuff": {
			"foo",
			"foo",
		},

		"hash order shouldn't matter": {
			map[string]testStruct{
				"foo": {S: "baz"},
				"bar": {S: "baz"},
			},
			map[string]testStruct{
				"bar": {S: "baz"},
				"foo": {S: "baz"},
			},
		},

		"we care about the contents, so we want different values of a struct with same contents to be same": {
			&testStruct{F32: 43.0, F64: 43.0},
			&testStruct{F32: 43.0, F64: 43.0},
			testStruct{F32: 43.0, F64: 43.0},
		},

		"slices and arrays should match": {
			[3]testStruct{
				{S: "FOO"},
				{S: "BAR"},
				{S: "BAZ"},
			},
			[]testStruct{
				{S: "FOO"},
				{S: "BAR"},
				{S: "BAZ"},
			},
			[]testStruct{
				{S: "FOO"},
				{S: "BAR"},
				{S: "BAZ"},
			},
		},

		"We should follow pointers of pointers and pointers within interfaces": {
			&testStruct{Interface: testStruct{I: 42}},
			&testStruct{Interface: &testStruct{I: 42}},
			&testStruct{Interface: reflect.ValueOf(&testStruct{I: 42}).Interface()},
		},
	} {
		t.Run(name, func(t *testing.T) {
			hash := uint64(0)
			var first interface{}
			for n, tc := range tcs {
				h := deephash.Hash(tc)
				if h == 0 {
					t.Errorf("Test case %v yields zero hash", tc)
					continue
				}

				if hash == 0 {
					hash = h
				} else if hash != h {
					t.Errorf("Test case %v hashes to '%v' which is different to previous '%v'", tc, h, hash)
				}

				if n == 0 {
					first = tc
				} else {
					diffs := deephash.Diff("xyz", first, tc)
					if len(diffs) > 0 {
						t.Errorf("got %#v, want no diffs", diffs)
					}

					diffs = deephash.Diff("abc", tc, first)
					if len(diffs) > 0 {
						t.Errorf("got %#v, want no diffs", diffs)
					}
				}
			}
		})
	}
}

type circular struct {
	V *circular
}

func TestCircular(t *testing.T) {
	a := &circular{}
	b := &circular{V: a}

	h := deephash.Hash(b)
	if h == 0 {
		t.Error("Hash circular should yield some hash value")
	}

	// now actually circular it up
	a.V = b
	h = deephash.Hash(b)
	if h == 0 {
		t.Error("Hash circular should yield some hash value")
	}
}

type RefB struct {
	Id string
}

type RefA struct {
	Id string
	B  RefB
}

func TestRef(t *testing.T) {
	a := RefA{
		Id: "test",
		B:  RefB{Id: "anothertest"},
	}
	b := RefA{
		Id: "test",
		B:  RefB{Id: "anothertest"},
	}

	if deephash.Hash(a) != deephash.Hash(b) {
		t.Fatal("Expecting our two reference cases to hash the same even though different underlying objects, because same values")
	}
}

func TestBooleans(t *testing.T) {
	if deephash.Hash(true) == deephash.Hash(false) {
		t.Fatal("Expecting true to hash differently than false")
	}
}

func TestDiff(t *testing.T) {
	for name, tc := range map[string]struct {
		lSrc, rSrc interface{}
		expected   []string
	}{
		"strings": {lSrc: "1", rSrc: "2", expected: []string{"xyz is not equal"}},
		"structs": {
			lSrc: testStruct{I: 31, S: "1", Interface: testStruct{I: 42}},
			rSrc: testStruct{I: 31, S: "2", Interface: 42},
			expected: []string{
				"xyz.Interface is not equal",
				"xyz.Interface.F32 is not equal",
				"xyz.Interface.F64 is not equal",
				"xyz.Interface.I is not equal",
				"xyz.Interface.I16 is not equal",
				"xyz.Interface.I32 is not equal",
				"xyz.Interface.I64 is not equal",
				"xyz.Interface.I8 is not equal",
				"xyz.Interface.S is not equal",
				"xyz.Interface.U16 is not equal",
				"xyz.Interface.U32 is not equal",
				"xyz.Interface.U64 is not equal",
				"xyz.Interface.U8 is not equal",
				"xyz.S is not equal",
			},
		},
		"map keys": {
			lSrc: map[string]int{"key1": 42},
			rSrc: map[string]int{"key2": 42},
			expected: []string{
				"xyz[key1] is not equal",
				"xyz[key1-key] is not equal",
				"xyz[key2] is not equal",
				"xyz[key2-key] is not equal",
			},
		},
		"map values": {
			lSrc:     map[string]int{"key1": 42},
			rSrc:     map[string]int{"key1": 43},
			expected: []string{"xyz[key1] is not equal"},
		},
		"slices": {
			lSrc:     []int{1, 2, 3, 4},
			rSrc:     []int{1, 5, 3, 6},
			expected: []string{"xyz[1] is not equal", "xyz[3] is not equal"},
		},
		"arrays": {
			lSrc:     [4]int{1, 2, 3, 4},
			rSrc:     [4]int{1, 5, 3, 6},
			expected: []string{"xyz[1] is not equal", "xyz[3] is not equal"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			sort.Strings(tc.expected)

			diffs := deephash.Diff("xyz", tc.lSrc, tc.rSrc)
			sort.Strings(diffs)
			if !reflect.DeepEqual(diffs, tc.expected) {
				t.Errorf("got %#v, want %#v", diffs, tc.expected)
			}

			diffs = deephash.Diff("xyz", tc.rSrc, tc.lSrc)
			sort.Strings(diffs)
			if !reflect.DeepEqual(diffs, tc.expected) {
				t.Errorf("got %#v, want %#v", diffs, tc.expected)
			}
		})
	}
}

type parent struct {
	c1, c2 *child
}

type child struct {
	val string
}

func TestStackCycle(t *testing.T) {
	c1 := child{val: "child"}
	c2 := child{val: "child"}

	ch1 := deephash.Hash(&c1)
	ch2 := deephash.Hash(&c2)
	if ch1 != ch2 {
		t.Fatalf("got %d != %d, want equal", ch1, ch2)
	}

	p1 := parent{c1: &c1, c2: &c1}
	p2 := parent{c1: &c1, c2: &c2}
	ph1 := deephash.Hash(p1)
	ph2 := deephash.Hash(p2)
	if ph1 != ph2 {
		t.Errorf("got %d != %d, want equal", ph1, ph2)
	}

	diffs := deephash.Diff("", &p1, &p2)
	if len(diffs) != 0 {
		t.Errorf("got %#v, want no differences", diffs)
	}

	diffs = deephash.Diff("", &p2, &p1)
	if len(diffs) != 0 {
		t.Errorf("got %#v, want no differences", diffs)
	}
}

func BenchmarkHash(b *testing.B) {
	for n, tc := range differentTestCases {
		b.Run(fmt.Sprintf("[%d] %#v", n, tc), func(b *testing.B) {
			for name, fn := range map[string]func(interface{}){
				// "upstream": func(i interface{}) {
				// 	b := upstream.Hash(i)
				// 	const hashBytes = 8
				// 	if len(b) < hashBytes {
				// 		newB := make([]byte, hashBytes)
				// 		copy(newB, b)
				// 		b = newB
				// 	}
				// 	h := binary.LittleEndian.Uint64(b)
				// 	if false {
				// 		fmt.Println(h)
				// 	}
				// },
				"deep hash": func(i interface{}) {
					h := deephash.Hash(i)
					if false {
						fmt.Println(h)
					}
				},
				// "fast deep hash": func(i interface{}) {
				// 	h := deephash.FastHash(i)
				// 	if false {
				// 		fmt.Println(h)
				// 	}
				// },
			} {
				b.Run(name, func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						fn(tc)
					}
				})
			}
		})
	}
}
