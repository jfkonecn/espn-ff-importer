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
		seasonInfo, err := processSeasonFile(file, *output)
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

	fmt.Printf("Static website generated successfully in: %s\n", *output)
	fmt.Printf("Processed %d seasons\n", len(seasons))
}

// SeasonInfo represents information about a season for the index page
type SeasonInfo struct {
	Year        string
	LeagueName  string
	TeamCount   int
	LastUpdated string
	FileName    string
}

// processSeasonFile processes a single season file and generates its HTML page
func processSeasonFile(filePath, outputDir string) (SeasonInfo, error) {
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

	// Generate the season page
	outputFile := filepath.Join(outputDir, fmt.Sprintf("season-%s.html", year))
	if err := generator.GenerateSeasonPage(outputFile); err != nil {
		return SeasonInfo{}, fmt.Errorf("failed to generate season page: %w", err)
	}

	// Get season information for the index page
	teams := reader.GetTeams()
	
	return SeasonInfo{
		Year:        year,
		LeagueName:  generator.getLeagueName(),
		TeamCount:   len(teams),
		LastUpdated: generator.getLastUpdated(),
		FileName:    fmt.Sprintf("season-%s.html", year),
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