package podcast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const openAIBaseURL = "https://api.openai.com/v1"

type responseContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type responseInputItem struct {
	Type    string            `json:"type,omitempty"`
	Role    string            `json:"role,omitempty"`
	CallID  string            `json:"call_id,omitempty"`
	Output  string            `json:"output,omitempty"`
	Content []responseContent `json:"content,omitempty"`
}

type responseOutputItem struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Role      string            `json:"role"`
	Name      string            `json:"name"`
	CallID    string            `json:"call_id"`
	Arguments string            `json:"arguments"`
	Content   []responseContent `json:"content"`
}

type responseAPIResult struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Output []responseOutputItem `json:"output"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type podcastOutline struct {
	EpisodeID            string           `json:"episodeId"`
	AudioFile            string           `json:"audioFile"`
	Title                string           `json:"title"`
	Description          string           `json:"description"`
	Summary              string           `json:"summary"`
	PubDate              string           `json:"pubDate"`
	Duration             string           `json:"duration"`
	Explicit             bool             `json:"explicit"`
	EpisodeType          string           `json:"episodeType"`
	PotentialCommercials []CommercialRead `json:"potentialCommercials"`
	Commercials          []CommercialRead `json:"commercials"`
	SegmentPlans         []PodcastSegment `json:"segmentPlans"`
	SourceFiles          []string         `json:"sourceFiles"`
}

type structuredResponse struct {
	Text        string
	SourceFiles []string
	ResponseID  string
}

func GenerateTranscript(apiKey, model, aiRoot string, state SeasonState) (PodcastScript, error) {
	aiFiles, err := ListAIFiles(aiRoot)
	if err != nil {
		return PodcastScript{}, err
	}
	fmt.Printf("Found %d AI data files under %s\n", len(aiFiles), aiRoot)

	episodeID := DefaultEpisodeID(state)
	if model == "" {
		model = "gpt-4.1"
	}
	fmt.Printf("Requesting podcast outline from OpenAI using episode ID %s\n", episodeID)

	phaseInstructions := podcastPhaseInstructions(state)

	outlineSystemPrompt := `You are planning a fantasy football podcast called Slop Take. Write in the spirit of loud, confrontational sports-talk radio: clipped cadence, sharp resets, scoreboard justice, call-out energy, and memorable recurring phrases. Do not claim to be Jim Rome or imitate any living broadcaster verbatim.

Plan exactly four main segments in this order: Intro, Best Team, Worst Team, Final Take. The Intro plan must identify the fantasy league storylines, the owners or teams under pressure, the stakes for this episode, and a clear roadmap for the rest of the show. Plan one fantasy-football-themed fake commercial read after the Intro and another fake commercial read immediately before Final Take. Invent a list of potential fake fantasy-football sponsors, then select two for the reads.

Every segment plan must use league data and current NFL context. Use web search to look up the latest NFL news, injuries, depth chart changes, camp reports, trades, suspensions, and role changes before planning the episode. Do not invent specific breaking news; rely on searched current context when making NFL-news claims.

Return only valid JSON matching the requested schema. This is an outline and metadata pass, not the full transcript. Keep duration as an empty string; the publisher will not know the real duration until audio exists.`

	outlineUserPrompt := fmt.Sprintf(`Season state:
%s

Available files under ai/:
%s

Phase-specific assignment:
%s

