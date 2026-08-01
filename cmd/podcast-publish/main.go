package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"espn-ff-importer/internal/podcast"
)

func main() {
	var (
		season          = flag.Int("season", seasonFromEnv(), "Season year")
		scriptPath      = flag.String("script", "", "Generated podcast script JSON path")
		generatedDir    = flag.String("generated", "static/assets/podcasts/generated", "Generated script directory")
		podcastDir      = flag.String("podcasts", "static/assets/podcasts", "Podcast asset directory")
		metadataPath    = flag.String("metadata", "static/assets/podcasts/metadata.json", "Podcast metadata JSON path")
		introSound      = flag.String("intro-sound", "podcast-sounds/slop_take_intro.mp3", "Intro sound MP3 path")
		transitionSound = flag.String("transition-sound", "podcast-sounds/slop_take_transition.mp3", "Transition sound MP3 path")
		outroSound      = flag.String("outro-sound", "podcast-sounds/slop_take_outro.mp3", "Outro sound MP3 path")
		model           = flag.String("model", getenv("PODCAST_TTS_MODEL", "gpt-4o-mini-tts"), "OpenAI TTS model")
		voice           = flag.String("voice", getenv("PODCAST_VOICE", "ballad"), "OpenAI TTS voice")
		forceRun        = flag.Bool("force", boolFromEnv("PODCAST_FORCE_RUN"), "Allow overwriting an existing generated podcast")
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
	if !*forceRun && fileExists(audioPath) {
		fatal("run safety check", fmt.Errorf("generated podcast audio already exists at %s; manually rerun with force enabled to overwrite it", audioPath))
	}
	fmt.Printf("Synthesizing segmented MP3 for episode %s with model=%s voice=%s\n", script.EpisodeID, *model, *voice)
	if err := synthesizeSegmentedEpisode(apiKey, *model, *voice, script, audioPath, *introSound, *transitionSound, *outroSound, *generatedDir); err != nil {
		fatal("synthesize segmented audio", err)
	}
	if duration, err := probeDuration(audioPath); err == nil && duration != "" {
		script.Duration = duration
		fmt.Printf("Detected final podcast duration: %s\n", duration)
	} else if err != nil {
		fmt.Printf("Warning: could not determine final podcast duration: %v\n", err)
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

func synthesizeSegmentedEpisode(apiKey, model, voice string, script podcast.PodcastScript, outputPath, introSound, transitionSound, outroSound, generatedDir string) error {
	if err := requireFile(introSound); err != nil {
		return err
	}
	if err := requireFile(transitionSound); err != nil {
		return err
	}
	if err := requireFile(outroSound); err != nil {
		return err
	}

	partsDir, err := os.MkdirTemp(generatedDir, script.EpisodeID+"-audio-parts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(partsDir)
	fmt.Printf("Writing temporary audio parts to %s\n", partsDir)

	spokenParts := orderedSpokenParts(script)
	if len(spokenParts) == 0 {
		return fmt.Errorf("no segment or commercial text found for TTS")
	}

	concatParts := []string{introSound}
	for i, part := range spokenParts {
		partPath := filepath.Join(partsDir, fmt.Sprintf("%02d_%s.mp3", i+1, sanitizeFilePart(part.Name)))
		fmt.Printf("Synthesizing audio part %d/%d: %s (%d characters)\n", i+1, len(spokenParts), part.Name, len(part.Text))
		if err := podcast.SynthesizeSpeech(apiKey, model, voice, part.Text, partPath); err != nil {
			return fmt.Errorf("synthesize %s: %w", part.Name, err)
		}
		concatParts = append(concatParts, partPath)
		if i != len(spokenParts)-1 {
			concatParts = append(concatParts, transitionSound)
		}
	}
	concatParts = append(concatParts, outroSound)

	fmt.Printf("Combining %d audio parts with ffmpeg\n", len(concatParts))
	return concatenateMP3s(concatParts, outputPath, partsDir)
}

type spokenPart struct {
	Name string
	Text string
}

func orderedSpokenParts(script podcast.PodcastScript) []spokenPart {
	segments := make(map[string]string, len(script.Segments))
	for _, segment := range script.Segments {
		segments[strings.ToLower(segment.Name)] = segment.Transcript
	}

	var parts []spokenPart
	if text := segments["intro"]; text != "" {
		parts = append(parts, spokenPart{Name: "Intro", Text: text})
	}
	if len(script.Commercials) > 0 && script.Commercials[0].Read != "" {
		parts = append(parts, spokenPart{Name: "Commercial 1 - " + script.Commercials[0].CompanyName, Text: script.Commercials[0].Read})
	}
	if text := segments["best team"]; text != "" {
		parts = append(parts, spokenPart{Name: "Best Team", Text: text})
	}
	if text := segments["worst team"]; text != "" {
		parts = append(parts, spokenPart{Name: "Worst Team", Text: text})
	}
	if len(script.Commercials) > 1 && script.Commercials[1].Read != "" {
		parts = append(parts, spokenPart{Name: "Commercial 2 - " + script.Commercials[1].CompanyName, Text: script.Commercials[1].Read})
	}
	if text := segments["final take"]; text != "" {
		parts = append(parts, spokenPart{Name: "Final Take", Text: text})
	}

	if len(parts) > 0 {
		return parts
	}
	if script.Transcript != "" {
		return []spokenPart{{Name: "Full Transcript", Text: script.Transcript}}
	}
	return nil
}

func concatenateMP3s(parts []string, outputPath, workDir string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	listPath := filepath.Join(workDir, "concat.txt")
	var builder strings.Builder
	for _, part := range parts {
		abs, err := filepath.Abs(part)
		if err != nil {
			return err
		}
		builder.WriteString("file '")
		builder.WriteString(strings.ReplaceAll(abs, "'", "'\\''"))
		builder.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(builder.String()), 0644); err != nil {
		return err
	}

	tmpOutput := outputPath + ".tmp.mp3"
	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-codec:a", "libmp3lame", "-b:a", "128k", tmpOutput)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat failed: %w", err)
	}
	return os.Rename(tmpOutput, outputPath)
}

func probeDuration(path string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return "", err
	}
	total := int(seconds + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60), nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("required podcast sound %s is not available: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("required podcast sound %s is a directory", path)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func boolFromEnv(key string) bool {
	value := os.Getenv(key)
	return value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "YES"
}

func sanitizeFilePart(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 && builder.String()[builder.Len()-1] != '_' {
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "part"
	}
	return result
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
