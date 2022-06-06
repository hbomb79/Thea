package profile

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/mitchellh/mapstructure"
)

type MatchType int
type ModifierType int
type MatchKey int

const (
	TITLE MatchKey = iota
	RESOLUTION
	SEASON_NUMBER
	EPISODE_NUMBER
	SOURCE_PATH
	SOURCE_NAME
	SOURCE_EXTENSION
)

func (e MatchKey) Values() []string {
	return []string{"TITLE", "RESOLUTION", "SEASON_NUMBER", "EPISODE_NUMBER", "SOURCE_PATH", "SOURCE_NAME", "SOURCE_EXTENSION"}
}

func (e MatchKey) String() string {
	return e.Values()[e]
}

const (
	EQUALS MatchType = iota
	NOT_EQUALS
	MATCHES
	DOES_NOT_MATCH
	LESS_THAN
	GREATER_THAN
	IS_PRESENT
	IS_NOT_PRESENT
)

func (e MatchType) Values() []string {
	return []string{"EQUALS", "NOT_EQUALS", "MATCHES", "DOES_NOT_MATCH", "LESS_THAN", "GREATER_THAN", "IS_PRESENT", "IS_NOT_PRESENT"}
}

func (e MatchType) String() string {
	return e.Values()[e]
}

const (
	AND ModifierType = iota
	OR
)

func (e ModifierType) Values() []string {
	return []string{"AND", "OR"}
}

func (e ModifierType) String() string {
	return e.Values()[e]
}

