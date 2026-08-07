package main

import (
	"golang.org/x/crypto/bcrypt"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	valid := map[string]string{
		"Player@example.com":                  "player@example.com",
		"  nation.leader+1@sub.example.org  ": "nation.leader+1@sub.example.org",
	}
	for input, want := range valid {
		got, err := normalizeEmail(input)
		if err != nil || got != want {
			t.Fatalf("normalizeEmail(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	invalid := []string{"", "player", "player@localhost", "Name <player@example.com>", "player@example..com", "@example.com"}
	for _, input := range invalid {
		if _, err := normalizeEmail(input); err == nil {
			t.Errorf("normalizeEmail(%q) unexpectedly succeeded", input)
		}
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "a-secure-test-password"
	if err := validatePassword(password); err != nil {
		t.Fatal(err)
	}
	hashA, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if string(hashA) == password || string(hashA) == string(hashB) {
		t.Fatal("bcrypt must store a salted, non-plaintext hash")
	}
	if bcrypt.CompareHashAndPassword(hashA, []byte(password)) != nil {
		t.Fatal("stored hash did not verify")
	}
	if bcrypt.CompareHashAndPassword(hashA, []byte("wrong-password")) == nil {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordLimits(t *testing.T) {
	if validatePassword("too-short") == nil {
		t.Fatal("short password accepted")
	}
	tooLong := string(make([]byte, 73))
	if validatePassword(tooLong) == nil {
		t.Fatal("password beyond bcrypt limit accepted")
	}
}
