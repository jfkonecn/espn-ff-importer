package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"espn-ff-importer/internal/podcast"
)

func main() {
	var (
		season       = flag.Int("season", seasonFromEnv(), "Season year")
		statePath    = flag.String("state", "", "Path to season-state JSON")
		aiDir        = flag.String("ai", "ai", "AI data directory")
		outputDir    = flag.String("output", "static/assets/podcasts/generated", "Directory for generated podcast scripts")
		podcastDir   = flag.String("podcasts", "static/assets/podcasts", "Podcast asset directory")
		model        = flag.String("model", getenv("PODCAST_MODEL", "gpt-4.1"), "OpenAI model for transcript generation")
		forceRun     = flag.Bool("force", boolFromEnv("PODCAST_FORCE_RUN"), "Allow overwriting an existing generated podcast")
		skipExisting = flag.Bool("skip-existing", boolFromEnv("PODCAST_SKIP_EXISTING"), "Exit successfully when a generated podcast already exists")
	)
	flag.Parse()
	if *season == 0 {
		fatal("determine season", fmt.Errorf("CURRENT_YEAR or -season is required"))
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fatal("validate environment", fmt.Errorf("OPENAI_API_KEY is required"))
	}

	path := *statePath
	if path == "" {
		path = filepath.Join(*aiDir, fmt.Sprintf("%d", *season), "season-state.json")
	}

	fmt.Printf("Reading podcast season state from %s\n", path)
	var state podcast.SeasonState
	if err := podcast.ReadJSON(path, &state); err != nil {
		fatal("read season state", err)
	}
	episodeID := podcast.DefaultEpisodeID(state)
	outputPath := filepath.Join(*outputDir, episodeID+".json")
	audioPath := filepath.Join(*podcastDir, episodeID+".mp3")
	if !*forceRun {
		if fileExists(outputPath) {
			if *skipExisting {
				fmt.Printf("Podcast transcript already exists at %s; skipping generation\n", outputPath)
				return
			}
			fatal("run safety check", fmt.Errorf("generated transcript already exists at %s; manually rerun with force enabled to overwrite it", outputPath))
		}
		if fileExists(audioPath) {
			if *skipExisting {
				fmt.Printf("Podcast audio already exists at %s; skipping generation\n", audioPath)
				return
			}
			fatal("run safety check", fmt.Errorf("generated podcast audio already exists at %s; manually rerun with force enabled to overwrite it", audioPath))
		}
	}
	fmt.Printf("Generating transcript for season=%d phase=%s week=%d using model=%s\n", state.Season, state.Phase, state.Week, *model)

	script, err := podcast.GenerateTranscript(apiKey, *model, *aiDir, state)
	if err != nil {
		fatal("generate transcript", err)
	}

	fmt.Printf("Writing transcript JSON for episode %s to %s\n", script.EpisodeID, outputPath)
	if err := podcast.WriteJSON(outputPath, script); err != nil {
		fatal("write transcript", err)
	}
	fmt.Printf("Transcript contains %d segments, %d selected commercials, and %d source files\n", len(script.Segments), len(script.Commercials), len(script.SourceFiles))
	fmt.Printf("Wrote podcast transcript to %s\n", outputPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func boolFromEnv(key string) bool {
	value := os.Getenv(key)
	return value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "YES"
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
