package match

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/media"
)

// Criteria is a struct that contains the basic information for performing
// some matching against other data, mainly media Containers.
// For example, a criteria might be "TITLE MATCHES 'pattern' AND". This is
// made up of four terms: the key, type, value and combine type (in order).
type Criteria struct {
	ID          uuid.UUID
	WorkflowID  uuid.UUID
	Key         Key         `json:"key"`
	Type        Type        `json:"type"`
	Value       string      `json:"value"`
	CombineType CombineType `json:"combine_type"`
}

// ValidateLegal ensures the criteria is LEGAL:
// - Does the key specified exist,
// - Is the match key specified compatible with the match type provided (e.g., you can't perform LESS_THAN on a STRING type.)
// - Is the value specified sensible for the match key (i.e. you cannot use a number as the right-side of a 'MATCHES' match type)
func (criteria *Criteria) ValidateLegal() error {
	if !IsTypeAcceptable(criteria.Key, criteria.Type) {
		return fmt.Errorf("match key %s does not accept match type %s", criteria.Key, criteria.Type)
	}

	switch criteria.Type {
	case MATCHES:
		fallthrough
	case DOES_NOT_MATCH:
		// expects regular expression
		if _, err := regexp.Compile(criteria.Value); err != nil {
			return fmt.Errorf("match type %s expects a valid regular expression as the value; '%v' is not parseable as a regular expression", criteria.Type, criteria.Value)
		}
	case LESS_THAN:
		fallthrough
	case GREATER_THAN:
		fallthrough
	case EQUALS:
		fallthrough
	case NOT_EQUALS:
		// expects a integer
		if _, err := strconv.Atoi(criteria.Value); err != nil {
			return fmt.Errorf("match type %s expects a valid int as the value; '%v' is not a valid int", criteria.Type, criteria.Value)
		}
	}

	return nil
}

// IsMediaAcceptable accepts a media container and checks to see if the criteria
// is a valid match against the media. It does this by using the critera's key to extract
// relevant information from the container, and then performing simple checks against it
// using the Type and Value of the criteria
func (criteria *Criteria) IsMediaAcceptable(m *media.Container) (bool, error) {
	var valueToCheck any
	switch criteria.Key {
	case TITLE:
		valueToCheck = m.Title()
	case RESOLUTION:
		valueToCheck, _ = m.Resolution()
	case EPISODE_NUMBER:
		if m.EpisodeNumber() != -1 {
			valueToCheck = m.EpisodeNumber()
		} else {
			valueToCheck = nil
		}
	case SEASON_NUMBER:
		if m.SeasonNumber() != -1 {
			valueToCheck = m.SeasonNumber()
		} else {
			valueToCheck = nil
		}
	case SOURCE_EXTENSION:
		valueToCheck = filepath.Ext(m.Source())
	case SOURCE_NAME:
		valueToCheck = filepath.Base(m.Source())
	case SOURCE_PATH:
		valueToCheck = m.Source()
	}

	isMatch, err := criteria.isValueAcceptable(valueToCheck)
	if err != nil {
		return false, fmt.Errorf("media %s is not acceptable for criteria %s: %s", m, criteria, err.Error())
	}

	return isMatch, nil
}

// isValueAcceptable is responsible for performing the underlying data checks using
// the value provided AND the Type/Value set in the criteria.
//
// Only if the data is coercible to the criteria Type, AND the values both match, will true be returned.
// Else, false will be returned.
// An error is ONLY returned if the match failed due to underlying problems with the criteria, NOT if
// the criteria is valid but simply wasn't a match for this val.
func (criteria *Criteria) isValueAcceptable(valToTest interface{}) (bool, error) {
	if valToTest == nil {
		switch criteria.Type {
		case IS_PRESENT:
			return false, nil
		case IS_NOT_PRESENT:
			return true, nil
		default:
			return false, fmt.Errorf("nil is not acceptable for criteria type %s", criteria.Type)
		}
	}

	switch criteria.Type {
	case MATCHES:
		return criteria.testStringEquality(valToTest)
	case DOES_NOT_MATCH:
		match, err := criteria.testStringEquality(valToTest)
		if err != nil {
			return false, err
		}

		return !match, nil
	case LESS_THAN:
		fallthrough
	case GREATER_THAN:
		fallthrough
	case EQUALS:
		fallthrough
	case NOT_EQUALS:
		return criteria.performIntComparison(valToTest)
	}

	return false, fmt.Errorf("criteria type %s unknown, cannot test %v and %v", criteria.Type, criteria.Value, valToTest)
}