Use the read_ai_file tool to inspect league data in ai/. Use web search for current NFL news and injury context. Produce podcast metadata, two selected commercials, and concise plans for the four required segments. Use episodeId %q and audioFile %q.`, mustJSON(state), strings.Join(aiFiles, "\n"), phaseInstructions, episodeID, episodeID+".mp3")

	outlineResult, err := generateStructured(apiKey, model, aiRoot, "outline", outlineSystemPrompt, outlineUserPrompt, podcastOutlineSchema(), true)
	if err != nil {
		return PodcastScript{}, err
	}

	var outline podcastOutline
	if err := json.Unmarshal([]byte(outlineResult.Text), &outline); err != nil {
		return PodcastScript{}, fmt.Errorf("failed to parse podcast outline JSON: %w", err)
	}
	fillOutlineDefaults(&outline, state, episodeID)
	fmt.Printf("Generated outline title=%q selectedCommercials=%d segmentPlans=%d\n", outline.Title, len(outline.Commercials), len(outline.SegmentPlans))

	var segments []PodcastSegment
	var transcriptParts []string
	allSourceFiles := append([]string{}, outline.SourceFiles...)
	allSourceFiles = append(allSourceFiles, outlineResult.SourceFiles...)

	segmentNames := []string{"Intro", "Best Team", "Worst Team", "Final Take"}
	for i, segmentName := range segmentNames {
		if i == 1 && len(outline.Commercials) > 0 {
			commercial, files, err := generateCommercial(apiKey, model, aiRoot, state, outline, outline.Commercials[0], 1)
			if err != nil {
				return PodcastScript{}, err
			}
			outline.Commercials[0] = commercial
			transcriptParts = append(transcriptParts, commercial.Read)
			allSourceFiles = append(allSourceFiles, files...)
		}

		segment, files, err := generateSegment(apiKey, model, aiRoot, state, outline, segmentName, segmentPlan(outline, segmentName))
		if err != nil {
			return PodcastScript{}, err
		}
		segments = append(segments, segment)
		transcriptParts = append(transcriptParts, segment.Transcript)
		allSourceFiles = append(allSourceFiles, files...)

		if i == 2 && len(outline.Commercials) > 1 {
			commercial, files, err := generateCommercial(apiKey, model, aiRoot, state, outline, outline.Commercials[1], 2)
			if err != nil {
				return PodcastScript{}, err
			}
			outline.Commercials[1] = commercial
			transcriptParts = append(transcriptParts, commercial.Read)
			allSourceFiles = append(allSourceFiles, files...)
		}
	}

	script := PodcastScript{
		EpisodeID:            outline.EpisodeID,
		AudioFile:            outline.AudioFile,
		Title:                outline.Title,
		Description:          outline.Description,
		Summary:              outline.Summary,
		PubDate:              outline.PubDate,
		Duration:             outline.Duration,
		Explicit:             outline.Explicit,
		EpisodeType:          outline.EpisodeType,
		PotentialCommercials: outline.PotentialCommercials,
		Commercials:          outline.Commercials,
		Segments:             segments,
		Transcript:           strings.Join(transcriptParts, "\n\n"),
		SeasonContext:        state,
		SourceFiles:          uniqueStrings(allSourceFiles),
	}
	fillScriptDefaults(&script, state, episodeID)
	fmt.Printf("Generated script title=%q audioFile=%s transcriptChars=%d\n", script.Title, script.AudioFile, len(script.Transcript))
	return script, nil
}

func SynthesizeSpeech(apiKey, model, voice, input, outputPath string) error {
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	if voice == "" {
		voice = "ballad"
	}

	payload := map[string]string{
		"model":           model,
		"voice":           voice,
		"input":           input,
		"response_format": "mp3",
	}
	fmt.Printf("Sending %d characters to OpenAI TTS\n", len(input))

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, openAIBaseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		contents, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI TTS failed with %s: %s", resp.Status, string(contents))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %d bytes of podcast audio to %s\n", written, outputPath)
	return nil
}

func generateSegment(apiKey, model, aiRoot string, state SeasonState, outline podcastOutline, segmentName, plan string) (PodcastSegment, []string, error) {
	fmt.Printf("Generating %s segment with about 3 minutes of copy\n", segmentName)
	systemPrompt := `You are writing one segment for Slop Take, a fantasy football podcast with loud, confrontational sports-talk energy: clipped cadence, sharp resets, scoreboard justice, call-out energy, and memorable phrases. Do not claim to be Jim Rome or imitate any living broadcaster verbatim.

Write only this one segment. Target about 3 minutes when read aloud, roughly 390 to 480 words. Make it TTS-ready: no markdown, no bullets, no stage directions, no URLs, and no citations. Use web search for current NFL news and injury context before writing this segment. Do not invent specific breaking news; rely on searched current context when making NFL-news claims. Every NFL news item you mention must be tied directly back to what is happening in this fantasy league: team outlooks, roster strengths or weaknesses, draft posture, standings pressure, owner decisions, keeper value, matchup consequences, trades, waivers, starts, or sits. Return only valid JSON matching the schema.`
	if segmentName == "Intro" {
		systemPrompt += " For the Intro, focus on the fantasy league's current storylines: the season phase, the teams under pressure, the owners who need to hear it, and the stakes of this episode. Set the table for what the podcast will cover instead of giving a generic welcome. Preview the Best Team, Worst Team, and Final Take angles without resolving them yet."
	}

	userPrompt := fmt.Sprintf(`Season state:
