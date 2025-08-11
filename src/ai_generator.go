package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// AIDataGenerator generates markdown files for AI consumption
type AIDataGenerator struct {
	reader *LeagueReader
	outputDir string
}

// NewAIDataGenerator creates a new AI data generator
func NewAIDataGenerator(reader *LeagueReader, outputDir string) *AIDataGenerator {
	return &AIDataGenerator{
		reader:    reader,
		outputDir: outputDir,
	}
}

// GenerateAllData generates all AI data files
func (g *AIDataGenerator) GenerateAllData() error {
	// Create AI directory if it doesn't exist
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create AI directory: %w", err)
	}

	// Generate each data file
	if err := g.generateStandings(); err != nil {
		return fmt.Errorf("failed to generate standings: %w", err)
	}

	if err := g.generateDraftOrKeepers(); err != nil {
		return fmt.Errorf("failed to generate draft/keepers: %w", err)
	}

	if err := g.generateTopMoves(); err != nil {
		return fmt.Errorf("failed to generate top moves: %w", err)
	}

	if err := g.generateLatestWeekResults(); err != nil {
		return fmt.Errorf("failed to generate latest week results: %w", err)
	}

	if err := g.generateSeasonResults(); err != nil {
		return fmt.Errorf("failed to generate season results: %w", err)
	}

	// Generate current matchups if there are pending games
	if err := g.generateCurrentMatchups(); err != nil {
		return fmt.Errorf("failed to generate current matchups: %w", err)
	}

	// Generate final standings if season is complete
	if err := g.generateFinalStandings(); err != nil {
		return fmt.Errorf("failed to generate final standings: %w", err)
	}

	return nil
}

// generateStandings generates current standings
func (g *AIDataGenerator) generateStandings() error {
	teams := g.reader.GetTeamStandings()
	
	content := "# Current League Standings\n\n"
	content += fmt.Sprintf("**League:** %s\n", g.reader.GetLeague().Settings.Name)
	content += fmt.Sprintf("**Season:** %d\n", g.reader.GetSeasonID())
	content += fmt.Sprintf("**Current Week:** %d\n\n", g.reader.GetScoringPeriodID())
	
	content += "| Rank | Team Name | Owner | Points | Record (W-L-T) | Points For | Points Against |\n"
	content += "|------|-----------|-------|--------|----------------|------------|----------------|\n"
	
	for i, team := range teams {
		record := team.Record.Overall
		// Get owner name using the same logic as the website generator
		owner := g.reader.GetMemberByID(team.PrimaryOwner)
		ownerName := "Unknown"
		if owner != nil {
			ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
		}
		
		content += fmt.Sprintf("| %d | %s | %s | %.2f | %d-%d-%d | %.2f | %.2f |\n",
			i+1, team.Name, ownerName, team.Points, record.Wins, record.Losses, record.Ties,
			record.PointsFor, record.PointsAgainst)
	}
	
	return g.writeFile("standings.md", content)
}

// generateDraftOrKeepers generates draft results or keeper information
func (g *AIDataGenerator) generateDraftOrKeepers() error {
	league := g.reader.GetLeague()
	
	if league.DraftDetail.Drafted {
		// Generate both draft results and keeper info
		if err := g.generateDraftResults(); err != nil {
			return err
		}
		return g.generateKeeperInfoFromDraft()
	} else {
		return g.generateKeeperInfo()
	}
}

