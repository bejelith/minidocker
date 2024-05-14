package server

import "testing"

func TestCommandAuthorization(t *testing.T) {
	i := NewRBACInterceptor()
	if !i.AuthorizeCmd("admin", "anystring") {
		t.Fatal()
	}

	if !i.AuthorizeCmd("user", "ls") {
		t.Fatal()
	}

	if i.AuthorizeCmd("user", "any") {
		t.Fatal()
	}
}
