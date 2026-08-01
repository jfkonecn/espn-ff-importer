package podcast

import (
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"
)

const defaultPodcastSiteURL = "https://jfkonecn.github.io/espn-ff-importer"

type FeedEpisode struct {
	Title         string
	FileName      string
	AbsoluteURL   string
	FileSizeBytes int64
	AudioType     string
	PubDate       string
	GUID          string
	Description   string
	Duration      string
	Explicit      string
	EpisodeType   string
}

type FeedData struct {
	Channel   PodcastChannel
	Link      string
	FeedURL   string
	BuildDate string
	Episodes  []FeedEpisode
}

func GeneratePodcastFeed(outputPath, podcastDir, metadataPath, siteURL string) error {
	if siteURL == "" {
		siteURL = defaultPodcastSiteURL
	}

	metadata, err := LoadPodcastMetadata(metadataPath)
	if err != nil {
		return err
	}
	metadata.Channel = metadata.Channel.withDefaults(siteURL)

	episodes, err := scanPodcastEpisodes(podcastDir, metadata, siteURL)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	feedURL := strings.TrimRight(siteURL, "/") + "/podcasts.xml"
	pageURL := strings.TrimRight(siteURL, "/") + "/podcasts.html"
	data := FeedData{
		Channel:   metadata.Channel,
		Link:      pageURL,
		FeedURL:   feedURL,
		BuildDate: time.Now().Format(time.RFC1123Z),
		Episodes:  episodes,
	}

	tmpl, err := texttemplate.New("podcasts.xml").Funcs(texttemplate.FuncMap{
		"xml":        template.HTMLEscapeString,
		"formatBool": formatBool,
	}).Parse(podcastRSSTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(file, data)
}

func LoadPodcastMetadata(path string) (PodcastMetadata, error) {
	metadata := PodcastMetadata{Channel: defaultPodcastChannel()}
	if err := ReadJSON(path, &metadata); err != nil {
		if os.IsNotExist(err) {
			return metadata, nil
		}
		return PodcastMetadata{}, err
	}
	return metadata, nil
}

func scanPodcastEpisodes(podcastDir string, metadata PodcastMetadata, siteURL string) ([]FeedEpisode, error) {
	entries, err := os.ReadDir(podcastDir)
	if err != nil {
		return nil, err
	}
	episodesByFile := make(map[string]PodcastEpisodeMetadata, len(metadata.Episodes))
	for _, episode := range metadata.Episodes {
		if episode.File != "" {
			episodesByFile[episode.File] = episode
		}
	}

	var episodes []FeedEpisode
	for _, entry := range entries {
		if entry.IsDir() || !isPodcastAudioFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		fileName := entry.Name()
		metadata := episodesByFile[fileName]
		title := metadata.Title
		if title == "" {
			title = titleFromAudioFile(fileName)
		}
		description := metadata.Description
		if description == "" {
			description = "Fantasy football league podcast"
		}
		pubDate := info.ModTime().Format(time.RFC1123Z)
		if metadata.PubDate != "" {
			if parsed, err := time.Parse(time.RFC3339, metadata.PubDate); err == nil {
				pubDate = parsed.Format(time.RFC1123Z)
			}
		}

		episodes = append(episodes, FeedEpisode{
			Title:         title,
			FileName:      fileName,
			AbsoluteURL:   absolutePodcastURL(siteURL, fileName),
			FileSizeBytes: info.Size(),
			AudioType:     podcastAudioType(fileName),
			PubDate:       pubDate,
			GUID:          absolutePodcastURL(siteURL, fileName),
			Description:   description,
			Duration:      metadata.Duration,
			Explicit:      formatBool(metadata.Explicit),
			EpisodeType:   firstNonEmpty(metadata.EpisodeType, "full"),
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].FileName > episodes[j].FileName
	})
	return episodes, nil
}

func absolutePodcastURL(siteURL, fileName string) string {
	return strings.TrimRight(siteURL, "/") + "/assets/podcasts/" + url.PathEscape(filepath.ToSlash(fileName))
}

func titleFromAudioFile(fileName string) string {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	parts := strings.Split(name, "_")
	if len(parts) >= 2 && len(parts[0]) == 4 {
		return parts[0] + " " + strings.Join(parts[1:], " ")
	}
	return name
}

func defaultPodcastChannel() PodcastChannel {
	return PodcastChannel{
		Title:       "Self Aware Fantasy Podcast",
		Description: "Fantasy football league podcasts and analysis.",
		Author:      "Self Aware Fantasy Podcast",
		Language:    "en-us",
		Explicit:    false,
		Category:    "Sports",
		OwnerName:   "Self Aware Fantasy Podcast",
		Copyright:   "Self Aware Fantasy Podcast",
	}
}

func (pc PodcastChannel) withDefaults(siteURL string) PodcastChannel {
	defaults := defaultPodcastChannel()
	pc.Title = firstNonEmpty(pc.Title, defaults.Title)
	pc.Description = firstNonEmpty(pc.Description, defaults.Description)
	pc.Author = firstNonEmpty(pc.Author, defaults.Author)
	pc.Language = firstNonEmpty(pc.Language, defaults.Language)
	pc.Category = firstNonEmpty(pc.Category, defaults.Category)
	pc.OwnerName = firstNonEmpty(pc.OwnerName, defaults.OwnerName)
	pc.Copyright = firstNonEmpty(pc.Copyright, defaults.Copyright)
	if pc.Image != "" && !strings.HasPrefix(pc.Image, "http://") && !strings.HasPrefix(pc.Image, "https://") {
		pc.Image = strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(pc.Image, "/")
	}
	return pc
}

func isPodcastAudioFile(fileName string) bool {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".mp3", ".wav":
		return true
	default:
		return false
	}
}