// generateDraftResults generates draft results
func (g *AIDataGenerator) generateDraftResults() error {
	league := g.reader.GetLeague()
	picks := league.DraftDetail.Picks
	
	content := "# Auction Draft Results\n\n"
	content += fmt.Sprintf("**Draft Date:** %s\n", time.Unix(league.DraftDetail.CompleteDate/1000, 0).Format("January 2, 2006"))
	content += fmt.Sprintf("**Total Picks:** %d\n\n", len(picks))
	
	// Sort picks by overall pick number (chronological order)
	sort.Slice(picks, func(i, j int) bool {
		return picks[i].OverallPickNumber < picks[j].OverallPickNumber
	})
	
	content += "| Pick | Team | Owner | Player | Position | Pro Team | Price | Keeper |\n"
	content += "|------|------|-------|--------|----------|----------|-------|--------|\n"
	
	for _, pick := range picks {
		team := g.reader.GetTeamByID(pick.TeamID)
		if team == nil {
			continue
		}
		
		// Get owner name using the same logic as the website generator
		owner := g.reader.GetMemberByID(team.PrimaryOwner)
		ownerName := "Unknown"
		if owner != nil {
			ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
		}
		
		// Get position name from lineup slot ID (same as website)
		position := g.getPositionFromSlotID(pick.LineupSlotId)
		
		// Look up player information
		player := g.reader.GetPlayerByID(pick.PlayerID)
		playerName := fmt.Sprintf("Player %d", pick.PlayerID)
		proTeamName := "Unknown Team"
		if player != nil {
			playerName = player.FullName
			proTeam := g.reader.GetProTeamByID(player.ProTeamID)
			if proTeam != nil {
				proTeamName = proTeam.Abbrev
			}
		}
		
		keeperStatus := "No"
		if pick.Keeper {
			keeperStatus = "Yes"
		}
		
		content += fmt.Sprintf("| %d | %s | %s | %s | %s | %s | $%d | %s |\n",
			pick.OverallPickNumber, team.Name, ownerName, playerName, position, proTeamName, pick.BidAmount, keeperStatus)
	}
	
	return g.writeFile("draft-results.md", content)
}

// generateKeeperInfo generates keeper eligibility information
func (g *AIDataGenerator) generateKeeperInfo() error {
	league := g.reader.GetLeague()
	
	content := "# Keeper Eligibility Information\n\n"
	
	if league.DraftDetail.Drafted {
		content += "**Note:** Draft has already occurred. Here is keeper eligibility information for next season.\n\n"
	} else {
		content += "**Note:** Draft has not occurred yet. Here is keeper eligibility information.\n\n"
	}
	
	content += "## Keeper Rules\n\n"
	content += "- Keepers must be declared before the draft\n"
	content += "- Each team can keep up to 2 players\n"
	content += "- Kept players cost 2 rounds higher than their original draft position\n"
	content += "- Players can only be kept for 2 consecutive seasons\n\n"
	
	// Get current season
	currentSeason := league.SeasonID
	
	// Try to load previous year's data for keeper calculations
	previousSeason := currentSeason - 1
	previousSeasonFile := fmt.Sprintf("data/espn_league_%d.json", previousSeason)
	
	var eligibilities []KeeperEligibility
	
	// Try to use previous year's data if available
	if previousReader, err := NewLeagueReader(previousSeasonFile); err == nil {
		websiteGen := NewWebsiteGenerator(previousReader)
		eligibilities = websiteGen.calculateKeeperEligibility()
	} else {
		// Fall back to current season data
		websiteGen := NewWebsiteGenerator(g.reader)
		eligibilities = websiteGen.calculateKeeperEligibility()
	}
	
	// Group eligibilities by team
	teamEligibilities := make(map[string][]KeeperEligibility)
	for _, eligibility := range eligibilities {
		teamEligibilities[eligibility.TeamName] = append(teamEligibilities[eligibility.TeamName], eligibility)
	}
	
	content += "## Keeper-Eligible Players by Team\n\n"
	
	// Sort teams for consistent output
	var teamNames []string
	for teamName := range teamEligibilities {
		teamNames = append(teamNames, teamName)
	}
	sort.Strings(teamNames)
	
	for _, teamName := range teamNames {
		content += fmt.Sprintf("### %s\n\n", teamName)
		
		eligibilities := teamEligibilities[teamName]
		if len(eligibilities) > 0 {
			content += "| Player | Position | Pro Team | Current Price | Keeper Cost | Years Kept | Eligible |\n"
			content += "|--------|----------|----------|---------------|-------------|------------|----------|\n"
			
			for _, eligibility := range eligibilities {
				eligible := "Yes"
				if !eligibility.IsEligible {
					eligible = "No"
				}
				
				content += fmt.Sprintf("| %s | %s | %s | %d | %d | %d | %s |\n",
					eligibility.PlayerName, eligibility.Position, eligibility.ProTeamAbbrev,
					eligibility.CurrentPrice, eligibility.NextYearPrice, eligibility.KeeperYears, eligible)
			}
		} else {
			content += "*No keeper-eligible players found for this team.*\n"
		}
		
		content += "\n"
	}
	
	return g.writeFile("keeper-info.md", content)
}

