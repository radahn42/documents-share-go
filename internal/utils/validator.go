package utils

import (
	"fmt"
	"regexp"
	"unicode"
)

var (
	loginRegex = regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)
)

func ValidateLogin(login string) error {
	if len(login) < 8 {
		return fmt.Errorf("login must be at least 8 characters long")
	}
	if !loginRegex.MatchString(login) {
		return fmt.Errorf("login must contain only latin letters and digits")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasDigit   = false
		hasSpecial = false
	)

	for _, char := range password {
		if unicode.In(char, unicode.Cyrillic) {
			return fmt.Errorf("password must not contain cyrillic characters")
		}

		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower {
		return fmt.Errorf("password must contain at least 2 letters in different case")
	}

	if !hasDigit {
		return fmt.Errorf("password must contain at least 1 digit")
	}

	if !hasSpecial {
		return fmt.Errorf("password must contain at least 1 special character")
	}

	return nil
}