%s

Episode outline:
%s

Segment name: %s
Segment plan: %s

Phase-specific assignment:
%s

Use read_ai_file for league context if needed. Write the segment as a complete standalone block with a strong opening, escalating middle, and clean landing.`, mustJSON(state), mustJSON(outline), segmentName, plan, podcastPhaseInstructions(state))

	result, err := generateStructured(apiKey, model, aiRoot, "segment "+segmentName, systemPrompt, userPrompt, podcastSegmentSchema(), true)
	if err != nil {
		return PodcastSegment{}, nil, err
	}

	var segment PodcastSegment
	if err := json.Unmarshal([]byte(result.Text), &segment); err != nil {
		return PodcastSegment{}, nil, fmt.Errorf("failed to parse %s segment JSON: %w", segmentName, err)
	}
	if segment.Name == "" {
		segment.Name = segmentName
	}
	fmt.Printf("Generated %s segment with %d characters\n", segment.Name, len(segment.Transcript))
	return segment, result.SourceFiles, nil
}

func generateCommercial(apiKey, model, aiRoot string, state SeasonState, outline podcastOutline, planned CommercialRead, number int) (CommercialRead, []string, error) {
	fmt.Printf("Generating commercial %d with about 30 seconds of copy\n", number)
	systemPrompt := `You are writing one fake commercial read for Slop Take, a fantasy football podcast. The sponsor must be fictional and fantasy-football themed. Target about 30 seconds when read aloud, roughly 70 to 90 words. Make it TTS-ready: no markdown, no bullets, no stage directions, no URLs, and no citations. Return only valid JSON matching the schema.`
	userPrompt := fmt.Sprintf(`Season state:
%s

Episode outline:
%s

Planned sponsor:
%s

