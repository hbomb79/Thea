package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/profile"
	"github.com/mitchellh/mapstructure"
)

// When a processor task encounters an error that requires
// user intervention to continue - a 'trouble' is raised.
// This trouble is raised, and resolved, via the 'Trouble'
// struct. This struct mainly acts as a way for the
// task to continue working on other items whilst
// keeping track of the trouble(s) that are pending
type Trouble interface {
	error
	Args() map[string]string
	Resolve(map[string]interface{}) error
	Item() *Item
	Type() TroubleType
	Payload() map[string]interface{}
	ResolutionContext() map[string]interface{}
	Uuid() *uuid.UUID
}

type TroubleType = int

const (
	TITLE_FAILURE TroubleType = iota
	OMDB_NO_RESULT_FAILURE
	OMDB_MULTIPLE_RESULT_FAILURE
	OMDB_REQUEST_FAILURE
	FFMPEG_FAILURE
	COMMANDER_FAILURE
	DATABASE_FAILURE
)

type BaseTaskError struct {
	message           string
	queueItem         *Item
	troubleType       TroubleType
	resolutionContext map[string]interface{}
	uuid              uuid.UUID
}

func NewBaseTaskError(message string, queueItem *Item, troubleType TroubleType) BaseTaskError {
	return BaseTaskError{
		message,
		queueItem,
		troubleType,
		make(map[string]interface{}),
		uuid.New(),
	}
}

func (base *BaseTaskError) Error() string {
	return base.message
}

func (base *BaseTaskError) Item() *Item {
	return base.queueItem
}

func (base *BaseTaskError) Args() map[string]string {
	return map[string]string{}
}

func (base *BaseTaskError) Payload() map[string]interface{} {
	return nil
}

func (base *BaseTaskError) Type() TroubleType {
	return base.troubleType
}

func (base *BaseTaskError) ProvideResolutionContext(key string, ctx interface{}) {
	base.resolutionContext[key] = ctx
	base.queueItem.SetStatus(Pending)
}

func (base *BaseTaskError) ResolutionContext() map[string]interface{} {
	return base.resolutionContext
}

func (base *BaseTaskError) Uuid() *uuid.UUID {
	return &base.uuid
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
		trouble.Item().ItemID,
	}

	return json.Marshal(res)
}

// TitleTaskError is an error raised during the processing of the
// title task
type TitleTaskError struct {
	BaseTaskError
}

// Args returns the arguments required to resolve this
// trouble
func (ex *TitleTaskError) Args() map[string]string {
	v, err := profile.ToArgsMap(TitleInfo{})
	if err != nil {
		panic(err)
	}

	return v
}

// Resolve will attempt to resolve the error by taking the arguments provided
// to the method, and casting it to a TitleInfo struct if possible.
func (ex *TitleTaskError) Resolve(args map[string]interface{}) error {
	var result TitleInfo
	err := mapstructure.WeakDecode(args, &result)
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Title) == "" {
		return errors.New("failed to resolve TitleTaskError - TitleInfo 'Title' property cannot be empty")
	}

	ex.ProvideResolutionContext("info", &result)
	return nil
}

func (ex *TitleTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

type OmdbTaskError struct {
	BaseTaskError
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
		var vStruct OmdbInfo
		if err := mapstructure.Decode(v, &vStruct); err != nil {
			return fmt.Errorf("unable to resovle OMDB task error - %v", err.Error())
		}

		vStruct.Genre = strings.Split((v.(map[string]interface{}))["Genre"].(string), ",")
		ex.ProvideResolutionContext("omdbStruct", vStruct)

		return nil
	} else if v, ok := args["action"]; ok {
		if v == "retry" {
			ex.ProvideResolutionContext("action", v)
			return nil
		}

		return fmt.Errorf("unable to resolve OMDB task error - 'action' value '%v' is invalid", v)
	} else if v, ok := args["choiceId"]; ok {
		if ex.troubleType != OMDB_MULTIPLE_RESULT_FAILURE {
			return errors.New("unable to resolve OMDB task error - 'choiceId' provided is illegal for this OMDB error")
		}

		vIdx, ok := v.(float64)
		if !ok {
			return errors.New("unable to resolve OMDB task error - 'choiceId' is not a valid number")
		}

		choiceIdx := int(vIdx)
		if choiceIdx < 0 || choiceIdx > len(ex.choices)-1 {
			return errors.New("unable to resolve OMDB task error - 'choiceId' is out of range")
		}

		ex.ProvideResolutionContext("fetchId", ex.choices[choiceIdx].ImdbId)
		return nil
	} else {
		return errors.New("unable to resolve OMDB task error - arguments provided are invalid! One of 'imdbId, action, choiceId, replacementStruct' was expected")
	}
}

// Args returns the arguments required to rebuild an OmdbInfo
// struct for use with the 'replacementStruct' paramater
// during a resolution
func (ex *OmdbTaskError) Args() map[string]string {
	v, err := profile.ToArgsMap(OmdbInfo{})
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
		return ex.BaseTaskError.Payload()
	}

	return map[string]interface{}{
		"choices": ex.choices,
	}
}

func (ex *OmdbTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

type FfmpegTaskError struct {
	BaseTaskError
	// taskInstance CommanderTask
}

func (ex *FfmpegTaskError) Resolve(args map[string]interface{}) error {
	if _, ok := args["retry"]; ok {
		ex.ProvideResolutionContext("retry", true)
		return nil
	} else if _, ok := args["cancel"]; ok {
		ex.ProvideResolutionContext("cancel", true)
		return nil
	}

	return nil
}

func (ex *FfmpegTaskError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

type ProfileSelectionError struct {
	BaseTaskError
}

func (ex *ProfileSelectionError) Resolve(args map[string]interface{}) error {
	return errors.New("there is no explicit resolution for this trouble - please add/correct your Thea profiles, or cancel this item, and this trouble will automatically resolve")
}

func (ex *ProfileSelectionError) MarshalJSON() ([]byte, error) {
	return marshalToJson(ex)
}

type DatabaseTaskError struct {
	BaseTaskError
}

func (ex *DatabaseTaskError) Resolve(args map[string]interface{}) error {
	if _, ok := args["retry"]; ok {
		ex.ProvideResolutionContext("retry", true)
		return nil
	}

	return errors.New("unable to resolve DatabaseTaskError - arguments provided are invalid! 'retry' was expected")
}
