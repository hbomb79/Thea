package media

type MetadataScraper struct{}

type FileMediaMetadata struct {
	Title         string
	Episodic      bool
	SeasonNumber  int
	EpisodeNumber int
	FrameW        *int
	FrameH        *int
	Bitrate       *int
}

func (scraper *MetadataScraper) ScrapeFileForMediaInfo(path string) *FileMediaMetadata {
	return nil
}