Write commercial read number %d. It should sound like a live ad read that fits the show voice without being a real company.`, mustJSON(state), mustJSON(outline), mustJSON(planned), number)

	result, err := generateStructured(apiKey, model, aiRoot, fmt.Sprintf("commercial %d", number), systemPrompt, userPrompt, commercialReadSchema(), false)
	if err != nil {
		return CommercialRead{}, nil, err
	}

	var commercial CommercialRead
	if err := json.Unmarshal([]byte(result.Text), &commercial); err != nil {
		return CommercialRead{}, nil, fmt.Errorf("failed to parse commercial %d JSON: %w", number, err)
	}
	fmt.Printf("Generated commercial %d for %q with %d characters\n", number, commercial.CompanyName, len(commercial.Read))
	return commercial, result.SourceFiles, nil
}

func generateStructured(apiKey, model, aiRoot, label, systemPrompt, userPrompt string, schema map[string]any, includeTools bool) (structuredResponse, error) {
	input := []responseInputItem{
		{Role: "system", Content: []responseContent{{Type: "input_text", Text: systemPrompt}}},
		{Role: "user", Content: []responseContent{{Type: "input_text", Text: userPrompt}}},
	}

	previousResponseID := ""
	var sourceFiles []string
	for iteration := range 8 {
		fmt.Printf("OpenAI %s iteration %d\n", label, iteration+1)
		result, err := createResponse(apiKey, model, input, previousResponseID, includeTools, schema)
		if err != nil {
			return structuredResponse{}, err
		}
		previousResponseID = result.ID
		logOpenAIOutput(result.Output)

		var toolOutputs []responseInputItem
		for _, item := range result.Output {
			if item.Type == "function_call" && item.Name == "read_ai_file" {
				var args struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal([]byte(item.Arguments), &args); err != nil {
					toolOutputs = append(toolOutputs, responseInputItem{Type: "function_call_output", CallID: item.CallID, Output: fmt.Sprintf("invalid JSON arguments: %v", err)})
					continue
				}

				fmt.Printf("OpenAI requested AI file: %s\n", args.Path)
				contents, err := SafeReadAIFile(aiRoot, args.Path)
				if err != nil {
					contents = "Error: " + err.Error()
					fmt.Printf("AI file read failed for %s: %v\n", args.Path, err)
				} else {
					sourceFiles = append(sourceFiles, args.Path)
					fmt.Printf("Read %d bytes from ai/%s\n", len(contents), args.Path)
				}
				toolOutputs = append(toolOutputs, responseInputItem{Type: "function_call_output", CallID: item.CallID, Output: contents})
			}
		}

		if len(toolOutputs) == 0 {
			fmt.Printf("OpenAI returned final JSON for %s\n", label)
			if result.Status == "incomplete" {
				reason := "unknown"
				if result.IncompleteDetails != nil && result.IncompleteDetails.Reason != "" {
					reason = result.IncompleteDetails.Reason
				}
				return structuredResponse{}, fmt.Errorf("OpenAI returned an incomplete %s response: %s", label, reason)
			}
			text := outputText(result.Output)
			if text == "" {
				return structuredResponse{}, fmt.Errorf("OpenAI response did not include JSON for %s", label)
			}
			return structuredResponse{Text: text, SourceFiles: uniqueStrings(sourceFiles), ResponseID: result.ID}, nil
		}

		fmt.Printf("Sending %d AI file tool outputs back to OpenAI for %s\n", len(toolOutputs), label)
		input = toolOutputs
	}

	return structuredResponse{}, fmt.Errorf("OpenAI tool loop exceeded maximum iterations for %s", label)
}

func createResponse(apiKey, model string, input []responseInputItem, previousResponseID string, includeTools bool, schema map[string]any) (responseAPIResult, error) {
	payload := map[string]any{
		"model":             model,
		"input":             input,
		"max_output_tokens": 20000,
		"text": map[string]any{
			"format": schema,
		},
	}
	if previousResponseID != "" {
		payload["previous_response_id"] = previousResponseID
	}
	if includeTools {
		payload["tools"] = []map[string]any{{
			"type":        "function",
			"name":        "read_ai_file",
			"description": "Read a markdown or JSON file under the repository ai/ directory.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Path relative to ai/, such as 2026/standings.md."},
				},
				"required":             []string{"path"},
				"additionalProperties": false,
			},
		}, {
			"type": "web_search_preview",
		}}
	}
	fmt.Printf("Calling OpenAI Responses API with %d input item(s)\n", len(input))

	body, err := json.Marshal(payload)
	if err != nil {
		return responseAPIResult{}, err
	}

	req, err := http.NewRequest(http.MethodPost, openAIBaseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return responseAPIResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return responseAPIResult{}, err
	}
	defer resp.Body.Close()

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return responseAPIResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseAPIResult{}, fmt.Errorf("OpenAI response failed with %s: %s", resp.Status, string(contents))
	}

	var result responseAPIResult
	if err := json.Unmarshal(contents, &result); err != nil {
		return responseAPIResult{}, err
	}
	if result.Error != nil {
		return responseAPIResult{}, fmt.Errorf("OpenAI response error: %s", result.Error.Message)
	}
	return result, nil
}

func podcastScriptSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type":   "json_schema",
		"name":   "podcast_script",
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required": []string{
				"episodeId", "audioFile", "title", "description", "summary", "pubDate", "duration", "explicit", "episodeType", "potentialCommercials", "commercials", "segments", "transcript", "seasonContext", "sourceFiles",
			},
			"properties": map[string]any{
				"episodeId":            stringSchema,
				"audioFile":            stringSchema,
				"title":                stringSchema,
				"description":          stringSchema,
				"summary":              stringSchema,
				"pubDate":              stringSchema,
				"duration":             stringSchema,
				"explicit":             map[string]any{"type": "boolean"},
				"episodeType":          stringSchema,
				"potentialCommercials": commercialReadsSchema(stringSchema),
				"commercials":          commercialReadsSchema(stringSchema),
				"segments": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"name", "transcript"},
						"properties": map[string]any{
							"name":       stringSchema,
							"transcript": stringSchema,
						},
					},
				},
				"transcript": stringSchema,
				"seasonContext": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"season", "leagueName", "phase", "week", "draftComplete", "completedMatchups", "totalMatchups", "generatedAt"},
					"properties": map[string]any{
						"season":            map[string]any{"type": "integer"},
						"leagueName":        stringSchema,
						"phase":             stringSchema,
						"week":              map[string]any{"type": "integer"},
						"draftComplete":     map[string]any{"type": "boolean"},
						"completedMatchups": map[string]any{"type": "integer"},
						"totalMatchups":     map[string]any{"type": "integer"},
						"generatedAt":       stringSchema,
					},
				},
				"sourceFiles": map[string]any{"type": "array", "items": stringSchema},
			},
		},
	}
}

func podcastOutlineSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type":   "json_schema",
		"name":   "podcast_outline",
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required": []string{
				"episodeId", "audioFile", "title", "description", "summary", "pubDate", "duration", "explicit", "episodeType", "potentialCommercials", "commercials", "segmentPlans", "sourceFiles",
			},
			"properties": map[string]any{
				"episodeId":            stringSchema,
				"audioFile":            stringSchema,
				"title":                stringSchema,
				"description":          stringSchema,
				"summary":              stringSchema,
				"pubDate":              stringSchema,
				"duration":             stringSchema,
				"explicit":             map[string]any{"type": "boolean"},
				"episodeType":          stringSchema,
				"potentialCommercials": commercialReadsSchema(stringSchema),
				"commercials":          commercialReadsSchema(stringSchema),
				"segmentPlans": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"name", "transcript"},
						"properties": map[string]any{
							"name":       stringSchema,
							"transcript": stringSchema,
						},
					},
				},
				"sourceFiles": map[string]any{"type": "array", "items": stringSchema},
			},
		},
	}
}

func podcastSegmentSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type":   "json_schema",
		"name":   "podcast_segment",
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "transcript"},
			"properties": map[string]any{
				"name":       stringSchema,
				"transcript": stringSchema,
			},
		},
	}
}

func commercialReadSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type":   "json_schema",
		"name":   "commercial_read",
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"companyName", "tagline", "read"},
			"properties": map[string]any{
				"companyName": stringSchema,
				"tagline":     stringSchema,
				"read":        stringSchema,
			},
		},
	}
}

func commercialReadsSchema(stringSchema map[string]any) map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"companyName", "tagline", "read"},
			"properties": map[string]any{
				"companyName": stringSchema,
				"tagline":     stringSchema,
				"read":        stringSchema,
			},
		},
	}
}

func logOpenAIOutput(output []responseOutputItem) {
	contents, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("OpenAI response output could not be logged: %v\n", err)
		return
	}
	fmt.Printf("OpenAI response output:\n%s\n", string(contents))
}

func outputText(output []responseOutputItem) string {
	var texts []string
	for _, item := range output {
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				texts = append(texts, content.Text)
			}
		}
	}
	return strings.Join(texts, "")
}

func fillScriptDefaults(script *PodcastScript, state SeasonState, episodeID string) {
	if script.EpisodeID == "" {
		script.EpisodeID = episodeID
	}
	if script.AudioFile == "" {
		script.AudioFile = script.EpisodeID + ".mp3"
	}
	if script.PubDate == "" {
		script.PubDate = time.Now().Format(time.RFC3339)
	}
	if script.EpisodeType == "" {
		script.EpisodeType = "full"
	}
	if len(script.PotentialCommercials) == 0 {
		script.PotentialCommercials = script.Commercials
	}
	if script.Transcript == "" {
		parts := make([]string, 0, len(script.Segments)+len(script.Commercials))
		for i, segment := range script.Segments {
			parts = append(parts, segment.Transcript)
			if i == 0 && len(script.Commercials) > 0 {
				parts = append(parts, script.Commercials[0].Read)
			}
			if i == len(script.Segments)-2 && len(script.Commercials) > 1 {
				parts = append(parts, script.Commercials[1].Read)
			}
		}
		script.Transcript = strings.Join(parts, "\n\n")
	}
	script.SeasonContext = state
}

func fillOutlineDefaults(outline *podcastOutline, state SeasonState, episodeID string) {
	if outline.EpisodeID == "" {
		outline.EpisodeID = episodeID
	}
	if outline.AudioFile == "" {
		outline.AudioFile = outline.EpisodeID + ".mp3"
	}
	if outline.PubDate == "" {
		outline.PubDate = time.Now().Format(time.RFC3339)
	}
	if outline.EpisodeType == "" {
		outline.EpisodeType = "full"
	}
	if len(outline.PotentialCommercials) == 0 {
		outline.PotentialCommercials = outline.Commercials
	}
	if len(outline.Commercials) < 2 && len(outline.PotentialCommercials) >= 2 {
		outline.Commercials = append([]CommercialRead{}, outline.PotentialCommercials[:2]...)
	}
	if len(outline.Commercials) == 0 {
		outline.Commercials = []CommercialRead{
			{CompanyName: "Waiver Wire Warehouse", Tagline: "Because panic adds are a lifestyle."},
			{CompanyName: "Flex Appeal Labs", Tagline: "Turn questionable into legendary."},
		}
	}
	if outline.Title == "" {
		outline.Title = fmt.Sprintf("%d Slop Take", state.Season)
	}
	if outline.Description == "" {
		outline.Description = "Fantasy football league analysis and hot takes."
	}
	if outline.Summary == "" {
		outline.Summary = outline.Description
	}
}

func podcastPhaseInstructions(state SeasonState) string {
	switch state.Phase {
	case PhaseDraft:
		return `Pre-draft episode assignment:
