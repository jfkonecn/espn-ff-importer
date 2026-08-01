package podcast

type SeasonPhase string

const (
	PhaseDraft          SeasonPhase = "draft"
	PhaseRegularSeason  SeasonPhase = "regular-season"
	PhasePostSeason     SeasonPhase = "post-season"
	PhaseSeasonComplete SeasonPhase = "season-complete"
)

type SeasonState struct {
	Season            int         `json:"season"`
	LeagueName        string      `json:"leagueName"`
	Phase             SeasonPhase `json:"phase"`
	Week              int         `json:"week,omitempty"`
	DraftComplete     bool        `json:"draftComplete"`
	CompletedMatchups int         `json:"completedMatchups"`
	TotalMatchups     int         `json:"totalMatchups"`
	GeneratedAt       string      `json:"generatedAt"`
}

type CommercialRead struct {
	CompanyName string `json:"companyName"`
	Tagline     string `json:"tagline"`
	Read        string `json:"read"`
}

type PodcastSegment struct {
	Name       string `json:"name"`
	Transcript string `json:"transcript"`
}

type PodcastScript struct {
	EpisodeID            string           `json:"episodeId"`
	AudioFile            string           `json:"audioFile"`
	Title                string           `json:"title"`
	Description          string           `json:"description"`
	Summary              string           `json:"summary"`
	PubDate              string           `json:"pubDate"`
	Duration             string           `json:"duration,omitempty"`
	Explicit             bool             `json:"explicit"`
	EpisodeType          string           `json:"episodeType"`
	PotentialCommercials []CommercialRead `json:"potentialCommercials"`
	Commercials          []CommercialRead `json:"commercials"`
	Segments             []PodcastSegment `json:"segments"`
	Transcript           string           `json:"transcript"`
	SeasonContext        SeasonState      `json:"seasonContext"`
	SourceFiles          []string         `json:"sourceFiles"`
}

type PodcastMetadata struct {
	Channel  PodcastChannel           `json:"channel"`
	Episodes []PodcastEpisodeMetadata `json:"episodes"`
}

type PodcastChannel struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Language    string `json:"language"`
	Explicit    bool   `json:"explicit"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Image       string `json:"image"`
	OwnerName   string `json:"ownerName"`
	OwnerEmail  string `json:"ownerEmail"`
	Copyright   string `json:"copyright"`
}

type PodcastEpisodeMetadata struct {
	File        string `json:"file"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PubDate     string `json:"pubDate"`
	Duration    string `json:"duration"`
	Explicit    bool   `json:"explicit"`
	EpisodeType string `json:"episodeType"`
}
