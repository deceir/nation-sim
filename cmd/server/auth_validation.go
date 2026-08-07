package main

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" || len(email) > 320 || !utf8.ValidString(email) {
		return "", errors.New("invalid email")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Address != email {
		return "", errors.New("invalid email")
	}
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 {
		return "", errors.New("invalid email")
	}
	local, domain := email[:at], strings.ToLower(email[at+1:])
	if len(local) > 64 || len(domain) > 255 || !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", errors.New("invalid email")
	}
	return strings.ToLower(local) + "@" + domain, nil
}

func validatePassword(password string) error {
	if len(password) < 10 {
		return errors.New("Password must contain at least 10 characters.")
	}
	if len([]byte(password)) > 72 {
		return errors.New("Password must be no longer than 72 bytes.")
	}
	if !utf8.ValidString(password) {
		return errors.New("Password contains invalid text.")
	}
	return nil
}