func MatchKeyAcceptableTypes() map[MatchKey][]MatchType {
	return map[MatchKey][]MatchType{
		TITLE:            {MATCHES, DOES_NOT_MATCH, IS_NOT_PRESENT, IS_PRESENT},
		RESOLUTION:       {MATCHES, DOES_NOT_MATCH, IS_NOT_PRESENT, IS_PRESENT},
		SEASON_NUMBER:    {EQUALS, NOT_EQUALS, LESS_THAN, GREATER_THAN, IS_NOT_PRESENT, IS_PRESENT},
		EPISODE_NUMBER:   {EQUALS, NOT_EQUALS, LESS_THAN, GREATER_THAN, IS_NOT_PRESENT, IS_PRESENT},
		SOURCE_PATH:      {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
		SOURCE_NAME:      {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
		SOURCE_EXTENSION: {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
	}
}

type MatchComponent struct {
	Key         MatchKey     `json:"key"`
	MatchType   MatchType    `json:"matchType"`
	Modifier    ModifierType `json:"modifier"`
	MatchTarget interface{}  `json:"matchTarget"`
}

func (cond *MatchComponent) Validate() error {
	const ERR_FMT = "Validation error - "
	acceptableTypes := MatchKeyAcceptableTypes()

	// 1. Ensure that the match key exists
	types, keyExists := acceptableTypes[cond.Key]
	if !keyExists {
		return fmt.Errorf("%vMatchComponent has unknown key %v", ERR_FMT, cond.Key)
	}

	// 2. Ensure the match key is compatible with the match type provided
	if i, _ := find(types, cond.MatchType); i == -1 {
		return fmt.Errorf("%vMatchComponent has key %s however the match type provided (%s) is not compatible with this key", ERR_FMT, cond.Key, cond.MatchType)
	}

	// 3. Ensure the values for the match target makes sense in the context of the match type
	switch cond.MatchType {
	case MATCHES:
		fallthrough
	case DOES_NOT_MATCH:
		// expects regular expression
		if _, err := regexp.Compile(cond.MatchTarget.(string)); err != nil {
			return fmt.Errorf("%vMatchComponent %v expects the target to be Regexp compliant, got '%v' while trying to parse '%v' as a regular expression", ERR_FMT, cond.Key, err.Error(), cond.MatchTarget)
		}
	case LESS_THAN:
		fallthrough
	case GREATER_THAN:
		fallthrough
	case EQUALS:
		fallthrough
	case NOT_EQUALS:
		// expects a integer
		if _, err := strconv.Atoi(cond.MatchTarget.(string)); err != nil {
			return fmt.Errorf("%vMatchComponent %v expects the target to be a valid int, got '%v' while trying to parse '%v' as an int", ERR_FMT, cond.Key, err.Error(), cond.MatchTarget)
		}
	}

	return nil
}

func (cond *MatchComponent) prepareIntegerComparison(val interface{}) (bool, error) {
	const ERR_FMT = "Integer comparsion failed - "
	value, err := strconv.Atoi(val.(string))
	if err != nil {
		return false, fmt.Errorf("%vExpected comparison value to be a valid integer, got (%v): %v", ERR_FMT, val, err.Error())
	}

	target, err := strconv.Atoi(cond.MatchTarget.(string))
	if err != nil {
		return false, fmt.Errorf("%vExpected target to be a valid integer, got (%v): %v", ERR_FMT, cond.MatchTarget, err.Error())
	}

	switch cond.MatchType {
	case LESS_THAN:
		return value < target, nil
	case GREATER_THAN:
		return value > target, nil
	case EQUALS:
		return value == target, nil
	case NOT_EQUALS:
		return value != target, nil
	default:
		return false, fmt.Errorf("%vMatch type %s(%v) is unknown", ERR_FMT, cond.MatchType, cond.MatchType)
	}
}

func (cond *MatchComponent) IsMatch(val interface{}) (bool, error) {
	const ERR_FMT = "Validation error - "
	switch cond.MatchType {
	case MATCHES:
		reg, err := regexp.Compile(cond.MatchTarget.(string))
		if err != nil {
			return false, fmt.Errorf("%vMatchComponent %v expects the target to be Regexp compliant, got '%v' while trying to parse '%v' as a regular expression", ERR_FMT, cond.Key, err.Error(), cond.MatchTarget)
		}

		return reg.MatchString(val.(string)), nil
	case DOES_NOT_MATCH:
		reg, err := regexp.Compile(cond.MatchTarget.(string))
		if err != nil {
			return false, fmt.Errorf("%vMatchComponent %v expects the target to be Regexp compliant, got '%v' while trying to parse '%v' as a regular expression", ERR_FMT, cond.Key, err.Error(), cond.MatchTarget)
		}

		return !reg.MatchString(val.(string)), nil
	case LESS_THAN:
		fallthrough
	case GREATER_THAN:
		fallthrough
	case EQUALS:
		fallthrough
	case NOT_EQUALS:
		return cond.prepareIntegerComparison(val)
	case IS_PRESENT:
		return val != nil, nil
	case IS_NOT_PRESENT:
		return val == nil, nil
	}

	return false, nil
}

type Profile interface {
	Tag() string
	SetMatchConditions(interface{}) error
	MatchConditions() []*MatchComponent
	SetCommand(interface{}) error
	Command() *ffmpeg.Options
	Output() string
}

type profile struct {
	sync.Mutex
	MatchCriteria  []*MatchComponent `mapstructure:"matchCriteria" json:"matchCriteria"`
	ProfileTag     string            `mapstructure:"tag" json:"tag"`
	OutputPath     string            `mapstructure:"outputPath" json:"outputPath"`
	FfmpegOptions  *ffmpeg.Options   `mapstructure:"command" json:"command"`
	ThreadBlocking bool              `mapstructure:"blocking" json:"blocking"`
}

// NewProfile accepts a single string argument (tag) and returns a new profile
// be reference to the caller with it's internal targets and tag set.
func NewProfile(tag string) Profile {
	return &profile{
		MatchCriteria: make([]*MatchComponent, 0),
		ProfileTag:    tag,
		FfmpegOptions: &ffmpeg.Options{},
	}
}

// Tag returns the profiles tag (name)
func (profile *profile) Tag() string {
	return profile.ProfileTag
}

func (profile *profile) SetCommand(command interface{}) error {
	var output *ffmpeg.Options
	err := mapstructure.WeakDecode(command, &output)
	if err != nil {
		return fmt.Errorf("failed to set Command: %v", err.Error())
	}

	profile.FfmpegOptions = output

	return nil
}

func (profile *profile) Command() *ffmpeg.Options {
	return profile.FfmpegOptions
}

func (profile *profile) Output() string {
	return profile.OutputPath
}

func (profile *profile) MatchConditions() []*MatchComponent {
	return profile.MatchCriteria
}

func (profile *profile) SetMatchConditions(conditions interface{}) error {
	var output []*MatchComponent
	err := mapstructure.Decode(conditions, &output)
	if err != nil {
		return fmt.Errorf("failed to SetMatchConditions: %v", err.Error())
	}

	if err := profile.validateMatchConditions(output); err != nil {
		return fmt.Errorf("failed to SetMatchConditions: %v", err.Error())
	}

	profile.MatchCriteria = output
	return nil
}

func find[T comparable](slice []T, item T) (int, *T) {
	for i, x := range slice {
		if x == item {
			return i, &x
		}
	}

	return -1, nil
}

func (profile *profile) validateMatchConditions(conditions []*MatchComponent) error {
	// Validate the provided match conditions
	for _, cond := range conditions {
		if err := cond.Validate(); err != nil {
			return err
		}
	}

	return nil
}