func podcastAudioType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

const podcastRSSTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>{{xml .Channel.Title}}</title>
    <link>{{.Link}}</link>
    <description>{{xml .Channel.Description}}</description>
    <language>{{xml .Channel.Language}}</language>
    <copyright>{{xml .Channel.Copyright}}</copyright>
    <lastBuildDate>{{.BuildDate}}</lastBuildDate>
    <atom:link href="{{.FeedURL}}" rel="self" type="application/rss+xml" />
    <itunes:author>{{xml .Channel.Author}}</itunes:author>
    <itunes:summary>{{xml .Channel.Description}}</itunes:summary>
    <itunes:explicit>{{formatBool .Channel.Explicit}}</itunes:explicit>
    {{if .Channel.Image}}<itunes:image href="{{.Channel.Image}}" />{{end}}
    {{if .Channel.OwnerEmail}}<itunes:owner>
      <itunes:name>{{xml .Channel.OwnerName}}</itunes:name>
      <itunes:email>{{xml .Channel.OwnerEmail}}</itunes:email>
    </itunes:owner>{{end}}
    {{if .Channel.Subcategory}}<itunes:category text="{{xml .Channel.Category}}">
      <itunes:category text="{{xml .Channel.Subcategory}}" />
    </itunes:category>{{else}}<itunes:category text="{{xml .Channel.Category}}" />{{end}}
    {{range .Episodes}}
    <item>
      <title>{{xml .Title}}</title>
      <description>{{xml .Description}}</description>
      <itunes:summary>{{xml .Description}}</itunes:summary>
      <pubDate>{{.PubDate}}</pubDate>
      <guid isPermaLink="false">{{.GUID}}</guid>
      <enclosure url="{{.AbsoluteURL}}" length="{{.FileSizeBytes}}" type="{{.AudioType}}" />
      {{if .Duration}}<itunes:duration>{{.Duration}}</itunes:duration>{{end}}
      <itunes:explicit>{{.Explicit}}</itunes:explicit>
      <itunes:episodeType>{{.EpisodeType}}</itunes:episodeType>
    </item>
    {{end}}
  </channel>
</rss>
`
