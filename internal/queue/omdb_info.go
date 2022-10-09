package queue

import (
	"strings"

	"gorm.io/gorm"
)

// OmdbInfo is used as an unmarshaller target for JSON. It's embedded
// inside the QueueItem to allow us to use the information to generate
// a file structure, and also to store the information inside
// of a cache file or a database.
type OmdbInfo struct {
	gorm.Model
	QueueItemID uint
	Genre       StringList `decode:"string" mapstructure:"-" gorm:"-"`
	Title       string
	Description string `json:"plot"`
	ReleaseYear int
	Runtime     string
	ImdbId      string
	Type        string
	PosterUrl   string           `json:"poster"`
	Response    OmdbResponseType `decode:"bool" gorm:"-"`
	Error       string           `gorm:"-"`
}

type StringList []string
type OmdbResponseType bool

// UnmarshalJSON on StringList will unmarshal the data provided by
// removing the surrounding quotes and splitting the provided
// information in to a slice (comma-separated)
func (sl *StringList) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)

	list := strings.Split(t, ", ")
	*sl = append(*sl, list...)

	return nil
}

func (sl *StringList) ToGenreList() []*Genre {
	out := make([]*Genre, 0)
	for _, e := range *sl {
		out = append(out, &Genre{Name: e})
	}

	return out
}

// UnmarshalJSON on OmdbResponseType converts the given string
// from OMDB in to a golang boolean - this method is required
// because the response from OMDB is not a JSON-bool as it's
// capitalised
func (rt *OmdbResponseType) UnmarshalJSON(data []byte) error {
	t := trimQuotesFromByteSlice(data)
	switch t {
	case "True":
		*rt = true
	case "False":
	default:
		*rt = false
	}

	return nil
}

// Responses from OMDB come packaged in quotes; trimQuotesFromByteSlice is
// used to remove the surrounding quotes from the provided byte slice
// and any remaining whitespace is trimmed off. The altered string is then
// returned to the caller
func trimQuotesFromByteSlice(data []byte) string {
	strData := string(data)
	if len(strData) >= 2 && strData[0] == '"' && strData[len(strData)-1] == '"' {
		strData = strData[1 : len(strData)-1]
	}

	return strings.TrimSpace(strData)
}
