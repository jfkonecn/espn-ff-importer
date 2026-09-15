package main

import "testing"

func TestReacquiredPlayerUsesHigherOfDraftPriceOrFreeAgencyMinimum(t *testing.T) {
	wg := &WebsiteGenerator{}
	for _, test := range []struct {
		name        string
		draftPrice  int
		keeperYears int
		want        int
	}{
		{name: "first year draft price below minimum", draftPrice: 8, keeperYears: 0, want: 15},
		{name: "first year draft price above minimum", draftPrice: 22, keeperYears: 0, want: 22},
		{name: "second year draft price below minimum", draftPrice: 18, keeperYears: 1, want: 22},
		{name: "second year draft price above minimum", draftPrice: 30, keeperYears: 1, want: 30},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := wg.calculateNextYearKeeperPrice(test.draftPrice, "free_agency_readd", test.keeperYears); got != test.want {
				t.Fatalf("calculateNextYearKeeperPrice(%d, %d) = %d, want %d", test.draftPrice, test.keeperYears, got, test.want)
			}
		})
	}
}

func TestWasReacquiredThroughFreeAgency(t *testing.T) {
	wg := &WebsiteGenerator{
		historicalReaders: map[int]*LeagueReader{
			2025: {league: &ESPNLeague{Teams: []Team{{ID: 7, Name: "Example Team"}}}},
		},
		historicalTransactions: map[int]TransactionHistory{
			2025: {Transactions: []Transaction{
				{Status: "EXECUTED", Type: "WAIVER", Items: []TransactionItem{{Type: "ADD", PlayerID: 42, ToTeamID: 7}}},
				{Status: "EXECUTED", Type: "TRADE", Items: []TransactionItem{{Type: "ADD", PlayerID: 99, ToTeamID: 7}}},
				{Status: "PENDING", Type: "WAIVER", Items: []TransactionItem{{Type: "ADD", PlayerID: 100, ToTeamID: 7}}},
			}},
		},
	}

	if !wg.wasReacquiredThroughFreeAgency(42, "Example Team", 2025) {
		t.Fatal("expected executed waiver add to be recognized")
	}
	if wg.wasReacquiredThroughFreeAgency(99, "Example Team", 2025) {
		t.Fatal("trade add must not be treated as a free-agency re-add")
	}
	if wg.wasReacquiredThroughFreeAgency(100, "Example Team", 2025) {
		t.Fatal("pending add must not be treated as a free-agency re-add")
	}
}

func TestTopHalfScoringUsesCurrentTeamCount(t *testing.T) {
	var teams []Team
	var schedule []Matchup
	for i := 1; i <= 12; i++ {
		teams = append(teams, Team{
			ID:     i,
			Name:   "Team",
			Points: float64(i),
		})
	}
	for i := 1; i <= 6; i++ {
		schedule = append(schedule, Matchup{
			MatchupPeriodID: 1,
			PlayoffTierType: "NONE",
			Winner:          "HOME",
			Home: TeamScore{
				TeamID:      i,
				TotalPoints: float64(i),
			},
			Away: TeamScore{
				TeamID:      i + 6,
				TotalPoints: float64(i + 6),
			},
		})
	}

	wg := &WebsiteGenerator{
		reader: &LeagueReader{league: &ESPNLeague{
			Teams:    teams,
			Schedule: schedule,
		}},
	}

	topHalfScorers := wg.getWeeklyTopHalfScorers()
	if len(topHalfScorers) != 1 {
		t.Fatalf("got %d top-half weeks, want 1", len(topHalfScorers))
	}
	if len(topHalfScorers[0]) != 6 {
		t.Fatalf("got %d top-half scorers, want 6 for a 12-team week", len(topHalfScorers[0]))
	}

	topHalfByTeamID := make(map[int]bool)
	for _, scorer := range topHalfScorers[0] {
		topHalfByTeamID[scorer.TeamID] = true
	}
	for teamID := 1; teamID <= 12; teamID++ {
		wantTopHalf := teamID >= 7
		if topHalfByTeamID[teamID] != wantTopHalf {
			t.Fatalf("team %d top-half = %v, want %v", teamID, topHalfByTeamID[teamID], wantTopHalf)
		}
	}

	standings := wg.calculateStandings()
	topHalfWinsByTeamID := make(map[int]int)
	for _, standing := range standings {
		topHalfWinsByTeamID[standing.Team.ID] = standing.TopHalfWins
	}
	for teamID := 1; teamID <= 12; teamID++ {
		wantWins := 0
		if teamID >= 7 {
			wantWins = 1
		}
		if topHalfWinsByTeamID[teamID] != wantWins {
			t.Fatalf("team %d TopHalfWins = %d, want %d", teamID, topHalfWinsByTeamID[teamID], wantWins)
		}
	}
}

func TestPayoutsUse2026AmountsFor2026AndBeyond(t *testing.T) {
	for _, test := range []struct {
		season      int
		weekly      int
		firstPlace  int
		secondPlace int
		thirdPlace  int
	}{
		{season: 2025, weekly: 10, firstPlace: 550, secondPlace: 180, thirdPlace: 100},
		{season: 2026, weekly: 15, firstPlace: 650, secondPlace: 195, thirdPlace: 100},
		{season: 2027, weekly: 15, firstPlace: 650, secondPlace: 195, thirdPlace: 100},
	} {
		t.Run("season", func(t *testing.T) {
			wg := &WebsiteGenerator{reader: &LeagueReader{league: &ESPNLeague{SeasonID: test.season}}}

			if got := wg.weeklyHighScorePayout(); got != test.weekly {
				t.Fatalf("weeklyHighScorePayout() = %d, want %d", got, test.weekly)
			}
			if got := wg.finalStandingPayout(1); got != test.firstPlace {
				t.Fatalf("finalStandingPayout(1) = %d, want %d", got, test.firstPlace)
			}
			if got := wg.finalStandingPayout(2); got != test.secondPlace {
				t.Fatalf("finalStandingPayout(2) = %d, want %d", got, test.secondPlace)
			}
			if got := wg.finalStandingPayout(3); got != test.thirdPlace {
				t.Fatalf("finalStandingPayout(3) = %d, want %d", got, test.thirdPlace)
			}
			if got := wg.finalStandingPayout(4); got != 0 {
				t.Fatalf("finalStandingPayout(4) = %d, want 0", got)
			}
		})
	}
}