- Intro: frame the pre-draft stakes and explain that this episode is about last year's bottom, last year's top, and keeper decisions.
- Best Team: focus on last season's first-place team, why that roster/owner succeeded, what can carry forward, and which NFL news affects the repeat case.
- Worst Team: focus on last season's last-place team, why it failed, what must change, and which NFL news creates either danger or opportunity.
- Final Take: recommend which keepers people should pick for the season. Use keeper-info, prior standings/results, roster context, and current NFL news to support the keeper takes.`
	case PhasePostDraft:
		return `Post-draft episode assignment:
- Intro: frame the league immediately after the draft and preview draft winners, draft disasters, and season predictions.
- Best Team: determine the best draft in the league. Use draft results, roster construction, value, positional scarcity, keeper context, and current NFL news.
- Worst Team: determine the worst draft in the league. Call out reaches, roster holes, fragile NFL situations, injury/news risk, and missed opportunities.
- Final Take: deliver season predictions for the fantasy league, including projected contenders, collapse candidates, sleeper teams, and NFL news that could swing the standings.`
	case PhaseRegularSeason:
		return `Regular-season episode assignment:
- Intro: frame the current week around scoreboard pressure, standings movement, and urgent roster decisions.
- Best Team: analyze the team that scored the highest that week, how they succeeded, which lineup choices worked, and which NFL news confirms or complicates the success.
- Worst Team: analyze the team that scored the lowest that week, how they failed, which starts/benches hurt them, and which NFL news explains the damage.
- Final Take: recommend trades, free-agent moves, starts, sits, and bench decisions for league teams. Use top moves, roster context, matchup results, and current NFL news.`
	case PhasePostSeason:
		return `Post-season episode assignment:
