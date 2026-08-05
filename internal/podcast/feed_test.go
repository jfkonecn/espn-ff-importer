package podcast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePodcastFeedIncludesChapters(t *testing.T) {
	tmpDir := t.TempDir()
	podcastDir := filepath.Join(tmpDir, "podcasts")
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(podcastDir, "episode.mp3"), []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	metadataPath := filepath.Join(tmpDir, "metadata.json")
	metadata := PodcastMetadata{
		Episodes: []PodcastEpisodeMetadata{
			{
				File:     "episode.mp3",
				Title:    "Episode",
				Chapters: "episode.chapters.json",
			},
		},
	}
	if err := WriteJSON(metadataPath, metadata); err != nil {
		t.Fatal(err)
	}

	feedPath := filepath.Join(tmpDir, "podcasts.xml")
	if err := GeneratePodcastFeed(feedPath, podcastDir, metadataPath, "https://example.com/show"); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatal(err)
	}
	feed := string(contents)
	if !strings.Contains(feed, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`) {
		t.Fatalf("feed missing podcast namespace: %s", feed)
	}
	if !strings.Contains(feed, `<podcast:chapters url="https://example.com/show/assets/podcasts/episode.chapters.json" type="application/json" />`) {
		t.Fatalf("feed missing chapters tag: %s", feed)
	}
}