// performStringComparison attempts to test the given value against the criterias Value. If either the
// criteria Value or the valToTets provided cannot be coerced to a string, an error will be returned.
//
// This function checks for equality differently depending on whether the critiera Value (not the valToTest
// passed to the function) is marked as a regular expression:
//
// If it IS marked as a regular expression (surrounded with '/'):
//   - If the string can be parsed as a regexp, then the val provided will be tested to see if it matches
//   - If the string cannot be parsed as a regular expression, an error will be returned.
//
// If the Value is NOT marked as a regular expression, a standard strings.Compare will be used to test for equality.
func (criteria *Criteria) testStringEquality(valToTest any) (bool, error) {
	if valToTest == nil {
		return false, fmt.Errorf("val %v cannot be coerced to a string as it's 'nil'", valToTest)
	}

	criteriaStrValue, err := toString(criteria.Value)
	if err != nil {
		return false, err
	}

	strValToTest, err := toString(valToTest)
	if err != nil {
		return false, err
	}

	strLen := len(criteriaStrValue)
	if criteriaStrValue[0] == '/' && criteriaStrValue[strLen-1] == '/' {
		pattern, err := regexp.Compile(criteriaStrValue)
		if err != nil {
			return false, err
		}

		return pattern.MatchString(strValToTest), nil
	}

	return strings.Compare(criteriaStrValue, strValToTest) == 0, nil
}

// performIntComparison accepts a valToTest and attempts to compare it with the criterias Value
// according to the criterias Type.
// If either the criteria Value or the valToTest cannot be coereced to an int, an error is returned.
func (criteria *Criteria) performIntComparison(valToTest interface{}) (bool, error) {
	if valToTest == nil {
		return false, fmt.Errorf("val %v cannot be coerced to an integer as it's 'nil'", valToTest)
	}

	criteriaIntValue, err := toInt(criteria.Value)
	if err != nil {
		return false, fmt.Errorf("criteria value illegal: %s", err.Error())
	}

	intToCheck, err := toInt(valToTest)
	if err != nil {
		return false, fmt.Errorf("value to check illegal: %s", err.Error())
	}

	switch criteria.Type {
	case LESS_THAN:
		return criteriaIntValue < intToCheck, nil
	case GREATER_THAN:
		return criteriaIntValue > intToCheck, nil
	case EQUALS:
		return criteriaIntValue == intToCheck, nil
	case NOT_EQUALS:
		return criteriaIntValue != intToCheck, nil
	default:
		return false, fmt.Errorf("criteria type %s is not valid for key %s (integer type)", criteria.Type, criteria.Key)
	}
}

// toString attempts to coerce the val provided to a string type, failing
// with an empty string and an error if it cannot.
func toString(val any) (string, error) {
	out, isStr := val.(string)
	if isStr {
		return out, nil
	}

	return "", fmt.Errorf("val %v cannot be coerced to a string", val)
}

// toInt accepts any value and attempts to coerce it to any.
// If the underlying type of the 'any' val is ALREADY an int, it
// will simply be returned.
// If not, then the val is converted to a string and parsed to an int, returning
// -1 and an error if either of those steps fail.
func toInt(val any) (int, error) {
	if out, isInt := val.(int); isInt {
		return out, nil
	}

	if str, isStr := val.(string); !isStr {
		return -1, fmt.Errorf("value %v cannot be coerced to integer because it has no string representation", val)
	} else {
		out, err := strconv.Atoi(str)
		if err != nil {
			return -1, fmt.Errorf("value %v cannot be coerced to integer: %s", val, err.Error())
		}

		return out, nil
	}
}