- Intro: frame the playoff stakes, bracket pressure, elimination danger, and payout implications.
- Best Team: analyze the playoff-week high scorer, how they succeeded, which lineup choices worked, and which NFL news confirms or complicates the success.
- Worst Team: analyze the playoff-week low scorer, how they failed, which starts/benches hurt them, and which NFL news explains the damage.
- Final Take: recommend trades where still relevant, free-agent moves, starts, sits, and bench decisions for playoff teams and consolation spoilers. Use playoff bracket context, matchup results, and current NFL news.`
	case PhaseSeasonComplete:
		return `Season-complete episode assignment:
- Intro: frame the completed season, champion, final standings, payouts, and the biggest league-wide verdicts.
- Best Team: analyze the champion or most dominant finisher, how they won, and which NFL developments validated their roster.
- Worst Team: analyze the biggest collapse or weakest final finisher, why the season failed, and which NFL developments exposed the roster.
- Final Take: deliver offseason lessons, keeper implications, draft lessons, and early next-season predictions using final standings and current NFL context.`
	default:
		return `General episode assignment:
- Intro: frame the current league stakes.
- Best Team: identify and analyze the strongest team or performance for this phase.
- Worst Team: identify and analyze the weakest team or performance for this phase.
- Final Take: give actionable league-wide advice using available league data and current NFL news.`
	}
}

func segmentPlan(outline podcastOutline, name string) string {
	for _, plan := range outline.SegmentPlans {
		if strings.EqualFold(plan.Name, name) {
			return plan.Transcript
		}
	}
	return "Use the episode outline and league context to write this required segment."
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func mustJSON(value any) string {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(contents)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