// generateKeeperInfoFromDraft generates keeper eligibility information from draft data
func (g *AIDataGenerator) generateKeeperInfoFromDraft() error {
	content := "# Keeper Eligibility Information\n\n"
	content += "**Note:** Draft has already occurred. Here is keeper eligibility information for next season.\n\n"
	
	content += "## Keeper Rules\n\n"
	content += "- Keepers must be declared before the draft\n"
	content += "- Each team can keep up to 2 players\n"
	content += "- Kept players cost 2 rounds higher than their original draft position\n"
	content += "- Players can only be kept for 2 consecutive seasons\n\n"
	
	// Get current season
	league := g.reader.GetLeague()
	currentSeason := league.SeasonID
	
	// Try to load previous year's data for keeper calculations
	previousSeason := currentSeason - 1
	previousSeasonFile := fmt.Sprintf("data/espn_league_%d.json", previousSeason)
	
	var eligibilities []KeeperEligibility
	
	// Try to use previous year's data if available
	if previousReader, err := NewLeagueReader(previousSeasonFile); err == nil {
		websiteGen := NewWebsiteGenerator(previousReader)
		eligibilities = websiteGen.calculateKeeperEligibility()
	} else {
		// Fall back to current season data
		websiteGen := NewWebsiteGenerator(g.reader)
		eligibilities = websiteGen.calculateKeeperEligibility()
	}
	
	// Group eligibilities by team
	teamEligibilities := make(map[string][]KeeperEligibility)
	for _, eligibility := range eligibilities {
		teamEligibilities[eligibility.TeamName] = append(teamEligibilities[eligibility.TeamName], eligibility)
	}
	
	content += "## Keeper-Eligible Players by Team\n\n"
	
	// Sort teams for consistent output
	var teamNames []string
	for teamName := range teamEligibilities {
		teamNames = append(teamNames, teamName)
	}
	sort.Strings(teamNames)
	
	for _, teamName := range teamNames {
		content += fmt.Sprintf("### %s\n\n", teamName)
		
		eligibilities := teamEligibilities[teamName]
		if len(eligibilities) > 0 {
			content += "| Player | Position | Pro Team | Current Price | Keeper Cost | Years Kept | Eligible |\n"
			content += "|--------|----------|----------|---------------|-------------|------------|----------|\n"
			
			for _, eligibility := range eligibilities {
				eligible := "Yes"
				if !eligibility.IsEligible {
					eligible = "No"
				}
				
				content += fmt.Sprintf("| %s | %s | %s | %d | %d | %d | %s |\n",
					eligibility.PlayerName, eligibility.Position, eligibility.ProTeamAbbrev,
					eligibility.CurrentPrice, eligibility.NextYearPrice, eligibility.KeeperYears, eligible)
			}
		} else {
			content += "*No keeper-eligible players found for this team.*\n"
		}
		
		content += "\n"
	}
	
	return g.writeFile("keeper-info.md", content)
}

