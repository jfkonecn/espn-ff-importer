package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"espn-ff-importer/internal/podcast"
)

type leagueFile struct {
	DraftDetail struct {
		Drafted bool `json:"drafted"`
	} `json:"draftDetail"`
	Schedule []struct {
		MatchupPeriodID int    `json:"matchupPeriodId"`
		PlayoffTierType string `json:"playoffTierType"`
		Winner          string `json:"winner"`
	} `json:"schedule"`
	ScoringPeriodID int `json:"scoringPeriodId"`
	SeasonID        int `json:"seasonId"`
	Settings        struct {
		Name string `json:"name"`
	} `json:"settings"`
	Teams []struct {
		RankCalculatedFinal int `json:"rankCalculatedFinal"`
		RankFinal           int `json:"rankFinal"`
	} `json:"teams"`
}

func main() {
	var (
		season  = flag.Int("season", seasonFromEnv(), "Season year")
		dataDir = flag.String("data", "data", "Directory containing ESPN league JSON files")
		output  = flag.String("output", "", "Output season-state JSON path")
	)
	flag.Parse()
	if *season == 0 {
		fatal("determine season", fmt.Errorf("CURRENT_YEAR or -season is required"))
	}

	leaguePath := filepath.Join(*dataDir, fmt.Sprintf("espn_league_%d.json", *season))
	fmt.Printf("Reading league data from %s\n", leaguePath)
	contents, err := os.ReadFile(leaguePath)
	if err != nil {
		fatal("read league data", err)
	}

	fmt.Printf("Determining podcast season state for %d\n", *season)
	var league leagueFile
	if err := json.Unmarshal(contents, &league); err != nil {
		fatal("parse league data", err)
	}

	state := determineState(league)
	if state.Season == 0 {
		state.Season = *season
	}

	outputPath := *output
	if outputPath == "" {
		outputPath = filepath.Join("ai", fmt.Sprintf("%d", state.Season), "season-state.json")
	}

	if err := podcast.WriteJSON(outputPath, state); err != nil {
		fatal("write season state", err)
	}
	fmt.Printf("Season state: phase=%s week=%d draftComplete=%t completedMatchups=%d totalMatchups=%d\n",
		state.Phase, state.Week, state.DraftComplete, state.CompletedMatchups, state.TotalMatchups)
	fmt.Printf("Wrote podcast season state to %s\n", outputPath)
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

func determineState(league leagueFile) podcast.SeasonState {
	completed, total := 0, len(league.Schedule)
	for _, matchup := range league.Schedule {
		if matchup.Winner != "" && matchup.Winner != "UNDECIDED" {
			completed++
		}
	}

	finalRanked := 0
	for _, team := range league.Teams {
		if team.RankCalculatedFinal > 0 || team.RankFinal > 0 {
			finalRanked++
		}
	}

	phase := podcast.PhaseRegularSeason
	if !league.DraftDetail.Drafted {
		phase = podcast.PhaseDraft
	} else if finalRanked == len(league.Teams) && len(league.Teams) > 0 {
		phase = podcast.PhaseSeasonComplete
	} else if total > 0 && completed == total {
		phase = podcast.PhaseSeasonComplete
	} else if isPostSeason(league) {
		phase = podcast.PhasePostSeason
	}

	return podcast.SeasonState{
		Season:            league.SeasonID,
		LeagueName:        league.Settings.Name,
		Phase:             phase,
		Week:              league.ScoringPeriodID,
		DraftComplete:     league.DraftDetail.Drafted,
		CompletedMatchups: completed,
		TotalMatchups:     total,
		GeneratedAt:       time.Now().Format(time.RFC3339),
	}
}

func isPostSeason(league leagueFile) bool {
	postSeasonWeek := 0
	for _, matchup := range league.Schedule {
		if matchup.PlayoffTierType == "" || matchup.PlayoffTierType == "NONE" {
			continue
		}
		if postSeasonWeek == 0 || matchup.MatchupPeriodID < postSeasonWeek {
			postSeasonWeek = matchup.MatchupPeriodID
		}
	}

	return postSeasonWeek > 0 && league.ScoringPeriodID >= postSeasonWeek
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "Failed to %s: %v\n", action, err)
	os.Exit(1)
}
