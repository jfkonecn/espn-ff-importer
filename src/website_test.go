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
