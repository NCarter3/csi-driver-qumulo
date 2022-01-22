package qumulo

import (
	"strings"
	"testing"
)

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorMatchesString(t *testing.T, err error, needle string) {
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	if !strings.Contains(err.Error(), needle) {
		t.Fatalf("error does not match %q: %q", needle, err.Error())
	}
}

func assertErrorEqualsString(t *testing.T, err error, needle string) {
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	if err.Error() == needle {
		t.Fatalf("error does not equal %q: %q", needle, err.Error())
	}
}

func assertRestError(t *testing.T, err error, expectedStatus int, expectedErrorClass string) {
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	switch err.(type) {
	case RestError:
		z := err.(RestError)
		if z.StatusCode != expectedStatus {
			t.Fatalf("error status %d != %d: %v", expectedStatus, z.StatusCode, err)
		}
		if z.ErrorClass != expectedErrorClass {
			t.Fatalf("error class does not match %q: %q", expectedErrorClass, z)
		}
	default:
		t.Fatalf("unexpected error: %v", err)
	}
}
