package export

import (
	"github.com/hbomb79/Thea/internal/db"
	"gorm.io/gorm"
)

func init() {
	db.DB.RegisterModel(&ExportedItem{}, &ExportDetail{}, &Series{}, &Genre{})
}

type ExportedItemDto struct {
	Name          *string         `json:"name"`
	Runtime       *string         `json:"runtime"`
	ReleaseYear   *int            `json:"release_year"`
	Description   *string         `json:"description"`
	Image         *string         `json:"image_url"`
	GenreIDs      []*uint         `json:"genre_ids"`
	SeriesID      *uint           `json:"series_id"`
	EpisodeNumber *uint           `json:"episode_number"`
	SeasonNumber  *uint           `json:"season_number"`
	Exports       []*ExportDetail `json:"exports"`
}

func NewExportedItemDto() *ExportedItemDto {
	return &ExportedItemDto{}
}

// ExportedItem represents a queue.Item that has been successfully completed and "exported". The
// data stored here is simply the fundamental information for the item.
type ExportedItem struct {
	*gorm.Model
	Name          string
	Description   string
	Runtime       string
	ReleaseYear   int
	Image         string
	Genres        []*Genre `gorm:"many2many:item_genres;"`
	Exports       []*ExportDetail
	EpisodeNumber *int
	SeasonNumber  *int
	SeriesID      *uint
	Series        *Series
}

// ExportDetail exists alongside ExportedItem, in a many-to-one relationship (an ExportedItem has many ExportDetail). These
// represent a particular FFmpeg export
type ExportDetail struct {
	*gorm.Model
	ExportedItemID uint
	Name           string
	Path           string `gorm:"uniqueIndex"`
}

// Series represents a way for multiple ExportedItems to group themselves under one "Series". Note that
// this is not the same as a "Season".
type Series struct {
	*gorm.Model
	Name string
}

// We store genres in their own table using this struct - this allows us to view all Genres we know about by consulting this
// table, rather than using the ExportedItem table.
type Genre struct {
	Name string `gorm:"primaryKey"`
}
