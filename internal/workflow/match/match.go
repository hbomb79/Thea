package match

type Key int

const (
	TITLE Key = iota
	RESOLUTION
	SEASON_NUMBER
	EPISODE_NUMBER
	SOURCE_PATH
	SOURCE_NAME
	SOURCE_EXTENSION
)

func (e Key) Values() []string {
	return []string{"TITLE", "RESOLUTION", "SEASON_NUMBER", "EPISODE_NUMBER", "SOURCE_PATH", "SOURCE_NAME", "SOURCE_EXTENSION"}
}

func (e Key) String() string {
	return e.Values()[e]
}

type Type int

const (
	EQUALS Type = iota
	NOT_EQUALS
	MATCHES
	DOES_NOT_MATCH
	LESS_THAN
	GREATER_THAN
	IS_PRESENT
	IS_NOT_PRESENT
)

func IsTypeAcceptable(key Key, t Type) bool {
	acceptableTypes := keyAcceptableTypes()
	if matchTypes, ok := acceptableTypes[key]; ok {
		for _, v := range matchTypes {
			if v == t {
				return true
			}
		}
	}

	return false
}

func keyAcceptableTypes() map[Key][]Type {
	return map[Key][]Type{
		TITLE:            {MATCHES, DOES_NOT_MATCH, IS_NOT_PRESENT, IS_PRESENT},
		RESOLUTION:       {MATCHES, DOES_NOT_MATCH, IS_NOT_PRESENT, IS_PRESENT},
		SEASON_NUMBER:    {EQUALS, NOT_EQUALS, LESS_THAN, GREATER_THAN, IS_NOT_PRESENT, IS_PRESENT},
		EPISODE_NUMBER:   {EQUALS, NOT_EQUALS, LESS_THAN, GREATER_THAN, IS_NOT_PRESENT, IS_PRESENT},
		SOURCE_PATH:      {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
		SOURCE_NAME:      {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
		SOURCE_EXTENSION: {MATCHES, DOES_NOT_MATCH, IS_PRESENT, IS_NOT_PRESENT},
	}
}

func (e Type) Values() []string {
	return []string{"EQUALS", "NOT_EQUALS", "MATCHES", "DOES_NOT_MATCH", "LESS_THAN", "GREATER_THAN", "IS_PRESENT", "IS_NOT_PRESENT"}
}

func (e Type) String() string {
	return e.Values()[e]
}

type CombineType int

const (
	AND CombineType = iota
	OR
)

func (e CombineType) Values() []string {
	return []string{"AND", "OR"}
}

func (e CombineType) String() string {
	return e.Values()[e]
}
