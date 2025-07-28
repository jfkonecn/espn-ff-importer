package main

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

// WebsiteGenerator handles the generation of the static website
type WebsiteGenerator struct {
	reader *LeagueReader
}

// NewWebsiteGenerator creates a new website generator
func NewWebsiteGenerator(reader *LeagueReader) *WebsiteGenerator {
	return &WebsiteGenerator{reader: reader}
}

// IndexGenerator handles the generation of the main index page
type IndexGenerator struct {
	seasons []SeasonInfo
}

// NewIndexGenerator creates a new index generator
func NewIndexGenerator(seasons []SeasonInfo) *IndexGenerator {
	return &IndexGenerator{seasons: seasons}
}

// IndexData represents the data passed to the index template
type IndexData struct {
	Seasons     []SeasonInfo
	GeneratedAt string
}

// GenerateIndexPage creates the main index HTML page
func (ig *IndexGenerator) GenerateIndexPage(outputPath string) error {
	// Parse the index template
	tmpl, err := template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("failed to parse index template: %w", err)
	}

	// Prepare the template data
	data := IndexData{
		Seasons:     ig.seasons,
		GeneratedAt: time.Now().Format("January 2, 2006 at 3:04 PM"),
	}

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute the template
	err = tmpl.ExecuteTemplate(file, "index.html", data)
	if err != nil {
		return fmt.Errorf("failed to execute index template: %w", err)
	}

	return nil
}

