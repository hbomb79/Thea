package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// toArgsMap takes a given struct and will go through all
// fields of the provided input and create an output map where
// each key is the name of the field, and each value is a string
// representation of the type of the field (e.g. string, int, bool)
func toArgsMap(in interface{}) (map[string]string, error) {
	out := make(map[string]string)

	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("toArgsMap only accepts structs - got %T", v)
	}

	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		var typeName string

		fi := typ.Field(i)
		if v, ok := fi.Tag.Lookup("decode"); ok {
			if v == "-" {
				// Field wants to be ignored
				continue
			}

			// Field has a tag to specify the decode type. Use that instead
			typeName = v
		} else {
			// Use actual type name
			typeName = fi.Type.Name()
		}

		out[fi.Name] = typeName
	}

	return out, nil
}

type TroubleType = int

const (
	TITLE_FAILURE TroubleType = iota
	OMDB_NO_RESULT_FAILURE
	OMDB_MULTIPLE_RESULT_FAILURE
	OMDB_REQUEST_FAILURE
	FFMPEG_FAILURE
)

type baseTaskError struct {
	message           string
	queueItem         *QueueItem
	troubleType       TroubleType
	resolutionContext map[string]interface{}
}

func NewBaseTaskError(message string, queueItem *QueueItem, troubleType TroubleType) baseTaskError {
	return baseTaskError{
		message,
		queueItem,
		troubleType,
		make(map[string]interface{}),
	}
}

func (base *baseTaskError) Error() string {
	return base.message
}

func (base *baseTaskError) Item() *QueueItem {
	return base.queueItem
}

func (base *baseTaskError) Args() map[string]string {
	return map[string]string{}
}

func (base *baseTaskError) Payload() map[string]interface{} {
	return nil
}

func (base *baseTaskError) Type() TroubleType {
	return base.troubleType
}

func (base *baseTaskError) ProvideResolutionContext(key string, ctx interface{}) {
	base.resolutionContext[key] = ctx
	base.queueItem.SetStatus(Pending)
}

func (base *baseTaskError) ResolutionContext() map[string]interface{} {
	return base.resolutionContext
}

func marshalToJson(trouble Trouble) ([]byte, error) {
	res := struct {
		Message      string                 `json:"message"`
		ExpectedArgs map[string]string      `json:"expected_args"`
		Type         int                    `json:"type"`
		Payload      map[string]interface{} `json:"payload"`
		ItemId       int                    `json:"item_id"`
	}{
		trouble.Error(),
		trouble.Args(),
		trouble.Type(),
		trouble.Payload(),
		trouble.Item().Id,
	}

	return json.Marshal(res)
}

// TitleTaskError is an error raised during the processing of the
// title task
type TitleTaskError struct {
	baseTaskError
}

// Args returns the arguments required to resolve this
// trouble
func (ex TitleTaskError) Args() map[string]string {
	v, err := toArgsMap(TitleInfo{})
	if err != nil {
		panic(err)
	}

	return v
}

// Resolve will attempt to resolve the error by taking the arguments provided
// to the method, and casting it to a TitleInfo struct if possible.
func (ex TitleTaskError) Resolve(args map[string]interface{}) error {
	var result TitleInfo
	err := mapstructure.WeakDecode(args, &result)
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Title) == "" {
		return errors.New("TitleInfo 'Title' cannot be empty!")
	}

	ex.ProvideResolutionContext("info", &result)
	return nil
}

func (ex *TitleTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

type OmdbTaskError struct {
	baseTaskError
	choices []*OmdbSearchItem
}

// Resolve will examine the provided arguments to this method and attempt to determine
// exactly what the user is trying to do, they could:
// - Provide a choice ID ('choiceId')
// - Provide an OMDB structure ('replacementStruct')
// - Provide an imdbId ('imdbId')
// - Provide a command ('action:<val>' where val is 'retry')
func (ex *OmdbTaskError) Resolve(args map[string]interface{}) error {
	if v, ok := args["imdbId"]; ok {
		ex.ProvideResolutionContext("fetchId", v)
		return nil
	} else if v, ok := args["replacementStruct"]; ok {
		if vStruct, ok := v.(OmdbInfo); ok {
			ex.ProvideResolutionContext("omdbStruct", vStruct)
			return nil
		}

		return errors.New("Unable to resovle OMDB task error - 'replacementStruct' paramater provided is not a valid OmdbInfo structure!")
	} else if v, ok := args["action"]; ok {
		if v == "retry" {
			ex.ProvideResolutionContext("action", v)
			return nil
		}

		return errors.New("Unable to resolve OMDB task error - 'action' value of '" + v.(string) + "' is invalid!")
	} else if v, ok := args["choiceId"]; ok {
		if ex.troubleType != OMDB_MULTIPLE_RESULT_FAILURE {
			return errors.New("Unable to resolve OMDB task error - 'choiceId' provided is illegal for this OMDB error")
		}

		vIdx, ok := v.(float64)
		if !ok {
			return errors.New("Unable to resolve OMDB task error - 'choiceId' is not a valid number!")
		}

		choiceIdx := int(vIdx)
		if choiceIdx < 0 || choiceIdx > len(ex.choices)-1 {
			return errors.New("Unable to resolve OMDB task error - 'choiceId' is out of range!")
		}

		ex.ProvideResolutionContext("fetchId", ex.choices[choiceIdx].ImdbId)
		return nil
	} else {
		return errors.New("Unable to resolve OMDB task error - arguments provided are invalid! One of 'imdbId, action, choiceId, replacementStruct' was expected.")
	}
}

// Args returns the arguments required to rebuild an OmdbInfo
// struct for use with the 'replacementStruct' paramater
// during a resolution
func (ex *OmdbTaskError) Args() map[string]string {
	v, err := toArgsMap(OmdbInfo{})
	if err != nil {
		panic(err)
	}

	return v
}

// Payload returns additional information for this trouble,
// for an OMDB_MULTIPLE_RESULT_FAILURE, it returns a map
// with one key (choices) with a value matching the 'choices'
// stored in this trouble. Any other trouble type will
// default to returning the baseTaskError Payload
func (ex *OmdbTaskError) Payload() map[string]interface{} {
	if ex.Type() != OMDB_MULTIPLE_RESULT_FAILURE {
		return ex.baseTaskError.Payload()
	}

	return map[string]interface{}{
		"choices": ex.choices,
	}
}

func (ex *OmdbTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

// FormatTaskError is an error/trouble type that is raised when ffmpeg/ffprobe encounters
// an error. The only real solution to this is to retry because an error of this type
// indicates that a glitch occurred, or that the input file is malformed.
type FormatTaskError struct {
	baseTaskError
}

// Resolve will attempt to resolve this trouble by resetting the queue items status
// and waking up any sleeping workers in the format worker pool. This essentially means
// that a worker will try this queue item again. Repeated failures likely means the input
// file is bad.
func (ex FormatTaskError) Resolve(map[string]interface{}) error {
	ex.queueItem.SetStatus(Pending)
	return nil
}

func (ex *FormatTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}
