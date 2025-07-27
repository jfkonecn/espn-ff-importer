package main

// ESPNLeague represents the main league data structure
type ESPNLeague struct {
	DraftDetail      DraftDetail `json:"draftDetail"`
	GameID           int         `json:"gameId"`
	ID               int         `json:"id"`
	Members          []Member    `json:"members"`
	Schedule         []Matchup   `json:"schedule"`
	ScoringPeriodID  int         `json:"scoringPeriodId"`
	SeasonID         int         `json:"seasonId"`
	SegmentID        int         `json:"segmentId"`
	Settings         Settings    `json:"settings"`
	Status           Status      `json:"status"`
	Teams            []Team      `json:"teams"`
}

// DraftDetail contains draft information
type DraftDetail struct {
	Drafted    bool `json:"drafted"`
	InProgress bool `json:"inProgress"`
}

// Member represents a league member
type Member struct {
	DisplayName         string               `json:"displayName"`
	FirstName           string               `json:"firstName"`
	ID                  string               `json:"id"`
	LastName            string               `json:"lastName"`
	NotificationSettings []NotificationSetting `json:"notificationSettings"`
}

// NotificationSetting represents notification preferences
type NotificationSetting struct {
	Enabled bool   `json:"enabled"`
	ID      string `json:"id"`
	Type    string `json:"type"`
}

// Matchup represents a game between two teams
type Matchup struct {
	Away             TeamScore `json:"away"`
	Home             TeamScore `json:"home"`
	ID               int       `json:"id"`
	MatchupPeriodID  int       `json:"matchupPeriodId"`
	PlayoffTierType  string    `json:"playoffTierType"`
	Winner           string    `json:"winner"`
}

// TeamScore represents a team's score in a matchup
type TeamScore struct {
	Adjustment        float64                `json:"adjustment"`
	CumulativeScore   CumulativeScore        `json:"cumulativeScore"`
	PointsByScoringPeriod map[string]float64 `json:"pointsByScoringPeriod"`
	TeamID            int                    `json:"teamId"`
	Tiebreak          float64                `json:"tiebreak"`
	TotalPoints       float64                `json:"totalPoints"`
}

// CumulativeScore represents cumulative statistics
type CumulativeScore struct {
	Losses     int                    `json:"losses"`
	ScoreByStat map[string]StatScore  `json:"scoreByStat"`
	StatBySlot interface{}            `json:"statBySlot"`
	Ties       int                    `json:"ties"`
	Wins       int                    `json:"wins"`
}

// StatScore represents individual stat scoring
type StatScore struct {
	Ineligible bool    `json:"ineligible"`
	Rank       float64 `json:"rank"`
	Result     interface{} `json:"result"`
	Score      float64 `json:"score"`
}

// Settings represents league settings
type Settings struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	// Add other fields as needed based on the actual data structure
}

// Status represents league status
type Status struct {
	// Add fields as needed based on the actual data structure
}

// Team represents a fantasy team
type Team struct {
	Abbrev                string      `json:"abbrev"`
	CurrentProjectedRank  int         `json:"currentProjectedRank"`
	DivisionID            int         `json:"divisionId"`
	DraftDayProjectedRank int         `json:"draftDayProjectedRank"`
	ID                    int         `json:"id"`
	IsActive              bool        `json:"isActive"`
	Logo                  string      `json:"logo"`
	LogoType              string      `json:"logoType"`
	Name                  string      `json:"name"`
	Owners                []string    `json:"owners"`
	PlayoffSeed           int         `json:"playoffSeed"`
	Points                float64     `json:"points"`
	PointsAdjusted        float64     `json:"pointsAdjusted"`
	PointsDelta           float64     `json:"pointsDelta"`
	PrimaryOwner          string      `json:"primaryOwner"`
	RankCalculatedFinal   int         `json:"rankCalculatedFinal"`
	RankFinal             int         `json:"rankFinal"`
	Record                Record      `json:"record"`
	TransactionCounter    TransactionCounter `json:"transactionCounter"`
	ValuesByStat          interface{} `json:"valuesByStat"`
	WaiverRank            int         `json:"waiverRank"`
}

// TransactionCounter represents team transaction statistics
type TransactionCounter struct {
	AcquisitionBudgetSpent int                    `json:"acquisitionBudgetSpent"`
	Acquisitions           int                    `json:"acquisitions"`
	Drops                  int                    `json:"drops"`
	MatchupAcquisitionTotals map[string]int       `json:"matchupAcquisitionTotals"`
	Misc                   int                    `json:"misc"`
	MoveToActive           int                    `json:"moveToActive"`
	MoveToIR               int                    `json:"moveToIR"`
	Paid                   float64                `json:"paid"`
	TeamCharges            float64                `json:"teamCharges"`
	Trades                 int                    `json:"trades"`
}

// Record represents team's win/loss record
type Record struct {
	Away     GameRecord `json:"away"`
	Division GameRecord `json:"division"`
	Home     GameRecord `json:"home"`
	Overall  GameRecord `json:"overall"`
}

// GameRecord represents wins, losses, and ties
type GameRecord struct {
	GamesBack float64 `json:"gamesBack"`
	Losses    int     `json:"losses"`
	Percentage float64 `json:"percentage"`
	PointsAgainst float64 `json:"pointsAgainst"`
	PointsFor float64 `json:"pointsFor"`
	Ties      int     `json:"ties"`
	Wins      int     `json:"wins"`
} 