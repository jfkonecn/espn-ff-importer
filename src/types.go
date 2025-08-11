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
	CompleteDate int64      `json:"completeDate"`
	Drafted      bool       `json:"drafted"`
	InProgress   bool       `json:"inProgress"`
	Picks        []DraftPick `json:"picks"`
}

// DraftPick represents a single draft pick
type DraftPick struct {
	AutoDraftTypeId     int     `json:"autoDraftTypeId"`
	BidAmount           int     `json:"bidAmount"`
	ID                  int     `json:"id"`
	Keeper              bool    `json:"keeper"`
	LineupSlotId        int     `json:"lineupSlotId"`
	MemberID            string  `json:"memberId"`
	NominatingTeamID    int     `json:"nominatingTeamId"`
	OverallPickNumber   int     `json:"overallPickNumber"`
	PlayerID            int     `json:"playerId"`
	ReservedForKeeper   bool    `json:"reservedForKeeper"`
	RoundID             int     `json:"roundId"`
	RoundPickNumber     int     `json:"roundPickNumber"`
	TeamID              int     `json:"teamId"`
	TradeLocked         bool    `json:"tradeLocked"`
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

// LineupSlotID represents the different lineup positions in fantasy football
type LineupSlotID int

const (
	QB LineupSlotID = iota
	RB
	WR
	TE
	FLEX
	SUPER_FLEX
	DEF
	K
	BENCH
	IR
	NA
)

// String returns the string representation of the lineup slot
func (l LineupSlotID) String() string {
	switch l {
	case QB:
		return "QB"
	case RB:
		return "RB"
	case WR:
		return "WR"
	case TE:
		return "TE"
	case FLEX:
		return "FLEX"
	case SUPER_FLEX:
		return "SUPER_FLEX"
	case DEF:
		return "DEF"
	case K:
		return "K"
	case BENCH:
		return "BENCH"
	case IR:
		return "IR"
	case NA:
		return "NA"
	default:
		return "UNKNOWN"
	}
}

// TeamScore represents a team's score in a matchup
type TeamScore struct {
	Adjustment        float64                `json:"adjustment"`
	CumulativeScore   CumulativeScore        `json:"cumulativeScore"`
	LineupSlotID      LineupSlotID           `json:"lineupSlotId"`
	PointsByScoringPeriod map[string]float64 `json:"pointsByScoringPeriod"`
	RosterForCurrentScoringPeriod *TeamRoster `json:"rosterForCurrentScoringPeriod,omitempty"`
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
	Roster                *TeamRoster `json:"roster,omitempty"`
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

// GameRecord represents individual game statistics
type GameRecord struct {
	GamesBack float64 `json:"gamesBack"`
	Losses    int     `json:"losses"`
	Percentage float64 `json:"percentage"`
	PointsAgainst float64 `json:"pointsAgainst"`
	PointsFor float64 `json:"pointsFor"`
	Ties      int     `json:"ties"`
	Wins      int     `json:"wins"`
}

// Player represents a fantasy football player
type Player struct {
	ID                 int     `json:"id"`
	FirstName          string  `json:"firstName"`
	LastName           string  `json:"lastName"`
	FullName           string  `json:"fullName"`
	DefaultPositionID  int     `json:"defaultPositionId"`
	ProTeamID          int     `json:"proTeamId"`
	UniverseID         int     `json:"universeId"`
	Droppable          bool    `json:"droppable"`
	EligibleSlots      []int   `json:"eligibleSlots"`
	Ownership          Ownership `json:"ownership"`
}

// Ownership represents player ownership information
type Ownership struct {
	PercentOwned float64 `json:"percentOwned"`
}

// ProTeam represents an NFL team
type ProTeam struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Abbrev string `json:"abbrev"`
	Location string `json:"location"`
	ByeWeek int    `json:"byeWeek"`
}

// ProTeamsData represents the pro teams data structure
type ProTeamsData struct {
	Display bool `json:"display"`
	Settings struct {
		ProTeams []ProTeam `json:"proTeams"`
	} `json:"settings"`
}

// KeeperEligibility represents keeper eligibility information
type KeeperEligibility struct {
	PlayerID        int
	PlayerName      string
	TeamName        string
	OwnerName       string
	Position        string
	ProTeamName     string
	ProTeamAbbrev   string
	IsEligible      bool
	KeeperYears     int
	AcquisitionType string // "draft" or "free_agency"
	CurrentPrice    int
	NextYearPrice   int
	IsKeeper        bool
}

// PlayerHistory represents a player's history for keeper calculations
type PlayerHistory struct {
	PlayerID        int
	PlayerName      string
	FirstAcquired   int    // Season when first acquired
	AcquisitionType string // "draft" or "free_agency"
	OriginalPrice   int    // Original acquisition price
	YearsKept       int    // Number of years kept so far
	LastKeptSeason  int    // Last season this player was kept
}

// RosterEntry represents a player on a team's roster
type RosterEntry struct {
	InjuryStatus string `json:"injuryStatus"`
	LineupSlotID int    `json:"lineupSlotId"`
	PlayerID     int    `json:"playerId"`
	PlayerPoolEntry RosterPlayerPoolEntry `json:"playerPoolEntry"`
}

// RosterPlayerPoolEntry represents the player pool entry for a roster player
type RosterPlayerPoolEntry struct {
	AppliedStatTotal float64 `json:"appliedStatTotal"`
	ID               int     `json:"id"`
	OnTeamID         int     `json:"onTeamId"`
	Player           RosterPlayer `json:"player"`
}

// RosterPlayer represents a player in the roster
type RosterPlayer struct {
	Active             bool    `json:"active"`
	DefaultPositionID  int     `json:"defaultPositionId"`
	EligibleSlots     []int   `json:"eligibleSlots"`
	FirstName         string  `json:"firstName"`
	FullName          string  `json:"fullName"`
	ID                int     `json:"id"`
	Injured           bool    `json:"injured"`
	InjuryStatus      string  `json:"injuryStatus"`
	Jersey            string  `json:"jersey"`
	LastName          string  `json:"lastName"`
	ProTeamID         int     `json:"proTeamId"`
	Stats             []RosterStat `json:"stats"`
}

// RosterStat represents player statistics
type RosterStat struct {
	AppliedStats map[string]float64 `json:"appliedStats"`
	AppliedTotal float64             `json:"appliedTotal"`
}

// TeamRoster represents a team's roster for a matchup period
type TeamRoster struct {
	AppliedStatTotal float64      `json:"appliedStatTotal"`
	Entries          []RosterEntry `json:"entries"`
}