package queue

import (
	"fmt"
	"path/filepath"
	"strconv"

	"gorm.io/gorm"
)

// TitleInfo contains the information about the import QueueItem
// that is gleamed from the pathname given; such as the title and
// if the show is an episode or a movie.
type TitleInfo struct {
	gorm.Model
	QueueItemID uint
	Title       string
	Episodic    bool
	Season      int
	Episode     int
	Year        int
	Resolution  string
}

// OutputPath is a method to calculate the path to which this
// item should be output to - based on the TitleInformation
func (tInfo *TitleInfo) OutputPath() string {
	if tInfo.Episodic {
		fName := fmt.Sprintf("%v_%v_%v_%v_%v", tInfo.Episode, tInfo.Season, tInfo.Title, tInfo.Resolution, tInfo.Year)
		return filepath.Join(tInfo.Title, fmt.Sprint(tInfo.Season), fName)
	}

	return fmt.Sprintf("%v_%v_%v", tInfo.Title, tInfo.Resolution, tInfo.Year)
}

type TitleFormatError struct {
	item    *Item
	message string
}

func (e TitleFormatError) Error() string {
	return fmt.Sprintf("failed to format title(%v) - %v", e.item.Name, e.message)
}

// convertToInt is a helper method that accepts
// a string input and will attempt to convert that string
// to an integer - if it fails, -1 is returned
func convertToInt(input string) int {
	v, err := strconv.Atoi(input)
	if err != nil {
		return -1
	}

	return v
}
