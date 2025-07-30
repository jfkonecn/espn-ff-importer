package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Define command line flags
	var (
		dataDir = flag.String("data", "data", "Directory containing ESPN league JSON files")
		output  = flag.String("output", "static", "Output directory for static website")
	)

	flag.Parse()

	// Create output directory
	if err := os.MkdirAll(*output, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Find all JSON files in the data directory
	files, err := filepath.Glob(filepath.Join(*dataDir, "espn_league_*.json"))
	if err != nil {
		fmt.Printf("Error finding JSON files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Printf("No JSON files found in %s\n", *dataDir)
		os.Exit(1)
	}

	// Process each season file
	var seasons []SeasonInfo
	for _, file := range files {
		seasonInfo, err := processSeasonFile(file, *output, *dataDir)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", file, err)
			continue
		}
		seasons = append(seasons, seasonInfo)
	}

	// Generate the main index page
	if err := generateIndexPage(seasons, *output); err != nil {
		fmt.Printf("Error generating index page: %v\n", err)
		os.Exit(1)
	}

	// Generate the podcasts page
	if err := generatePodcastsPage(*output); err != nil {
		fmt.Printf("Error generating podcasts page: %v\n", err)
		os.Exit(1)
	}

	// Generate AI data files
	if err := generateAIData(files, *dataDir); err != nil {
		fmt.Printf("Error generating AI data: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Static website generated successfully in: %s\n", *output)
	fmt.Printf("AI data generated successfully for all seasons in: ai/\n")
	fmt.Printf("Processed %d seasons\n", len(seasons))
}

// SeasonInfo represents information about a season for the index page
type SeasonInfo struct {
	Year        string
	LeagueName  string
	TeamCount   int
	LastUpdated string
	FileName    string
	HasDraft    bool
}

// processSeasonFile processes a single season file and generates its HTML page
func processSeasonFile(filePath, outputDir, dataDir string) (SeasonInfo, error) {
	// Extract year from filename (e.g., "espn_league_2024.json" -> "2024")
	baseName := filepath.Base(filePath)
	year := strings.TrimSuffix(strings.TrimPrefix(baseName, "espn_league_"), ".json")
	
	// Create league reader
	reader, err := NewLeagueReader(filePath)
	if err != nil {
		return SeasonInfo{}, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Create website generator
	generator := NewWebsiteGenerator(reader)

	// Load historical data for keeper analysis
	if err := generator.LoadHistoricalData(dataDir); err != nil {
		// Log warning but don't fail - historical data is optional
		fmt.Printf("Warning: Could not load historical data: %v\n", err)
	}

	// Generate the season page
	outputFile := filepath.Join(outputDir, fmt.Sprintf("season-%s.html", year))
	if err := generator.GenerateSeasonPage(outputFile); err != nil {
		return SeasonInfo{}, fmt.Errorf("failed to generate season page: %w", err)
	}

	// Check if draft data exists and generate draft page
	league := reader.GetLeague()
	hasDraft := len(league.DraftDetail.Picks) > 0
	if hasDraft {
		draftFile := filepath.Join(outputDir, fmt.Sprintf("draft-%s.html", year))
		if err := generator.GenerateDraftPage(draftFile); err != nil {
			return SeasonInfo{}, fmt.Errorf("failed to generate draft page: %w", err)
		}
	}

	// Get season information for the index page
	teams := reader.GetTeams()
	
	return SeasonInfo{
		Year:        year,
		LeagueName:  generator.getLeagueName(),
		TeamCount:   len(teams),
		LastUpdated: generator.getLastUpdated(),
		FileName:    fmt.Sprintf("season-%s.html", year),
		HasDraft:    hasDraft,
	}, nil
}

// generateIndexPage generates the main index page with links to all seasons
func generateIndexPage(seasons []SeasonInfo, outputDir string) error {
	// Create index generator
	generator := NewIndexGenerator(seasons)

	// Generate the index page
	outputFile := filepath.Join(outputDir, "index.html")
	return generator.GenerateIndexPage(outputFile)
}

// generatePodcastsPage generates the podcasts page
func generatePodcastsPage(outputDir string) error {
	// Create a dummy reader for the website generator (we don't need league data for podcasts)
	// We'll use the first available season file or create a minimal reader
	files, err := filepath.Glob(filepath.Join("data", "espn_league_*.json"))
	if err != nil || len(files) == 0 {
		// If no league files, create a minimal generator
		generator := &WebsiteGenerator{}
		outputFile := filepath.Join(outputDir, "podcasts.html")
		return generator.GeneratePodcastsPage(outputFile)
	}

	// Use the first available season file
	reader, err := NewLeagueReader(files[0])
	if err != nil {
		return fmt.Errorf("failed to create league reader for podcasts: %w", err)
	}

	// Create website generator
	generator := NewWebsiteGenerator(reader)

	// Generate the podcasts page
	outputFile := filepath.Join(outputDir, "podcasts.html")
	return generator.GeneratePodcastsPage(outputFile)
}

// generateAIData generates AI data files for all seasons
func generateAIData(files []string, dataDir string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to process")
	}

	// Process each season file
	for _, file := range files {
		// Extract season from filename
		season := extractSeasonFromFilename(file)
		if season == "" {
			continue
		}

		// Create league reader for this season
		reader, err := NewLeagueReader(file)
		if err != nil {
			fmt.Printf("Warning: failed to read file %s: %v\n", file, err)
			continue
		}

		// Create AI data generator for this season
		seasonDir := fmt.Sprintf("ai/%s", season)
		generator := NewAIDataGenerator(reader, seasonDir)
		
		// Generate all AI data files for this season
		if err := generator.GenerateAllData(); err != nil {
			fmt.Printf("Warning: failed to generate AI data for season %s: %v\n", season, err)
			continue
		}
		
		fmt.Printf("Generated AI data for season %s\n", season)
	}

	return nil
}

// extractSeasonFromFilename extracts the season year from a filename
func extractSeasonFromFilename(filename string) string {
	// Look for patterns like "espn_league_2024.json"
	if strings.Contains(filename, "espn_league_") {
		parts := strings.Split(filename, "_")
		for i, part := range parts {
			if part == "league" && i+1 < len(parts) {
				season := strings.TrimSuffix(parts[i+1], ".json")
				// Validate it's a 4-digit year
				if len(season) == 4 {
					return season
				}
			}
		}
	}
	return ""
}
