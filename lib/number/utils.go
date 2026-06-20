package number

import (
	"fmt"
	"regexp"
	"strconv"

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

func ValidateE164(number string) error {
	formatted := "+" + FormatNumber(number)
	country := ParseCountryCode(formatted)

	num, err := phonenumbers.Parse(formatted, country)
	if err != nil {
		return fmt.Errorf("phone number %q is not a valid E.164 number: %v", number, err)
	}

	if !phonenumbers.IsValidNumber(num) {
		return fmt.Errorf("phone number %q is not a valid E.164 number", number)
	}

	e164 := phonenumbers.Format(num, phonenumbers.E164)
	e164Pattern := regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	if !e164Pattern.MatchString(e164) {
		return fmt.Errorf("phone number %q does not conform to E.164 format (must be +[country_code][subscriber_number], max 15 digits)", number)
	}

	return nil
}
