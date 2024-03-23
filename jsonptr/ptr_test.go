package jsonptr

import "testing"

func TestValidateJSONPointerFunc(t *testing.T) {
	var ptrTests = []struct {
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

	for _, data := range ptrTests {
		t.Run(data.ptr, func(t *testing.T) {
			err := ValidateJSONPointerFunc(data.ptr, nil)

			if data.err == "" && err != nil {
				t.Logf("expected no error, got %q", err)
				t.FailNow()
			}

			if (data.err != "" && err == nil) || (err != nil && err.Error() != data.err) {
				t.Logf("expected error %q, got %q", data.err, err)
				t.FailNow()
			}
		})
	}

	c := make(map[string]int)
	_ = ValidateJSONPointerFunc("/foo/bar//baz", func(i int, segments []string) error {
		c[segments[i]] += 1
		return nil
	})

	if c["foo"] != 1 || c["bar"] != 1 || c[""] != 1 || c["baz"] != 1 {
		t.Logf("expected fn to be called once per segment, was %v: ", c)
		t.FailNow()
	}
}
