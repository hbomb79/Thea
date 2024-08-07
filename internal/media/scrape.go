package media

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hbomb79/Thea/internal/ffmpeg"
)

type (
	FileMediaMetadata struct {
		Title         string
		Episodic      bool
		SeasonNumber  int
		EpisodeNumber int
		Runtime       string
		Year          int
		FrameW        int
		FrameH        int
		Path          string
	}

	ScraperConfig struct {
		FfprobeBinPath string
	}

	MetadataScraper struct {
		config ScraperConfig
	}
)

func NewScraper(config ScraperConfig) *MetadataScraper {
	return &MetadataScraper{config: config}
}

// ScrapeFileForMediaInfo accepts a file path and tries to extract
// some standard metadata from it for the purpose of later searching
// third-party services.
//
// This function will first extract as much information as it can from the
// title (such as the title and episode/season information), and also
// uses ffprobe information for bitrate/duration.
func (scraper *MetadataScraper) ScrapeFileForMediaInfo(path string) (*FileMediaMetadata, error) {
	output := FileMediaMetadata{
		SeasonNumber:  -1,
		EpisodeNumber: -1,
		Path:          path,
	}

	// Extract information from title
	filename := filepath.Base(path)
	if err := scraper.extractTitleInformation(filename, &output); err != nil {
		return nil, err
	}

	// Use ffprobe to extract reliable information, such as frame width/height and bitrate
	if err := scraper.extractFfprobeInformation(path, &output); err != nil {
		return nil, err
	}

	return &output, nil
}

// extractTitleInformation uses regular expressions to try and find:
// - Title
// - Year
// - Is episode or movie?
// - Season/episode information.
func (scraper *MetadataScraper) extractTitleInformation(title string, output *FileMediaMetadata) error {
	normaliserMatcher := regexp.MustCompile(`(?i)[\.\s\-]`)
	seasonMatcher := regexp.MustCompile(`(?i)^(.*?)\s?s(\d+)\s?e(\d+)\s*((?:20|19)\d{2})?`)
	movieMatcher := regexp.MustCompile(`(?i)^(.+?)\s*((?:20|19)\d{2})`)

	normalizedTitle := normaliserMatcher.ReplaceAllString(title, " ")

	// Search for season info and optional year information
	if seasonGroups := seasonMatcher.FindStringSubmatch(normalizedTitle); len(seasonGroups) >= 1 {
		output.Episodic = true
		output.Title = strings.TrimSpace(seasonGroups[1])
		output.SeasonNumber = convertToInt(seasonGroups[2])
		output.EpisodeNumber = convertToInt(seasonGroups[3])
		year := convertToInt(seasonGroups[4])
		output.Year = year

		return nil
	}

	// Try find if it's a movie instead
	if movieGroups := movieMatcher.FindStringSubmatch(normalizedTitle); len(movieGroups) >= 1 {
		output.Episodic = false
		output.Title = strings.TrimSpace(movieGroups[1])
		output.SeasonNumber = -1
		output.EpisodeNumber = -1
		year := convertToInt(movieGroups[2])
		output.Year = year

		return nil
	}

	// Didn't match either case; return error so that trouble
	// can be raised by the worker.
	return errors.New("failed to extract file metadata from title - regular expressions failed")
}

// extractFfprobeInformation will read the media metadata using ffprobe. If successful,
// the frame width/height and the runtime of the media will be populated in the output.
func (scraper *MetadataScraper) extractFfprobeInformation(path string, output *FileMediaMetadata) error {
	metadata, err := ffmpeg.ProbeFile(path, scraper.config.FfprobeBinPath)
	if err != nil {
		return ffmpeg.ParseFfmpegError(err)
	}

	// TODO Consider revising how we select the stream
	streams := metadata.GetStreams()
	stream := streams[0]
	width := stream.GetWidth()
	height := stream.GetHeight()

	output.FrameW = width
	output.FrameH = height
	output.Runtime = metadata.GetFormat().GetDuration()

	return nil
}

// convertToInt is a helper method that accepts
// a string input and will attempt to convert that string
// to an integer - if it fails, -1 is returned.
func convertToInt(input string) int {
	v, err := strconv.Atoi(input)
	if err != nil {
		return -1
	}

	return v
}

func (m FileMediaMetadata) String() string {
	return fmt.Sprintf(`FileMediaMetadata {
		Title = %s,
		Episodic? = %v (season = %+v, episode = %+v),
		Runtime = %v,
		Year = %+v,
		Resolution = %+v x %+v,
		Path = %s,
	}`, m.Title, m.Episodic, m.SeasonNumber, m.EpisodeNumber, m.Runtime, m.Year, m.FrameW, m.FrameH, m.Path)
}