// generateTopMoves generates top added and dropped players from ESPN data
func (g *AIDataGenerator) generateTopMoves() error {
	content := "# Top Added and Dropped Players\n\n"
	
	// Get the season from the league data
	league := g.reader.GetLeague()
	season := league.SeasonID
	
	// Try to read the most added and dropped files
	addedFile := fmt.Sprintf("data/espn_most_added_%d.json", season)
	droppedFile := fmt.Sprintf("data/espn_most_dropped_%d.json", season)
	
	content += "## Most Added Players\n\n"
	
	// Read most added players
	if addedData, err := os.ReadFile(addedFile); err == nil {
		var addedResponse struct {
			Players []struct {
				Player struct {
					FullName string `json:"fullName"`
					FirstName string `json:"firstName"`
					LastName string `json:"lastName"`
					ProTeamId int `json:"proTeamId"`
					DefaultPositionId int `json:"defaultPositionId"`
					Ownership struct {
						PercentChange float64 `json:"percentChange"`
						PercentOwned float64 `json:"percentOwned"`
					} `json:"ownership"`
				} `json:"player"`
			} `json:"players"`
		}
		
		if err := json.Unmarshal(addedData, &addedResponse); err == nil {
			content += "| Rank | Player | Position | Team | % Change | % Owned |\n"
			content += "|------|--------|----------|------|----------|----------|\n"
			
			// Get position names
			positionNames := map[int]string{
				0: "QB", 1: "RB", 2: "RB", 3: "WR", 4: "WR", 5: "TE", 6: "TE", 16: "D/ST", 17: "K",
			}
			
			// Get team names
			teamNames := map[int]string{
				0: "FA", 1: "ATL", 2: "BUF", 3: "CAR", 4: "CHI", 5: "CIN", 6: "DAL", 7: "DEN", 8: "DET", 9: "GB",
				10: "TEN", 11: "IND", 12: "KC", 13: "LV", 14: "LAR", 15: "MIA", 16: "MIN", 17: "NE", 18: "NO", 19: "NYG",
				20: "NYJ", 21: "PHI", 22: "PIT", 23: "SEA", 24: "BAL", 25: "TB", 26: "LAC", 27: "SF", 28: "ARI", 29: "JAX",
				30: "HOU", 31: "CLE", 32: "WAS", 33: "BAL", 34: "HOU",
			}
			
			for i, player := range addedResponse.Players {
				if i >= 20 { // Limit to top 20
					break
				}
				
				position := positionNames[player.Player.DefaultPositionId]
				team := teamNames[player.Player.ProTeamId]
				percentChange := player.Player.Ownership.PercentChange
				percentOwned := player.Player.Ownership.PercentOwned
				
				content += fmt.Sprintf("| %d | %s | %s | %s | %.1f%% | %.1f%% |\n",
					i+1,
					player.Player.FullName,
					position,
					team,
					percentChange,
					percentOwned)
			}
		}
	} else {
		content += "Most added data not available.\n"
	}
	
	content += "\n## Most Dropped Players\n\n"
	
	// Read most dropped players
	if droppedData, err := os.ReadFile(droppedFile); err == nil {
		var droppedResponse struct {
			Players []struct {
				Player struct {
					FullName string `json:"fullName"`
					FirstName string `json:"firstName"`
					LastName string `json:"lastName"`
					ProTeamId int `json:"proTeamId"`
					DefaultPositionId int `json:"defaultPositionId"`
					Ownership struct {
						PercentChange float64 `json:"percentChange"`
						PercentOwned float64 `json:"percentOwned"`
					} `json:"ownership"`
				} `json:"player"`
			} `json:"players"`
		}
		
		if err := json.Unmarshal(droppedData, &droppedResponse); err == nil {
			content += "| Rank | Player | Position | Team | % Change | % Owned |\n"
			content += "|------|--------|----------|------|----------|----------|\n"
			
			// Get position names
			positionNames := map[int]string{
				0: "QB", 1: "RB", 2: "RB", 3: "WR", 4: "WR", 5: "TE", 6: "TE", 16: "D/ST", 17: "K",
			}
			
			// Get team names
			teamNames := map[int]string{
				0: "FA", 1: "ATL", 2: "BUF", 3: "CAR", 4: "CHI", 5: "CIN", 6: "DAL", 7: "DEN", 8: "DET", 9: "GB",
				10: "TEN", 11: "IND", 12: "KC", 13: "LV", 14: "LAR", 15: "MIA", 16: "MIN", 17: "NE", 18: "NO", 19: "NYG",
				20: "NYJ", 21: "PHI", 22: "PIT", 23: "SEA", 24: "BAL", 25: "TB", 26: "LAC", 27: "SF", 28: "ARI", 29: "JAX",
				30: "HOU", 31: "CLE", 32: "WAS", 33: "BAL", 34: "HOU",
			}
			
			for i, player := range droppedResponse.Players {
				if i >= 20 { // Limit to top 20
					break
				}
				
				position := positionNames[player.Player.DefaultPositionId]
				team := teamNames[player.Player.ProTeamId]
				percentChange := player.Player.Ownership.PercentChange
				percentOwned := player.Player.Ownership.PercentOwned
				
				content += fmt.Sprintf("| %d | %s | %s | %s | %.1f%% | %.1f%% |\n",
					i+1,
					player.Player.FullName,
					position,
					team,
					percentChange,
					percentOwned)
			}
		}
	} else {
		content += "Most dropped data not available.\n"
	}
	
	return g.writeFile("top-moves.md", content)
}

