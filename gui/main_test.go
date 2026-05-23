package main

import (
	"regexp"
	"testing"
)

func TestSingleInstanceLockConfig(t *testing.T) {
	lock := singleInstanceLock(NewApp())
	if lock == nil {
		t.Fatal("single instance lock is nil")
	}
	if lock.UniqueId != singleInstanceUniqueID {
		t.Fatalf("unique id = %q, want %q", lock.UniqueId, singleInstanceUniqueID)
	}
	if lock.OnSecondInstanceLaunch == nil {
		t.Fatal("second instance callback is nil")
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(lock.UniqueId) {
		t.Fatalf("unique id %q is not a UUID v4", lock.UniqueId)
	}
}
