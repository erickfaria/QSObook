package main

import "testing"

func TestListenAddressRequiresExplicitContainerOptIn(t *testing.T) {
	for _, tc := range []struct {
		address   string
		container bool
		valid     bool
	}{
		{"127.0.0.1:8080", false, true},
		{"[::1]:8080", false, true},
		{"127.0.0.1:8081", true, true},
		{"0.0.0.0:8080", false, false},
		{"0.0.0.0:8080", true, true},
		{"[::]:8080", true, false},
		{"192.168.1.10:8080", false, false},
		{"192.168.1.10:8080", true, false},
		{":8080", true, false},
		{"localhost:8080", true, false},
		{"not-an-address", true, false},
	} {
		if got := validateListenAddress(tc.address, tc.container); (got == nil) != tc.valid {
			t.Errorf("address=%q container=%t: %v; valid=%t", tc.address, tc.container, got, tc.valid)
		}
	}
}