// generateLatestWeekResults generates the latest week's results
func (g *AIDataGenerator) generateLatestWeekResults() error {
	currentPeriod := g.reader.GetScoringPeriodID()
	matchups := g.reader.GetMatchupsByPeriod(currentPeriod)
	
	content := fmt.Sprintf("# Week %d Results\n\n", currentPeriod)
	
	if len(matchups) == 0 {
		content += "No completed games for this week yet.\n"
		return g.writeFile("latest-week-results.md", content)
	}
	
	for _, matchup := range matchups {
		homeTeam := g.reader.GetTeamByID(matchup.Home.TeamID)
		awayTeam := g.reader.GetTeamByID(matchup.Away.TeamID)
		
		if homeTeam == nil || awayTeam == nil {
			continue
		}
		
		content += fmt.Sprintf("## %s vs %s\n\n", awayTeam.Name, homeTeam.Name)
		content += fmt.Sprintf("**Final Score:** %s %.2f - %s %.2f\n\n", 
			awayTeam.Name, matchup.Away.TotalPoints, homeTeam.Name, matchup.Home.TotalPoints)
		
		// Add player performance details if roster data is available
		if matchup.Home.TotalPoints > 0 || matchup.Away.TotalPoints > 0 {
			content += "### Key Performances\n\n"
			content += "*This section would show individual player performances*\n\n"
		}
		
		content += "---\n\n"
	}
	
	return g.writeFile("latest-week-results.md", content)
}

// generateSeasonResults generates all season results
func (g *AIDataGenerator) generateSeasonResults() error {
	schedule := g.reader.GetSchedule()
	
	content := "# Complete Season Results\n\n"
	content += fmt.Sprintf("**Season:** %d\n\n", g.reader.GetSeasonID())
	
	// Group by week
	weeks := make(map[int][]Matchup)
	for _, matchup := range schedule {
		weeks[matchup.MatchupPeriodID] = append(weeks[matchup.MatchupPeriodID], matchup)
	}
	
	// Sort weeks
	var weekNumbers []int
	for week := range weeks {
		weekNumbers = append(weekNumbers, week)
	}
	sort.Ints(weekNumbers)
	
	for _, week := range weekNumbers {
		matchups := weeks[week]
		content += fmt.Sprintf("## Week %d\n\n", week)
		
		for _, matchup := range matchups {
			homeTeam := g.reader.GetTeamByID(matchup.Home.TeamID)
			awayTeam := g.reader.GetTeamByID(matchup.Away.TeamID)
			
			if homeTeam == nil || awayTeam == nil {
				continue
			}
			
			winner := "Tie"
			if matchup.Winner == "HOME" {
				winner = homeTeam.Name
			} else if matchup.Winner == "AWAY" {
				winner = awayTeam.Name
			}
			
			content += fmt.Sprintf("**%s** %.2f - **%s** %.2f *(Winner: %s)*\n\n",
				awayTeam.Name, matchup.Away.TotalPoints, homeTeam.Name, matchup.Home.TotalPoints, winner)
		}
		content += "---\n\n"
	}
	
	return g.writeFile("season-results.md", content)
}

