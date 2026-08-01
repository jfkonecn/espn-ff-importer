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
		season    = flag.Int("season", seasonFromEnv(), "Season year")
		statePath = flag.String("state", "", "Path to season-state JSON")
		aiDir     = flag.String("ai", "ai", "AI data directory")
		outputDir = flag.String("output", "static/assets/podcasts/generated", "Directory for generated podcast scripts")
		model     = flag.String("model", getenv("PODCAST_MODEL", "gpt-4.1"), "OpenAI model for transcript generation")
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

	var state podcast.SeasonState
	if err := podcast.ReadJSON(path, &state); err != nil {
		fatal("read season state", err)
	}

	script, err := podcast.GenerateTranscript(apiKey, *model, *aiDir, state)
	if err != nil {
		fatal("generate transcript", err)
	}

	outputPath := filepath.Join(*outputDir, script.EpisodeID+".json")
	if err := podcast.WriteJSON(outputPath, script); err != nil {
		fatal("write transcript", err)
	}
	fmt.Printf("Wrote podcast transcript to %s\n", outputPath)
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
