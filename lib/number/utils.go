package number

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	phoneiso3166 "github.com/onlinecity/go-phone-iso3166"
	"github.com/nyaruka/phonenumbers"
)

// FormatNumber formats a phone number to remove
// unnecessary chars and avoid dealing with unwanted input.
func FormatNumber(n string) string {
	re := regexp.MustCompile(`[_\W]+`)
	number := re.ReplaceAllString(n, "")

	return number
}

// ParseCountryCode parses a phone number and returns ISO country code.
// This is required in order to use the phonenumbers library.
func ParseCountryCode(n string) string {
	var number uint64
	number, _ = strconv.ParseUint(FormatNumber(n), 10, 64)

	return phoneiso3166.E164.Lookup(number)
}

// IsValid indicate if a phone number has a valid format.
func IsValid(number string) bool {
	number = FormatNumber(number)

	re := regexp.MustCompile("^[0-9]+$")

	return len(re.FindString(number)) != 0
}

func ValidateE164(input string) error {
	working := strings.ReplaceAll(input, "\u00a0", " ")
	working = strings.ReplaceAll(working, "\u202f", " ")
	working = strings.ReplaceAll(working, "\u3000", " ")

	trimmed := strings.TrimSpace(working)

	plusPrefix := false
	switch {
	case strings.HasPrefix(trimmed, "+"):
		plusPrefix = true
		trimmed = strings.TrimPrefix(trimmed, "+")
	case strings.HasPrefix(trimmed, "00"):
		plusPrefix = true
		trimmed = strings.TrimPrefix(trimmed, "00")
	}

	reStrip := regexp.MustCompile(`[\s\-\(\)\.\[\]\{\}\<\>\*\/\\\,\;\:\_\#\~\!\@\$\%\^\&\=\|\\\\\x{2010}\x{2011}\x{2012}\x{2013}\x{2014}\x{2015}\x{2212}\x{FE58}\x{FF0D}\x{FE63}]+`)
	digitsPart := reStrip.ReplaceAllString(trimmed, "")

	digitsRe := regexp.MustCompile(`^[0-9]+$`)
	if !digitsRe.MatchString(digitsPart) {
		cleanedShow := digitsPart
		if plusPrefix {
			cleanedShow = "+" + cleanedShow
		}
		return fmt.Errorf("phone number %q contains invalid characters after stripping formatting (cleaned: %q). E.164 numbers may only contain digits and a leading +", input, cleanedShow)
	}

	cleaned := "+" + digitsPart

	e164Pattern := regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	if !e164Pattern.MatchString(cleaned) {
		return fmt.Errorf("phone number %q is not a valid E.164 number (cleaned: %q). E.164 format: +[country_code][subscriber_number], max 15 digits total", input, cleaned)
	}

	country := ParseCountryCode(cleaned)

	num, err := phonenumbers.Parse(cleaned, country)
	if err != nil {
		return fmt.Errorf("phone number %q is not a valid E.164 number: %v (cleaned: %q)", input, err, cleaned)
	}

	if !phonenumbers.IsValidNumber(num) {
		return fmt.Errorf("phone number %q is not a valid E.164 number (cleaned: %q)", input, cleaned)
	}

	return nil
}
