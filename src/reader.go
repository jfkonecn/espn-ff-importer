package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// LeagueReader provides functionality to read and access ESPN league data
type LeagueReader struct {
	league    *ESPNLeague
	players   map[int]*Player
	proTeams  map[int]*ProTeam
}

// NewLeagueReader creates a new LeagueReader from a JSON file path
func NewLeagueReader(filePath string) (*LeagueReader, error) {
	// Read the JSON file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Parse the JSON data
	var league ESPNLeague
	if err := json.Unmarshal(data, &league); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create reader and load player and pro team data
	reader := &LeagueReader{
		league:   &league,
		players:  make(map[int]*Player),
		proTeams: make(map[int]*ProTeam),
	}
	
	// Load player data for the same season
	if err := reader.loadPlayerData(); err != nil {
		// Log the error but don't fail - player data is optional
		fmt.Printf("Warning: Could not load player data: %v\n", err)
	}

	// Load pro team data for the same season
	if err := reader.loadProTeamData(); err != nil {
		// Log the error but don't fail - pro team data is optional
		fmt.Printf("Warning: Could not load pro team data: %v\n", err)
	}

	return reader, nil
}

// loadPlayerData loads player data from the corresponding season file
func (lr *LeagueReader) loadPlayerData() error {
	// Extract season from league data
	season := lr.league.SeasonID
	
	// Try to find the player file
	playerFilePath := fmt.Sprintf("data/espn_players_%d.json", season)
	
	// Read the player JSON file
	data, err := ioutil.ReadFile(playerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read player file %s: %w", playerFilePath, err)
	}

	// Parse the player data
	var players []Player
	if err := json.Unmarshal(data, &players); err != nil {
		return fmt.Errorf("failed to parse player JSON: %w", err)
	}

	// Build player lookup map
	for i := range players {
		lr.players[players[i].ID] = &players[i]
	}

	return nil
}

// loadProTeamData loads pro team data from the corresponding season file
func (lr *LeagueReader) loadProTeamData() error {
	// Extract season from league data
	season := lr.league.SeasonID
	
	// Try to find the pro team file
	proTeamFilePath := fmt.Sprintf("data/espn_pro_teams_%d.json", season)
	
	// Read the pro team JSON file
	data, err := ioutil.ReadFile(proTeamFilePath)
	if err != nil {
		return fmt.Errorf("failed to read pro team file %s: %w", proTeamFilePath, err)
	}

	// Parse the pro team data
	var proTeamsData ProTeamsData
	if err := json.Unmarshal(data, &proTeamsData); err != nil {
		return fmt.Errorf("failed to parse pro team JSON: %w", err)
	}

	// Build pro team lookup map
	for i := range proTeamsData.Settings.ProTeams {
		lr.proTeams[proTeamsData.Settings.ProTeams[i].ID] = &proTeamsData.Settings.ProTeams[i]
	}

	return nil
}

// GetProTeamByID returns a pro team by their ID
func (lr *LeagueReader) GetProTeamByID(proTeamID int) *ProTeam {
	return lr.proTeams[proTeamID]
}

// GetPlayerByID returns a player by their ID
func (lr *LeagueReader) GetPlayerByID(playerID int) *Player {
	return lr.players[playerID]
}

// GetLeague returns the full league data
func (lr *LeagueReader) GetLeague() *ESPNLeague {
	return lr.league
}

// GetTeams returns all teams in the league
func (lr *LeagueReader) GetTeams() []Team {
	return lr.league.Teams
}

// GetTeamByID returns a team by its ID
func (lr *LeagueReader) GetTeamByID(teamID int) *Team {
	for _, team := range lr.league.Teams {
		if team.ID == teamID {
			return &team
		}
	}
	return nil
}

// GetTeamByName returns a team by its name
func (lr *LeagueReader) GetTeamByName(name string) *Team {
	for _, team := range lr.league.Teams {
		if team.Name == name {
			return &team
		}
	}
	return nil
}

// GetSchedule returns all matchups in the schedule
func (lr *LeagueReader) GetSchedule() []Matchup {
	// remove games that haven't ended
	var schedule []Matchup
	for _, item := range lr.league.Schedule {
		if item.Winner != "UNDECIDED" {
			schedule = append(schedule, item)
		}
	}
	return schedule

}

// GetMatchupsByPeriod returns all matchups for a specific period
func (lr *LeagueReader) GetMatchupsByPeriod(periodID int) []Matchup {
	var matchups []Matchup
	for _, matchup := range lr.league.Schedule {
		if matchup.MatchupPeriodID == periodID {
			matchups = append(matchups, matchup)
		}
	}
	return matchups
}

// GetMembers returns all league members
func (lr *LeagueReader) GetMembers() []Member {
	return lr.league.Members
}

// GetMemberByID returns a member by their ID
func (lr *LeagueReader) GetMemberByID(memberID string) *Member {
	for _, member := range lr.league.Members {
		if member.ID == memberID {
			return &member
		}
	}
	return nil
}

// GetLeagueID returns the league ID
func (lr *LeagueReader) GetLeagueID() int {
	return lr.league.ID
}

// GetSeasonID returns the season ID
func (lr *LeagueReader) GetSeasonID() int {
	return lr.league.SeasonID
}

// GetScoringPeriodID returns the current scoring period ID
func (lr *LeagueReader) GetScoringPeriodID() int {
	return lr.league.ScoringPeriodID
}

// GetTeamStandings returns teams sorted by points (descending)
func (lr *LeagueReader) GetTeamStandings() []Team {
	teams := make([]Team, len(lr.league.Teams))
	copy(teams, lr.league.Teams)

	// Sort by points (descending)
	for i := 0; i < len(teams)-1; i++ {
		for j := i + 1; j < len(teams); j++ {
			if teams[i].Points < teams[j].Points {
				teams[i], teams[j] = teams[j], teams[i]
			}
		}
	}

	return teams
}

// GetTeamRecord returns the record for a specific team
func (lr *LeagueReader) GetTeamRecord(teamID int) *Record {
	team := lr.GetTeamByID(teamID)
	if team == nil {
		return nil
	}
	return &team.Record
}

// PrintLeagueSummary prints a summary of the league
func (lr *LeagueReader) PrintLeagueSummary() {
	fmt.Printf("League ID: %d\n", lr.league.ID)
	fmt.Printf("Season ID: %d\n", lr.league.SeasonID)
	fmt.Printf("Current Scoring Period: %d\n", lr.league.ScoringPeriodID)
	fmt.Printf("Number of Teams: %d\n", len(lr.league.Teams))
	fmt.Printf("Number of Members: %d\n", len(lr.league.Members))
	fmt.Printf("Number of Matchups: %d\n", len(lr.league.Schedule))
	fmt.Printf("Draft Status: Drafted=%v, InProgress=%v\n",
		lr.league.DraftDetail.Drafted, lr.league.DraftDetail.InProgress)
}

// PrintTeamStandings prints the current team standings
func (lr *LeagueReader) PrintTeamStandings() {
	standings := lr.GetTeamStandings()
	fmt.Println("\nTeam Standings:")
	fmt.Println("Rank | Team Name | Points | Record (W-L-T)")
	fmt.Println("-----|-----------|--------|---------------")

	for i, team := range standings {
		record := team.Record.Overall
		fmt.Printf("%4d | %-10s | %6.2f | %d-%d-%d\n",
			i+1, team.Name, team.Points, record.Wins, record.Losses, record.Ties)
	}
}

// SaveToFile saves the league data to a new JSON file
func (lr *LeagueReader) SaveToFile(filePath string) error {
	data, err := json.MarshalIndent(lr.league, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
