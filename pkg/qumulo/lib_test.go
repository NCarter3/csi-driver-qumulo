package qumulo

import (
	"testing"
)

func assertRestError(t *testing.T, err error, expectedStatus int, expectedErrorClass string) {
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	switch err.(type) {
	case RestError:
		z := err.(RestError)
		if z.StatusCode != expectedStatus {
			t.Fatalf("error status %d != %d: %v", expectedStatus, z.StatusCode, z)
		}
		if z.ErrorClass != expectedErrorClass {
			t.Fatalf("error class %q does not match %q: %q", expectedErrorClass, z.ErrorClass, z)
		}
	default:
		t.Fatalf("unexpected error: %v", err)
	}
}
