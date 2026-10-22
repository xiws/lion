package main

import (
	"reflect"
	"testing"
)

func TestParseHeaderFlags(t *testing.T) {
	got := parseHeaderFlags([]string{
		"X-A=1",
		"Authorization: Bearer dev",
		" x-b : 2 ",
		"X-Empty=",
		"no-separator",
		"=no-key",
		": no-key",
	})
	want := map[string]string{
		"X-A":           "1",
		"Authorization": "Bearer dev",
		"x-b":           "2",
		"X-Empty":       "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHeaderFlags() = %#v, want %#v", got, want)
	}
}