// generateFinalStandings generates final standings for completed seasons
func (g *AIDataGenerator) generateFinalStandings() error {
	league := g.reader.GetLeague()
	teams := g.reader.GetTeams()

	content := "# Final Season Standings\n\n"
	content += fmt.Sprintf("**League:** %s\n", league.Settings.Name)
	content += fmt.Sprintf("**Season:** %d\n\n", league.SeasonID)

	// Check if season is complete by looking for teams with final rankings
	var finalStandings []struct {
		TeamName  string
		OwnerName string
		Rank      int
		Points    float64
		Record    GameRecord
	}

	for _, team := range teams {
		// Only include teams that have final rankings (season is complete)
		if team.RankCalculatedFinal > 0 {
			// Get owner name using the same logic as the website generator
			owner := g.reader.GetMemberByID(team.PrimaryOwner)
			ownerName := "Unknown"
			if owner != nil {
				ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
			}

			finalStandings = append(finalStandings, struct {
				TeamName  string
				OwnerName string
				Rank      int
				Points    float64
				Record    GameRecord
			}{
				TeamName:  team.Name,
				OwnerName: ownerName,
				Rank:      team.RankCalculatedFinal,
				Points:    team.Points,
				Record:    team.Record.Overall,
			})
		}
	}

	// If no final standings, season is not complete
	if len(finalStandings) == 0 {
		content += "**Season is not yet complete.**\n\n"
		content += "Final standings will be available once the season ends and playoff results are finalized.\n"
		return g.writeFile("final-standings.md", content)
	}

	// Sort by final rank
	sort.Slice(finalStandings, func(i, j int) bool {
		return finalStandings[i].Rank < finalStandings[j].Rank
	})

	content += "## Final Standings\n\n"
	content += "| Rank | Team Name | Owner | Points | Record (W-L-T) | Points For | Points Against |\n"
	content += "|------|-----------|-------|--------|----------------|------------|----------------|\n"

	for _, standing := range finalStandings {
		record := standing.Record
		content += fmt.Sprintf("| %d | %s | %s | %.2f | %d-%d-%d | %.2f | %.2f |\n",
			standing.Rank, standing.TeamName, standing.OwnerName, standing.Points,
			record.Wins, record.Losses, record.Ties, record.PointsFor, record.PointsAgainst)
	}

	return g.writeFile("final-standings.md", content)
}

// generateCurrentMatchups generates a markdown file showing current week matchups with starting lineups and projected totals
func (g *AIDataGenerator) generateCurrentMatchups() error {
	currentMatchups := g.reader.GetCurrentMatchups()
	
	// If no current matchups, don't generate the file
	if len(currentMatchups) == 0 {
		return nil
	}
	
	content := "# Current Week Matchups\n\n"
	content += fmt.Sprintf("**League:** %s\n", g.reader.GetLeague().Settings.Name)
	content += fmt.Sprintf("**Season:** %d\n", g.reader.GetSeasonID())
	content += fmt.Sprintf("**Current Week:** %d\n\n", g.reader.GetScoringPeriodID())
	content += "**Status:** Games are pending - showing projected lineups and totals\n\n"
	
	for i, matchup := range currentMatchups {
		content += fmt.Sprintf("## Matchup %d\n\n", i+1)
		
		// Away team
		awayTeam := g.reader.GetTeamByID(matchup.Away.TeamID)
		awayOwner := g.reader.GetMemberByID(awayTeam.PrimaryOwner)
		awayOwnerName := "Unknown"
		if awayOwner != nil {
			awayOwnerName = fmt.Sprintf("%s %s", awayOwner.FirstName, awayOwner.LastName)
		}
		
		content += fmt.Sprintf("### %s (%s)\n\n", awayTeam.Name, awayOwnerName)
		if matchup.Away.RosterForCurrentScoringPeriod != nil {
			content += g.generateTeamRosterTable(matchup.Away.RosterForCurrentScoringPeriod, "Away")
		} else {
			content += "*Roster not available*\n\n"
		}
		
		// Home team
		homeTeam := g.reader.GetTeamByID(matchup.Home.TeamID)
		homeOwner := g.reader.GetMemberByID(homeTeam.PrimaryOwner)
		homeOwnerName := "Unknown"
		if homeOwner != nil {
			homeOwnerName = fmt.Sprintf("%s %s", homeOwner.FirstName, homeOwner.LastName)
		}
		
		content += fmt.Sprintf("### %s (%s)\n\n", homeTeam.Name, homeOwnerName)
		if matchup.Home.RosterForCurrentScoringPeriod != nil {
			content += g.generateTeamRosterTable(matchup.Home.RosterForCurrentScoringPeriod, "Home")
		} else {
			content += "*Roster not available*\n\n"
		}
		
		content += "---\n\n"
	}
	
	return g.writeFile("current-matchups.md", content)
}

