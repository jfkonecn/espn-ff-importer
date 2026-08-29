package podcast

import "testing"

func TestNormalizeScriptIdentityUsesCanonicalEpisodeID(t *testing.T) {
	script := PodcastScript{
		EpisodeID: "ep001",
		AudioFile: "https://example.com/podcast/episode1.mp3",
	}

	NormalizeScriptIdentity(&script, "2026_PostDraft")

	if script.EpisodeID != "2026_PostDraft" {
		t.Fatalf("EpisodeID = %q, want %q", script.EpisodeID, "2026_PostDraft")
	}
	if script.AudioFile != "2026_PostDraft.mp3" {
		t.Fatalf("AudioFile = %q, want %q", script.AudioFile, "2026_PostDraft.mp3")
	}
}
