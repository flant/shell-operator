package admission

import "testing"

func Test_AdmissionResponseFromFile_Allowed(t *testing.T) {
	r, err := ResponseFromFile("testdata/response/good_allow.json")

	if err != nil {
		t.Fatalf("ValidatingResponse should be loaded from file: %v", err)
	}

	if r == nil {
		t.Fatalf("ValidatingResponse should not be nil")
	}

	if !r.Allowed {
		t.Fatalf("ValidatingResponse should have allowed=true: %#v", r)
	}
}

func Test_AdmissionResponseFromFile_AllowedWithWarnings(t *testing.T) {
	r, err := ResponseFromFile("testdata/response/good_allow_warnings.json")

	if err != nil {
		t.Fatalf("ValidatingResponse should be loaded from file: %v", err)
	}

	if r == nil {
		t.Fatalf("ValidatingResponse should not be nil")
	}

	if !r.Allowed {
		t.Fatalf("ValidatingResponse should have allowed=true: %#v", r)
	}

	if len(r.Warnings) != 2 {
		t.Fatalf("ValidatingResponse should have warnings: %#v", r)
	}
}

func Test_AdmissionResponseFromFile_NotAllowed_WithMessage(t *testing.T) {
	r, err := ResponseFromFile("testdata/response/good_deny.json")

	if err != nil {
		t.Fatalf("ValidatingResponse should be loaded from file: %v", err)
	}

	if r == nil {
		t.Fatalf("ValidatingResponse should not be nil")
	}

	if r.Allowed {
		t.Fatalf("ValidatingResponse should have allowed=false: %#v", r)
	}

	if r.Message == "" {
		t.Fatalf("ValidatingResponse should have message: %#v", r)
	}
}

func Test_AdmissionResponseFromFile_NotAllowed_WithoutMessage(t *testing.T) {
	r, err := ResponseFromFile("testdata/response/good_deny_quiet.json")

	if err != nil {
		t.Fatalf("ValidatingResponse should be loaded from file: %v", err)
	}

	if r == nil {
		t.Fatalf("ValidatingResponse should not be nil")
	}

	if r.Allowed {
		t.Fatalf("ValidatingResponse should have allowed=false: %#v", r)
	}

	if r.Message != "" {
		t.Fatalf("ValidatingResponse should have no message: %#v", r)
	}
}
