package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"espn-ff-importer/internal/podcast"
)

func main() {
	var (
		season       = flag.Int("season", seasonFromEnv(), "Season year")
		scriptPath   = flag.String("script", "", "Generated podcast script JSON path")
		generatedDir = flag.String("generated", "static/assets/podcasts/generated", "Generated script directory")
		podcastDir   = flag.String("podcasts", "static/assets/podcasts", "Podcast asset directory")
		metadataPath = flag.String("metadata", "static/assets/podcasts/metadata.json", "Podcast metadata JSON path")
		model        = flag.String("model", getenv("PODCAST_TTS_MODEL", "gpt-4o-mini-tts"), "OpenAI TTS model")
		voice        = flag.String("voice", getenv("PODCAST_VOICE", "ballad"), "OpenAI TTS voice")
	)
	flag.Parse()
	if *season == 0 {
		fatal("determine season", fmt.Errorf("CURRENT_YEAR or -season is required"))
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fatal("validate environment", fmt.Errorf("OPENAI_API_KEY is required"))
	}

	path := *scriptPath
	if path == "" {
		fmt.Printf("Looking for latest generated script for season %d in %s\n", *season, *generatedDir)
		path = latestScriptPath(*generatedDir, *season)
	}
	if path == "" {
		fatal("find generated script", fmt.Errorf("no generated podcast script found"))
	}

	fmt.Printf("Reading generated podcast script from %s\n", path)
	var script podcast.PodcastScript
	if err := podcast.ReadJSON(path, &script); err != nil {
		fatal("read generated script", err)
	}
	if script.AudioFile == "" {
		script.AudioFile = script.EpisodeID + ".mp3"
	}

	audioPath := filepath.Join(*podcastDir, script.AudioFile)
	fmt.Printf("Synthesizing MP3 for episode %s with model=%s voice=%s\n", script.EpisodeID, *model, *voice)
	if err := podcast.SynthesizeSpeech(apiKey, *model, *voice, script.Transcript, audioPath); err != nil {
		fatal("synthesize audio", err)
	}

	fmt.Printf("Updating podcast metadata at %s\n", *metadataPath)
	if err := upsertMetadata(*metadataPath, script); err != nil {
		fatal("update podcast metadata", err)
	}

	fmt.Println("Regenerating podcast page and RSS feed")
	if err := runAnalyzer(); err != nil {
		fatal("regenerate podcast feed", err)
	}

	fmt.Printf("Published podcast audio to %s\n", audioPath)
}

func seasonFromEnv() int {
	value := os.Getenv("CURRENT_YEAR")
	if value == "" {
		return 0
	}
	season, err := strconv.Atoi(value)
	if err != nil {
		fatal("parse CURRENT_YEAR", err)
	}
	return season
}

func latestScriptPath(dir string, season int) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var newest string
	var newestTime time.Time
	prefix := fmt.Sprintf("%d_", season)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if prefix != "" && len(entry.Name()) >= len(prefix) && entry.Name()[:len(prefix)] != prefix {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = filepath.Join(dir, entry.Name())
			newestTime = info.ModTime()
		}
	}
	return newest
}

func upsertMetadata(path string, script podcast.PodcastScript) error {
	metadata := podcast.PodcastMetadata{}
	if err := podcast.ReadJSON(path, &metadata); err != nil && !os.IsNotExist(err) {
		return err
	}

	episode := podcast.PodcastEpisodeMetadata{
		File:        script.AudioFile,
		Title:       script.Title,
		Description: firstNonEmpty(script.Description, script.Summary),
		PubDate:     script.PubDate,
		Duration:    script.Duration,
		Explicit:    script.Explicit,
		EpisodeType: firstNonEmpty(script.EpisodeType, "full"),
	}

	updated := false
	for i := range metadata.Episodes {
		if metadata.Episodes[i].File == episode.File {
			metadata.Episodes[i] = episode
			updated = true
			break
		}
	}
	if !updated {
		metadata.Episodes = append([]podcast.PodcastEpisodeMetadata{episode}, metadata.Episodes...)
		fmt.Printf("Added metadata entry for %s\n", episode.File)
	} else {
		fmt.Printf("Updated existing metadata entry for %s\n", episode.File)
	}

	return podcast.WriteJSON(path, metadata)
}

func runAnalyzer() error {
	cmd := exec.Command("go", "run", "./src", "-data", "data", "-output", "static")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "Failed to %s: %v\n", action, err)
	os.Exit(1)
}