// GenerateWebsite creates the static HTML website
func (wg *WebsiteGenerator) GenerateWebsite(outputPath string) error {
	// Parse all template files
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	// Prepare the template data
	data := wg.prepareTemplateData()

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute the template
	err = tmpl.ExecuteTemplate(file, "website.html", data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateSeasonPage creates a season-specific HTML page
func (wg *WebsiteGenerator) GenerateSeasonPage(outputPath string) error {
	// Parse all template files
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	// Prepare the template data
	data := wg.prepareTemplateData()

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute the template
	err = tmpl.ExecuteTemplate(file, "website.html", data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// getLastUpdated returns the current timestamp as a formatted string
func (wg *WebsiteGenerator) getLastUpdated() string {
	return time.Now().Format("January 2, 2006 at 3:04 PM")
}

// GenerateDraftPage creates a draft-specific HTML page
func (wg *WebsiteGenerator) GenerateDraftPage(outputPath string) error {
	// Parse all template files
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	// Prepare the template data
	data := wg.prepareDraftData()

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute the template
	err = tmpl.ExecuteTemplate(file, "draft.html", data)
	if err != nil {
		return fmt.Errorf("failed to execute draft template: %w", err)
	}

	return nil
}

// prepareDraftData prepares all the data needed for the draft template
func (wg *WebsiteGenerator) prepareDraftData() DraftData {
	league := wg.reader.GetLeague()
	
	// Get draft picks and mark keepers
	draftPicks := wg.prepareDraftPicks()
	keeperPicks := wg.getKeeperPicks(draftPicks)
	
	// Format draft date
	draftDate := "Unknown"
	if league.DraftDetail.CompleteDate > 0 {
		draftTime := time.Unix(league.DraftDetail.CompleteDate/1000, 0)
		draftDate = draftTime.Format("January 2, 2006 at 3:04 PM")
	}
	
	// Determine draft status
	draftStatus := "Not Started"
	if league.DraftDetail.InProgress {
		draftStatus = "In Progress"
	} else if league.DraftDetail.Drafted {
		draftStatus = "Completed"
	}

	return DraftData{
		LeagueName:    wg.getLeagueName(),
		SeasonYear:    fmt.Sprintf("%d", league.SeasonID),
		LastUpdated:   wg.getLastUpdated(),
		GeneratedAt:   time.Now().Format("January 2, 2006 at 3:04 PM"),
		TotalPicks:    len(draftPicks),
		DraftDate:     draftDate,
		DraftStatus:   draftStatus,
		DraftPicks:    draftPicks,
		KeeperPicks:   keeperPicks,
	}
}

// prepareDraftPicks converts draft picks to template rows
func (wg *WebsiteGenerator) prepareDraftPicks() []DraftPickRow {
	league := wg.reader.GetLeague()
	var picks []DraftPickRow

	// Track first picks for each team to mark as keepers
	teamFirstPicks := make(map[int]bool)
	
	// First pass: identify first picks for each team
	for _, pick := range league.DraftDetail.Picks {
		if !teamFirstPicks[pick.TeamID] {
			teamFirstPicks[pick.TeamID] = true
		}
	}

	// Second pass: create pick rows
	for _, pick := range league.DraftDetail.Picks {
		team := wg.reader.GetTeamByID(pick.TeamID)
		if team == nil {
			continue
		}

		owner := wg.reader.GetMemberByID(team.PrimaryOwner)
		ownerName := "Unknown"
		if owner != nil {
			ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
		}

		// Determine if this is the team's first pick (keeper)
		isKeeper := teamFirstPicks[pick.TeamID] && pick.OverallPickNumber == pick.RoundPickNumber

		// Get position name from lineup slot ID
		position := wg.getPositionFromSlotID(pick.LineupSlotId)

		// Look up player information
		player := wg.reader.GetPlayerByID(pick.PlayerID)
		playerName := fmt.Sprintf("Player %d", pick.PlayerID)
		proTeamName := "Unknown Team"
		proTeamAbbrev := "UNK"
		if player != nil {
			playerName = player.FullName
			proTeamName = wg.getProTeamName(player.ProTeamID)
			proTeamAbbrev = wg.getProTeamAbbrev(player.ProTeamID)
		}

		picks = append(picks, DraftPickRow{
			OverallPickNumber: pick.OverallPickNumber,
			TeamName:         team.Name,
			OwnerName:        ownerName,
			PlayerName:       playerName,
			Position:         position,
			ProTeamName:      proTeamName,
			ProTeamAbbrev:    proTeamAbbrev,
			BidAmount:        pick.BidAmount,
			IsKeeper:         isKeeper,
		})
	}

	// Sort by overall pick number
	sort.Slice(picks, func(i, j int) bool {
		return picks[i].OverallPickNumber < picks[j].OverallPickNumber
	})

	return picks
}

// getKeeperPicks filters the draft picks to return only keeper picks
func (wg *WebsiteGenerator) getKeeperPicks(allPicks []DraftPickRow) []DraftPickRow {
	var keeperPicks []DraftPickRow
	for _, pick := range allPicks {
		if pick.IsKeeper {
			keeperPicks = append(keeperPicks, pick)
		}
	}
	return keeperPicks
}

// getPositionFromSlotID converts lineup slot ID to position name
func (wg *WebsiteGenerator) getPositionFromSlotID(slotID int) string {
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

// getProTeamName converts pro team ID to team name using lookup data
func (wg *WebsiteGenerator) getProTeamName(proTeamID int) string {
	proTeam := wg.reader.GetProTeamByID(proTeamID)
	if proTeam != nil {
		return fmt.Sprintf("%s %s", proTeam.Location, proTeam.Name)
	}
	return "Unknown Team"
}

// getProTeamAbbrev converts pro team ID to team abbreviation using lookup data
func (wg *WebsiteGenerator) getProTeamAbbrev(proTeamID int) string {
	proTeam := wg.reader.GetProTeamByID(proTeamID)
	if proTeam != nil {
		return proTeam.Abbrev
	}
	return "UNK"
}

// TemplateData represents the data passed to the HTML template
type TemplateData struct {
	LeagueName          string
	SeasonID            int
	LastUpdated         string
	HasDraft            bool
	Standings           []StandingRow
	WeeklyHighScorers   []WeeklyHighScoreRow
	FinalStandings      []FinalStandingRow
	TeamPayoutTotals    []TeamPayoutTotal
	RecentGamesByWeek   []WeekGames
	WeeklyHighScorerMap map[int]int
	TopHalfMap          map[int]map[int]bool
}

// WeekGames represents a week with its games
type WeekGames struct {
	Week  int
	Games []GameRow
}

// StandingRow represents a row in the standings table
type StandingRow struct {
	Rank        int
	TeamName    string
	OwnerName   string
	Points      string
	Record      string
	H2HWins     int
	TopHalfWins int
	TotalPoints int
	RowClass    string
}

// WeeklyHighScoreRow represents a weekly high score entry
type WeeklyHighScoreRow struct {
	TeamName  string
	OwnerName string
	Score     string
	Week      int
}

// FinalStandingRow represents a final standing payout entry
type FinalStandingRow struct {
	TeamName  string
	OwnerName string
	Rank      int
	Payout    int
}

// TeamPayoutTotal represents total payout for a team
type TeamPayoutTotal struct {
	TeamName            string
	OwnerName           string
	WeeklyHighScores    int
	FinalStandingPayout int
	TotalPayout         int
}

// GameRow represents a game in the recent games section
type GameRow struct {
	Week            int
	MatchupID       int
	AwayTeamName    string
	AwayScore       string
	AwayTeamID      int
	AwayOwnerName   string
	HomeTeamName    string
	HomeScore       string
	HomeTeamID      int
	HomeOwnerName   string
	Winner          string
	WinnerClass     string
	AwayIsTopHalf   bool
	AwayIsHighScore bool
	HomeIsTopHalf   bool
	HomeIsHighScore bool
}

// GetAwayTeamData returns the away team data for the team-score template
func (gr *GameRow) GetAwayTeamData() map[string]interface{} {
	return map[string]interface{}{
		"TeamName":    gr.AwayTeamName,
		"OwnerName":   gr.AwayOwnerName,
		"Score":       gr.AwayScore,
		"IsHighScore": gr.AwayIsHighScore,
		"IsTopHalf":   gr.AwayIsTopHalf,
	}
}

// GetHomeTeamData returns the home team data for the team-score template
func (gr *GameRow) GetHomeTeamData() map[string]interface{} {
	return map[string]interface{}{
		"TeamName":    gr.HomeTeamName,
		"OwnerName":   gr.HomeOwnerName,
		"Score":       gr.HomeScore,
		"IsHighScore": gr.HomeIsHighScore,
		"IsTopHalf":   gr.HomeIsTopHalf,
	}
}

// prepareTemplateData prepares all the data needed for the template
func (wg *WebsiteGenerator) prepareTemplateData() TemplateData {
	league := wg.reader.GetLeague()

	// Calculate standings with custom scoring
	standings := wg.calculateStandings()

	// Calculate payouts
	payouts := wg.calculatePayouts()

	// Get recent games (most recent first)
	recentGames := wg.getRecentGames()

	// Prepare game rows and create lookup maps
	gameRows := wg.prepareGameRows(recentGames)
	recentGamesByWeek := wg.groupGamesByWeek(gameRows)

	// Create lookup maps for template
	weeklyHighScorerMap := make(map[int]int)
	topHalfMap := make(map[int]map[int]bool)

	weeklyHighScorers := wg.getWeeklyHighScorers()
	topHalfScorers := wg.getWeeklyTopHalfScorers()

	for _, hs := range weeklyHighScorers {
		weeklyHighScorerMap[hs.Week] = hs.TeamID
	}

	for _, weekTopHalf := range topHalfScorers {
		if len(weekTopHalf) > 0 {
			week := weekTopHalf[0].Week
			topHalfMap[week] = make(map[int]bool)
			for _, team := range weekTopHalf {
				topHalfMap[week][team.TeamID] = true
			}
		}
	}

	return TemplateData{
		LeagueName:          wg.getLeagueName(),
		SeasonID:            league.SeasonID,
		LastUpdated:         time.Now().Format("January 2, 2006 at 3:04 PM"),
		HasDraft:            len(league.DraftDetail.Picks) > 0,
		Standings:           wg.prepareStandingsRows(standings),
		WeeklyHighScorers:   wg.prepareWeeklyHighScorerRows(payouts.WeeklyHighScorers),
		FinalStandings:      wg.prepareFinalStandingRows(payouts.FinalStandings),
		TeamPayoutTotals:    wg.prepareTeamPayoutTotals(payouts.WeeklyHighScorers, payouts.FinalStandings),
		RecentGamesByWeek:   recentGamesByWeek,
		WeeklyHighScorerMap: weeklyHighScorerMap,
		TopHalfMap:          topHalfMap,
	}
}

// getLeagueName returns the league name from settings
func (wg *WebsiteGenerator) getLeagueName() string {
	league := wg.reader.GetLeague()
	if league.Settings.Name != "" {
		return league.Settings.Name
	}
	// Fallback to first team name if settings name is empty
	teams := wg.reader.GetTeams()
	if len(teams) > 0 {
		return teams[0].Name
	}
	return "Fantasy Football"
}

// Standing represents a team's standing with custom scoring
type Standing struct {
	Team                *Team
	TotalPointsScored   float64
	Record              Record
	H2HWins             int
	TopHalfWins         int
	TotalStandingPoints int
	Rank                int
}

// Payout represents payout information
type Payout struct {
	WeeklyHighScorers []WeeklyHighScore
	FinalStandings    []FinalStanding
}

// WeeklyHighScore represents a weekly high score
type WeeklyHighScore struct {
	TeamName string
	Score    float64
	Week     int
}

// FinalStanding represents final standing payout
type FinalStanding struct {
	TeamName string
	Rank     int
	Payout   int
}

// calculateStandings calculates standings with custom scoring
func (wg *WebsiteGenerator) calculateStandings() []Standing {
	teams := wg.reader.GetTeams()
	standings := make([]Standing, len(teams))

	// Calculate weekly high scorers for top half wins
	topHalfScorers := wg.getWeeklyTopHalfScorers()

	for i, team := range teams {
		h2hWins := wg.reader.GetTeamRecord(team.ID).Overall.Wins

		// Calculate top half wins (only for regular season)
		topHalfWins := 0
		for _, topHalfScoresForWeek := range topHalfScorers {
			for _, topHalfScorer := range topHalfScoresForWeek {
				if topHalfScorer.TeamID == team.ID {
					topHalfWins++
				}
			}
		}

		standings[i] = Standing{
			Team:                &team,
			TotalPointsScored:   team.Points,
			Record:              team.Record,
			H2HWins:             h2hWins,
			TopHalfWins:         topHalfWins,
			TotalStandingPoints: h2hWins + topHalfWins,
		}
	}

	// Sort by total points (descending), then by head-to-head wins, then by total fantasy points
	sort.Slice(standings, func(i, j int) bool {
		if standings[i].TotalStandingPoints != standings[j].TotalStandingPoints {
			return standings[i].TotalStandingPoints > standings[j].TotalStandingPoints
		}
		return standings[i].TotalPointsScored > standings[j].TotalPointsScored
	})

	// Assign ranks
	for i := range standings {
		standings[i].Rank = i + 1
	}

	return standings
}

// calculatePayouts calculates payout information
func (wg *WebsiteGenerator) calculatePayouts() Payout {
	weeklyHighScorers := wg.getWeeklyHighScorers()

	// Convert to payout format
	weeklyPayouts := make([]WeeklyHighScore, len(weeklyHighScorers))
	for i, hs := range weeklyHighScorers {
		team := wg.reader.GetTeamByID(hs.TeamID)
		weeklyPayouts[i] = WeeklyHighScore{
			TeamName: team.Name,
			Score:    hs.Score,
			Week:     hs.Week,
		}
	}

	teams := wg.reader.GetTeams()
	// Final standings payouts (top 3)
	finalPayouts := make([]FinalStanding, 0, 3)
	for _, team := range teams {
		payout := 0
		switch team.RankCalculatedFinal {
		case 1:
			payout = 550
		case 2:
			payout = 180
		case 3:
			payout = 100
		default:
			continue
		}

		finalPayouts = append(finalPayouts, FinalStanding{
			TeamName: team.Name,
			Rank:     team.RankCalculatedFinal,
			Payout:   payout,
		})
	}
	sort.Slice(finalPayouts, func(i, j int) bool {
		return finalPayouts[i].Rank < finalPayouts[j].Rank
	})

	return Payout{
		WeeklyHighScorers: weeklyPayouts,
		FinalStandings:    finalPayouts,
	}
}

// WeeklyHighScoreData represents weekly high score data
type WeeklyHighScoreData struct {
	TeamID int
	Score  float64
	Week   int
}

// getWeeklyHighScorers gets the highest scorer for each week
func (wg *WebsiteGenerator) getWeeklyHighScorers() []WeeklyHighScoreData {
	schedule := wg.reader.GetSchedule()
	weeklyScores := make(map[int][]WeeklyHighScoreData)

	// Group scores by week
	for _, matchup := range schedule {
		week := matchup.MatchupPeriodID

		// Add home team score
		weeklyScores[week] = append(weeklyScores[week], WeeklyHighScoreData{
			TeamID: matchup.Home.TeamID,
			Score:  matchup.Home.TotalPoints,
			Week:   week,
		})

		// Add away team score
		weeklyScores[week] = append(weeklyScores[week], WeeklyHighScoreData{
			TeamID: matchup.Away.TeamID,
			Score:  matchup.Away.TotalPoints,
			Week:   week,
		})
	}

	// Find highest scorer for each week
	var highScorers []WeeklyHighScoreData
	for weekNum, scores := range weeklyScores {
		if len(scores) == 0 {
			continue
		}

		highest := scores[0]
		for _, score := range scores {
			if score.Score > highest.Score {
				highest = score
			}
		}
		highest.Week = weekNum
		highScorers = append(highScorers, highest)
	}

	// Sort by week
	sort.Slice(highScorers, func(i, j int) bool {
		return highScorers[i].Week < highScorers[j].Week
	})

	return highScorers
}

func (wg *WebsiteGenerator) getWeeklyTopHalfScorers() [][]WeeklyHighScoreData {
	schedule := wg.reader.GetSchedule()
	weeklyScores := make(map[int][]WeeklyHighScoreData)

	// Group scores by week
	for _, matchup := range schedule {
		// skip playoff games
		if matchup.PlayoffTierType != "NONE" {
			continue
		}
		week := matchup.MatchupPeriodID

		// Add home team score
		weeklyScores[week] = append(weeklyScores[week], WeeklyHighScoreData{
			TeamID: matchup.Home.TeamID,
			Score:  matchup.Home.TotalPoints,
			Week:   week,
		})

		// Add away team score
		weeklyScores[week] = append(weeklyScores[week], WeeklyHighScoreData{
			TeamID: matchup.Away.TeamID,
			Score:  matchup.Away.TotalPoints,
			Week:   week,
		})
	}

	// Find highest scorer for each week
	var highScorers [][]WeeklyHighScoreData
	for _, scores := range weeklyScores {
		if len(scores) == 0 {
			continue
		}

		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score < scores[j].Score
		})
		mid := len(scores) / 2
		highScorers = append(highScorers, scores[mid:])
	}

	// Sort by week
	sort.Slice(highScorers, func(i, j int) bool {
		// we can always assume there is at least one game in a week
		return highScorers[i][0].Week < highScorers[j][0].Week
	})

	return highScorers
}

// getRecentGames gets recent games sorted by most recent first
func (wg *WebsiteGenerator) getRecentGames() []Matchup {
	schedule := wg.reader.GetSchedule()

	// Sort by matchup period ID (descending) and then by matchup ID (descending)
	sort.Slice(schedule, func(i, j int) bool {
		if schedule[i].MatchupPeriodID != schedule[j].MatchupPeriodID {
			return schedule[i].MatchupPeriodID > schedule[j].MatchupPeriodID
		}
		return schedule[i].ID > schedule[j].ID
	})

	return schedule
}

// prepareStandingsRows converts standings to template rows
func (wg *WebsiteGenerator) prepareStandingsRows(standings []Standing) []StandingRow {
	rows := make([]StandingRow, len(standings))

	for i, standing := range standings {
		record := standing.Record.Overall
		rowClass := "hover:bg-gray-50"
		if standing.Rank <= 6 {
			rowClass += " bg-yellow-50"
		}

		owner := wg.reader.GetMemberByID(standing.Team.PrimaryOwner)

		rows[i] = StandingRow{
			Rank:        standing.Rank,
			TeamName:    standing.Team.Name,
			OwnerName:   fmt.Sprintf("%s %s", owner.FirstName, owner.LastName),
			Points:      fmt.Sprintf("%.2f", standing.TotalPointsScored),
			Record:      fmt.Sprintf("%d-%d-%d", record.Wins, record.Losses, record.Ties),
			H2HWins:     standing.H2HWins,
			TopHalfWins: standing.TopHalfWins,
			TotalPoints: standing.TotalStandingPoints,
			RowClass:    rowClass,
		}
	}

	return rows
}

// prepareWeeklyHighScorerRows converts weekly high scorers to template rows
func (wg *WebsiteGenerator) prepareWeeklyHighScorerRows(highScorers []WeeklyHighScore) []WeeklyHighScoreRow {
	rows := make([]WeeklyHighScoreRow, len(highScorers))

	for i, hs := range highScorers {
		// Find the team to get the owner
		team := wg.reader.GetTeamByName(hs.TeamName)
		ownerName := "Unknown"
		if team != nil {
			owner := wg.reader.GetMemberByID(team.PrimaryOwner)
			if owner != nil {
				ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
			}
		}

		rows[i] = WeeklyHighScoreRow{
			TeamName:  hs.TeamName,
			OwnerName: ownerName,
			Score:     fmt.Sprintf("%.2f", hs.Score),
			Week:      hs.Week,
		}
	}

	return rows
}

// prepareFinalStandingRows converts final standings to template rows
func (wg *WebsiteGenerator) prepareFinalStandingRows(finalStandings []FinalStanding) []FinalStandingRow {
	rows := make([]FinalStandingRow, len(finalStandings))

	for i, fs := range finalStandings {
		// Find the team to get the owner
		team := wg.reader.GetTeamByName(fs.TeamName)
		ownerName := "Unknown"
		if team != nil {
			owner := wg.reader.GetMemberByID(team.PrimaryOwner)
			if owner != nil {
				ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
			}
		}

		rows[i] = FinalStandingRow{
			TeamName:  fs.TeamName,
			OwnerName: ownerName,
			Rank:      fs.Rank,
			Payout:    fs.Payout,
		}
	}

	return rows
}

// prepareGameRows converts games to template rows
func (wg *WebsiteGenerator) prepareGameRows(games []Matchup) []GameRow {
	var rows []GameRow

	// Get weekly data for determining top half and high scorers
	weeklyHighScorers := wg.getWeeklyHighScorers()
	topHalfScorers := wg.getWeeklyTopHalfScorers()

	// Create maps for quick lookup
	highScorerMap := make(map[int]int) // week -> teamID
	for _, hs := range weeklyHighScorers {
		highScorerMap[hs.Week] = hs.TeamID
	}

	topHalfMap := make(map[int]map[int]bool) // week -> teamID -> bool
	for _, weekTopHalf := range topHalfScorers {
		if len(weekTopHalf) > 0 {
			week := weekTopHalf[0].Week
			topHalfMap[week] = make(map[int]bool)
			for _, team := range weekTopHalf {
				topHalfMap[week][team.TeamID] = true
			}
		}
	}

	for _, game := range games {
		homeTeam := wg.reader.GetTeamByID(game.Home.TeamID)
		awayTeam := wg.reader.GetTeamByID(game.Away.TeamID)

		if homeTeam == nil || awayTeam == nil {
			continue
		}

		winner := "Tie"
		winnerClass := "text-gray-500"
		if game.Winner == "HOME" {
			winner = homeTeam.Name
			winnerClass = "text-fantasy-green font-bold"
		} else if game.Winner == "AWAY" {
			winner = awayTeam.Name
			winnerClass = "text-fantasy-green font-bold"
		}

		// Check if teams were in top half or highest scorer for this week
		week := game.MatchupPeriodID
		homeIsTopHalf := topHalfMap[week][game.Home.TeamID]
		awayIsTopHalf := topHalfMap[week][game.Away.TeamID]
		homeIsHighScore := highScorerMap[week] == game.Home.TeamID
		awayIsHighScore := highScorerMap[week] == game.Away.TeamID

		// Get owner names
		awayOwner := wg.reader.GetMemberByID(awayTeam.PrimaryOwner)
		homeOwner := wg.reader.GetMemberByID(homeTeam.PrimaryOwner)
		awayOwnerName := "Unknown"
		homeOwnerName := "Unknown"
		if awayOwner != nil {
			awayOwnerName = fmt.Sprintf("%s %s", awayOwner.FirstName, awayOwner.LastName)
		}
		if homeOwner != nil {
			homeOwnerName = fmt.Sprintf("%s %s", homeOwner.FirstName, homeOwner.LastName)
		}

		rows = append(rows, GameRow{
			Week:            week,
			MatchupID:       game.ID,
			AwayTeamName:    awayTeam.Name,
			AwayScore:       fmt.Sprintf("%.2f", game.Away.TotalPoints),
			AwayTeamID:      game.Away.TeamID,
			AwayOwnerName:   awayOwnerName,
			HomeTeamName:    homeTeam.Name,
			HomeScore:       fmt.Sprintf("%.2f", game.Home.TotalPoints),
			HomeTeamID:      game.Home.TeamID,
			HomeOwnerName:   homeOwnerName,
			Winner:          winner,
			WinnerClass:     winnerClass,
			AwayIsTopHalf:   awayIsTopHalf,
			AwayIsHighScore: awayIsHighScore,
			HomeIsTopHalf:   homeIsTopHalf,
			HomeIsHighScore: homeIsHighScore,
		})
	}

	return rows
}

// groupGamesByWeek groups games by week number and returns them sorted by week (descending)
func (wg *WebsiteGenerator) groupGamesByWeek(games []GameRow) []WeekGames {
	grouped := make(map[int][]GameRow)

	for _, game := range games {
		grouped[game.Week] = append(grouped[game.Week], game)
	}

	// Sort games within each week by matchup ID (descending)
	for week := range grouped {
		sort.Slice(grouped[week], func(i, j int) bool {
			return grouped[week][i].MatchupID > grouped[week][j].MatchupID
		})
	}

	// Convert to slice and sort by week (descending)
	var result []WeekGames
	for week, games := range grouped {
		result = append(result, WeekGames{Week: week, Games: games})
	}

	// Sort by week in descending order (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Week > result[j].Week
	})

	return result
}

// prepareTeamPayoutTotals calculates total payouts for each team
func (wg *WebsiteGenerator) prepareTeamPayoutTotals(weeklyHighScorers []WeeklyHighScore, finalStandings []FinalStanding) []TeamPayoutTotal {
	// Create a map to track payouts by team
	teamPayouts := make(map[string]*TeamPayoutTotal)

	// Initialize all teams with zero payouts
	teams := wg.reader.GetTeams()
	for _, team := range teams {
		owner := wg.reader.GetMemberByID(team.PrimaryOwner)
		ownerName := "Unknown"
		if owner != nil {
			ownerName = fmt.Sprintf("%s %s", owner.FirstName, owner.LastName)
		}

		teamPayouts[team.Name] = &TeamPayoutTotal{
			TeamName:            team.Name,
			OwnerName:           ownerName,
			WeeklyHighScores:    0,
			FinalStandingPayout: 0,
			TotalPayout:         0,
		}
	}

	// Count weekly high scores
	for _, hs := range weeklyHighScorers {
		if team, exists := teamPayouts[hs.TeamName]; exists {
			team.WeeklyHighScores++
			team.TotalPayout += 10 // $10 per weekly high score
		}
	}

	// Add final standing payouts
	for _, fs := range finalStandings {
		if team, exists := teamPayouts[fs.TeamName]; exists {
			team.FinalStandingPayout = fs.Payout
			team.TotalPayout += fs.Payout
		}
	}

	// Convert to slice and sort by total payout (descending)
	var result []TeamPayoutTotal
	for _, team := range teamPayouts {
		result = append(result, *team)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalPayout > result[j].TotalPayout
	})

	return result
}



// DraftPickRow represents a draft pick for the template
type DraftPickRow struct {
	OverallPickNumber int
	TeamName         string
	OwnerName        string
	PlayerName       string
	Position         string
	ProTeamName      string
	ProTeamAbbrev    string
	BidAmount        int
	IsKeeper         bool
}

// DraftData represents the data passed to the draft template
type DraftData struct {
	LeagueName    string
	SeasonYear    string
	LastUpdated   string
	GeneratedAt   string
	TotalPicks    int
	DraftDate     string
	DraftStatus   string
	DraftPicks    []DraftPickRow
	KeeperPicks   []DraftPickRow
}
