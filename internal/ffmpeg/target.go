package ffmpeg

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/floostack/transcoder/ffmpeg"
	"github.com/google/uuid"
)

type (
	Target struct {
		ID    uuid.UUID `json:"id"`
		Label string    `json:"label"` // unique
		// NB: These JSON struct tags are important! It's used when unmarhsalling the JSON coalesced rows from the DB
		FfmpegOptions *Opts  `db:"ffmpeg_options" json:"ffmpeg_options"`
		Ext           string `db:"extension" json:"extension"`
	}

	Opts ffmpeg.Options
)

// Scan scan value into Jsonb, implements sql.Scanner interface
func (opts *Opts) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := Opts{}
	err := json.Unmarshal(bytes, &result)
	*opts = Opts(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (opts Opts) Value() (driver.Value, error) {
	return json.Marshal(opts)
}

func (opts Opts) GetStrArguments() []string {
	f := reflect.TypeOf(opts)
	v := reflect.ValueOf(opts)

	values := []string{}

	for i := 0; i < f.NumField(); i++ {
		flag := f.Field(i).Tag.Get("flag")
		value := v.Field(i).Interface()

		if !v.Field(i).IsNil() {
			if _, ok := value.(*bool); ok {
				values = append(values, flag)
			}

			if vs, ok := value.(*string); ok {
				values = append(values, flag, *vs)
			}

			if va, ok := value.([]string); ok {
				for i := 0; i < len(va); i++ {
					item := va[i]
					values = append(values, flag, item)
				}
			}

			if vm, ok := value.(map[string]interface{}); ok {
				for k, v := range vm {
					values = append(values, k, fmt.Sprintf("%v", v))
				}
			}

			if vi, ok := value.(*int); ok {
				values = append(values, flag, fmt.Sprintf("%d", *vi))
			}
		}
	}

	return values
}

func (target *Target) String() string {
	return fmt.Sprintf("Target{ID=%s Label=%s}", target.ID, target.Label)
}

func (target *Target) RequiredThreads() int { return 2 }