// generateTeamRosterTable generates a markdown table for a team's roster
func (g *AIDataGenerator) generateTeamRosterTable(roster *TeamRoster, teamType string) string {
	content := fmt.Sprintf("**%s Team Starting Lineup:**\n\n", teamType)
	content += "| Position | Player | Pro Team | Status | Projected Points |\n"
	content += "|----------|--------|----------|--------|------------------|\n"
	
	// Sort entries by custom position order: QB, RB, WR, TE, FLEX, D/ST, K, Bench, IR
	sort.Slice(roster.Entries, func(i, j int) bool {
		orderI := g.getPositionOrder(roster.Entries[i].LineupSlotID)
		orderJ := g.getPositionOrder(roster.Entries[j].LineupSlotID)
		return orderI < orderJ
	})
	
	for _, entry := range roster.Entries {
		position := g.getPositionFromSlotID(entry.LineupSlotID)
		player := entry.PlayerPoolEntry.Player
		
		// Get pro team name
		proTeamName := "Unknown"
		if player.ProTeamID > 0 {
			proTeam := g.reader.GetProTeamByID(player.ProTeamID)
			if proTeam != nil {
				proTeamName = proTeam.Abbrev
			}
		}
		
		// Get projected points from stats
		projectedPoints := "N/A"
		if len(player.Stats) > 0 && player.Stats[0].AppliedTotal > 0 {
			projectedPoints = fmt.Sprintf("%.1f", player.Stats[0].AppliedTotal)
		}
		
		// Status indicator
		status := "Active"
		if player.Injured {
			status = "Injured"
		} else if !player.Active {
			status = "Inactive"
		}
		
		content += fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			position, player.FullName, proTeamName, status, projectedPoints)
	}
	
	content += "\n"
	return content
}

// getPositionOrder returns a numeric order for sorting positions (lower = earlier in lineup)
func (g *AIDataGenerator) getPositionOrder(slotID int) int {
	order := map[int]int{
		0:  1,  // QB
		2:  2,  // RB
		4:  3,  // WR
		6:  4,  // TE
		23: 5,  // FLEX (between TE and D/ST)
		16: 6,  // D/ST
		17: 7,  // K
		20: 8,  // Bench
		21: 9,  // IR
	}
	
	if pos, exists := order[slotID]; exists {
		return pos
	}
	return 999 // Unknown positions go last
}

// getPositionFromSlotID converts lineup slot ID to position name (same as website)
func (g *AIDataGenerator) getPositionFromSlotID(slotID int) string {
	positions := map[int]string{
		0:  "QB",
		2:  "RB",
		4:  "WR",
		6:  "TE",
		16: "D/ST",
		17: "K",
		20: "Bench",
		21: "IR",
		23: "FLEX",
	}

	if pos, exists := positions[slotID]; exists {
		return pos
	}
	return "Unknown"
}

// writeFile writes content to a file in the AI directory
func (g *AIDataGenerator) writeFile(filename, content string) error {
	filepath := filepath.Join(g.outputDir, filename)
	return os.WriteFile(filepath, []byte(content), 0644)
}

// getPositionName converts position ID to name
func (g *AIDataGenerator) getPositionName(positionID int) string {
	positions := map[int]string{
		1: "QB",
		2: "RB", 
		3: "WR",
		4: "TE",
		5: "K",
		6: "DEF",
		7: "FLEX",
		8: "D/ST",
		9: "IDP",
	}
	
	if name, exists := positions[positionID]; exists {
		return name
	}
	return "Unknown"
} 