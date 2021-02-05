package string_helper

import "testing"

func Test_TrimGroup(t *testing.T) {
	var in string
	var actual string
	var expect string

	in = "stable.example.com/v1beta1"
	expect = "v1beta1"
	actual = TrimGroup(in)
	if actual != expect {
		t.Fatalf("group prefix should be trimmed: expect '%s', got '%s'", expect, actual)
	}

	in = "v1beta1"
	expect = "v1beta1"
	actual = TrimGroup(in)
	if actual != expect {
		t.Fatalf("group prefix should be trimmed: expect '%s', got '%s'", expect, actual)
	}

	in = "stable.example.com/"
	expect = ""
	actual = TrimGroup(in)
	if actual != expect {
		t.Fatalf("group prefix should be trimmed: expect '%s', got '%s'", expect, actual)
	}

	in = ""
	expect = ""
	actual = TrimGroup(in)
	if actual != expect {
		t.Fatalf("group prefix should be trimmed: expect '%s', got '%s'", expect, actual)
	}
}
