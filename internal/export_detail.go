package internal

import "gorm.io/gorm"

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

type ExportDetail struct {
	*gorm.Model
	ExportedItemID uint
	Name           string
	Path           string `gorm:"uniqueIndex"`
}

type Series struct {
	*gorm.Model
	Name string
}

type Genre struct {
	Name string `gorm:"primaryKey"`
}
