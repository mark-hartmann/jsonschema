package jsonptr

import (
	"errors"
	"fmt"
	"testing"
)

func TestValidateJSONPointerFunc(t *testing.T) {
	var tests = []struct {
		ptr, err string
	}{
		{ptr: "#", err: "invalid JSON pointer: #"},
		{ptr: "#/", err: "invalid JSON pointer: #/"},
		{ptr: "/#"},
		{ptr: "//foo"},
		{ptr: "/foo///bar"},
		{ptr: "/~0"},
		{ptr: "/foo/~1"},
		{ptr: "/~/", err: `invalid segment "~": invalid escape sequence: ~`},
		{ptr: "/~2abc/", err: `invalid segment "~2abc": invalid escape sequence: ~2`},
		{ptr: "/foo/b#ar/ä"},
		{ptr: "/+1"},
		{ptr: "/foo/🙂/baz"},
		{ptr: "/foo/0/\u002F"},
		// Trailing slashes are legal as they are used to skip empty keys!
		{ptr: "/foo/0/\u007F/bar/1/baz/2//"},
		{ptr: "/-1"},

		// RFC examples:
		{ptr: ""},
		{ptr: "/foo"},
		{ptr: "/foo/0"},
		{ptr: "/"},
		{ptr: "/a~1b"},
		{ptr: "/c%d"},
		{ptr: "/e^f"},
		{ptr: "/g|h"},
		{ptr: "/i\\j"},
		{ptr: "/k\"l"},
		{ptr: "/ "},
		{ptr: "/m~0n"},
	}

	for i, test := range tests {
		err := ValidateJSONPointerFunc(test.ptr, nil)

		if test.err == "" && err != nil {
			t.Errorf("test[%d]: expected no error, got %q", i, err)
		}

		if (test.err != "" && err == nil) || (err != nil && err.Error() != test.err) {
			t.Errorf("test[%d]: expected error %q, got %q", i, test.err, err)
		}
	}

	t.Run("fn call count", func(t *testing.T) {
		c := make(map[string]int)
		_ = ValidateJSONPointerFunc("/foo/bar//baz", func(i int, segments []string) error {
			c[segments[i]] += 1
			return nil
		})

		if c["foo"] != 1 || c["bar"] != 1 || c[""] != 1 || c["baz"] != 1 {
			t.Errorf("expected fn to be called once per segment, was %v: ", c)
		}
	})

	t.Run("errors", func(t *testing.T) {
		err := ValidateJSONPointerFunc("/~2", nil)
		if err2 := errors.Unwrap(err); err2 == nil || err2.Error() != `invalid escape sequence: ~2` {
			t.Errorf("expected error %q, got %q", EscapeSequenceError("~2"), err)
		}
	})
}

func TestIsArrayIndex(t *testing.T) {
	var tests = []struct {
		in string
		ok bool
	}{
		{in: "1", ok: true},
		{in: "-1", ok: false},
		{in: "+1", ok: false},
		{in: "12", ok: true},
		{in: "102", ok: true},
		{in: "02", ok: false},
		{in: "0", ok: true},
		{in: "三", ok: false},
	}

	for i, test := range tests {
		if ok := IsArrayIndex(test.in); test.ok != ok {
			t.Errorf("test[%d]: expected %t, got %t", i, test.ok, ok)
		}
	}
}

func ExampleValidateJSONPointerFunc() {
	err := ValidateJSONPointerFunc("/~1", nil)
	fmt.Println(err)
	// Output: <nil>
}
func ExampleValidateJSONPointerFunc_invalidEscapeSequence() {
	err := ValidateJSONPointerFunc("/~2", nil)
	fmt.Println(err)
	// Output: invalid segment "~2": invalid escape sequence: ~2
}

func ExampleValidateJSONPointerFunc_validatorFunc() {
	var NonEmptyErr = errors.New("segment must be non-empty")
	err := ValidateJSONPointerFunc("/foo/bar//baz", func(i int, segments []string) error {
		if segments[i] == "" {
			return fmt.Errorf("invalid segment %q: %w", segments[i], NonEmptyErr)
		}
		return nil
	})
	fmt.Println(err)
	fmt.Println(errors.Is(err, NonEmptyErr))
	// Output:
	// invalid segment "": segment must be non-empty
	// true
}
