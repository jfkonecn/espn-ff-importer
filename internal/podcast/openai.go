package podcast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	openAIBaseURL             = "https://api.openai.com/v1"
	openAIRateLimitMaxRetries = 3
	openAIRateLimitBaseDelay  = 30 * time.Second
	openAIRateLimitMaxDelay   = 2 * time.Minute
)

var openAIRetryAfterMessageRE = regexp.MustCompile(`(?i)try again in ([0-9]+(?:\.[0-9]+)?)s`)

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
	segmentNames := podcastSegmentNames(state)
	segmentTangents := selectSegmentTangents(segmentNames)
	if model == "" {
		model = "gpt-4.1"
	}
	fmt.Printf("Requesting podcast outline from OpenAI using episode ID %s\n", episodeID)

	phaseInstructions := podcastPhaseInstructions(state)

	outlineSystemPrompt := fmt.Sprintf(`You are planning a fantasy football podcast called Slop Take. Write in the spirit of loud, confrontational sports-talk radio: clipped cadence, sharp resets, scoreboard justice, call-out energy, and memorable recurring phrases. Do not claim to be Jim Rome or imitate any living broadcaster verbatim.

Plan exactly %d main segments in this order: %s. The Intro plan must identify the fantasy league storylines, the owners or teams under pressure, the stakes for this episode, and a clear roadmap for the rest of the show. Plan two fantasy-football-themed fake commercial reads. Commercial placement: %s. Invent a list of potential fake fantasy-football sponsors, then select two for the reads.

Every segment plan must use league data and current NFL context. Use web search to look up the latest NFL news, injuries, depth chart changes, camp reports, trades, suspensions, and role changes before planning the episode. Do not invent specific breaking news; rely on searched current context when making NFL-news claims.

Return only valid JSON matching the requested schema. This is an outline and metadata pass, not the full transcript. Keep duration as an empty string; the publisher will not know the real duration until audio exists.`, len(segmentNames), strings.Join(segmentNames, ", "), commercialPlacementDescription(state))

	outlineUserPrompt := fmt.Sprintf(`Season state:
%s

Available files under ai/:
%s

Phase-specific assignment:
%s

Assigned sports-talk tangent topics by segment:
%s

Use the read_ai_file tool to inspect league data in ai/. Use web search for current NFL news and injury context. Produce podcast metadata, two selected commercials, and concise plans for the required segments in the specified order. Each segment plan should include a quick, natural way to connect that segment's assigned tangent topic to the league take without letting the tangent take over. Use episodeId %q and audioFile %q.`, mustJSON(state), strings.Join(aiFiles, "\n"), phaseInstructions, mustJSON(segmentTangents), episodeID, episodeID+".mp3")

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

	commercialIndex := 0
	for _, segmentName := range segmentNames {
		segment, files, err := generateSegment(apiKey, model, aiRoot, state, outline, segmentName, segmentPlan(outline, segmentName), segmentTangents[segmentName])
		if err != nil {
			return PodcastScript{}, err
		}
		segments = append(segments, segment)
		transcriptParts = append(transcriptParts, segment.Transcript)
		allSourceFiles = append(allSourceFiles, files...)

		if hasCommercialAfterSegment(state, segmentName, segmentNames) && commercialIndex < len(outline.Commercials) {
			commercial, files, err := generateCommercial(apiKey, model, aiRoot, state, outline, outline.Commercials[commercialIndex], commercialIndex+1)
			if err != nil {
				return PodcastScript{}, err
			}
			outline.Commercials[commercialIndex] = commercial
			transcriptParts = append(transcriptParts, commercial.Read)
			allSourceFiles = append(allSourceFiles, files...)
			commercialIndex++
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

	resp, err := postOpenAI(apiKey, "/audio/speech", body)
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

func generateSegment(apiKey, model, aiRoot string, state SeasonState, outline podcastOutline, segmentName, plan, tangentTopic string) (PodcastSegment, []string, error) {
	fmt.Printf("Generating %s segment with about 3 minutes of copy\n", segmentName)
	systemPrompt := `You are writing one segment for Slop Take, a fantasy football podcast with loud, confrontational sports-talk energy: clipped cadence, sharp resets, scoreboard justice, call-out energy, and memorable phrases. Do not claim to be Jim Rome or imitate any living broadcaster verbatim.

Write only this one segment. Target about 3 minutes when read aloud, roughly 390 to 480 words. Make it TTS-ready: no markdown, no bullets, no stage directions, no URLs, and no citations. Use web search for current NFL news and injury context before writing this segment. Do not invent specific breaking news; rely on searched current context when making NFL-news claims. Every NFL news item you mention must be tied directly back to what is happening in this fantasy league: team outlooks, roster strengths or weaknesses, draft posture, standings pressure, owner decisions, keeper value, matchup consequences, trades, waivers, starts, or sits. Include the assigned sports-talk tangent topic as a quick, natural aside or analogy that supports the segment's fantasy take. Do not let the tangent become the segment. Return only valid JSON matching the schema.`
	if segmentName == "Intro" {
		systemPrompt += fmt.Sprintf(" For the Intro, focus on the fantasy league's current storylines: the season phase, the teams under pressure, the owners who need to hear it, and the stakes of this episode. Set the table for what the podcast will cover instead of giving a generic welcome. Preview these segment angles without resolving them yet: %s.", strings.Join(nonIntroSegmentNames(state), ", "))
	}

	userPrompt := fmt.Sprintf(`Season state:
%s

Episode outline:
%s

Segment name: %s
Segment plan: %s

Phase-specific assignment:
%s

Assigned sports-talk tangent topic for this segment: %s

Use read_ai_file for league context if needed. Write the segment as a complete standalone block with a strong opening, escalating middle, and clean landing.`, mustJSON(state), mustJSON(outline), segmentName, plan, podcastPhaseInstructions(state), tangentTopic)

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

	resp, err := postOpenAI(apiKey, "/responses", body)
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

func postOpenAI(apiKey, path string, body []byte) (*http.Response, error) {
	for attempt := range openAIRateLimitMaxRetries + 1 {
		req, err := http.NewRequest(http.MethodPost, openAIBaseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt == openAIRateLimitMaxRetries {
			return resp, nil
		}

		contents, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		delay := openAIRateLimitDelay(resp, string(contents), attempt)
		fmt.Printf("OpenAI rate limit hit for %s; sleeping %s before retry %d/%d\n", path, delay, attempt+1, openAIRateLimitMaxRetries)
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("OpenAI request retry loop exited unexpectedly")
}

func openAIRateLimitDelay(resp *http.Response, body string, attempt int) time.Duration {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if retryAt, err := http.ParseTime(retryAfter); err == nil {
			if delay := time.Until(retryAt); delay > 0 {
				return delay
			}
		}
	}
	if matches := openAIRetryAfterMessageRE.FindStringSubmatch(body); len(matches) == 2 {
		if seconds, err := strconv.ParseFloat(matches[1], 64); err == nil && seconds > 0 {
			return time.Duration(seconds * float64(time.Second))
		}
	}

	delay := openAIRateLimitBaseDelay << attempt
	if delay > openAIRateLimitMaxDelay {
		return openAIRateLimitMaxDelay
	}
	return delay
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

var sportsTangentTopics = []string{
	"Tom Brady",
	"Dallas Cowboys",
	"Bill Belichick",
	"New York Knicks",
	"LeBron James",
	"Patrick Mahomes",
	"Aaron Rodgers",
	"Michael Jordan",
	"Tiger Woods",
	"Shohei Ohtani",
	"Caitlin Clark",
	"Deion Sanders",
	"Coach Prime at Colorado",
	"Stephen A. Smith",
	"Skip Bayless",
	"Jerry Jones",
	"the Lakers",
	"the Yankees",
	"the Red Sox",
	"the Celtics",
	"the Chiefs dynasty",
	"the Patriots dynasty",
	"the SEC",
	"Alabama football",
	"Duke basketball",
	"the transfer portal",
	"NIL money",
	"bad officiating",
	"load management",
	"analytics ruining sports",
	"whether a player is clutch",
	"the GOAT debate",
	"Hall of Fame debates",
	"legacy talk",
	"Super Bowl rings",
	"playoff choking",
	"locker-room culture",
	"coaching hot seats",
	"New York media pressure",
	"Philly fans",
	"fantasy football bad beats",
	"Vegas lines",
	"sports betting parlays",
	"stadium food",
	"terrible uniforms",
	"national anthem controversies",
	"referee conspiracies",
	"media hot-take shows",
	"player podcasts",
	"old-school vs modern athletes",
}

func selectSegmentTangents(segmentNames []string) map[string]string {
	selected := make(map[string]string, len(segmentNames))
	if len(sportsTangentTopics) == 0 {
		return selected
	}

	topics := append([]string{}, sportsTangentTopics...)
	for i := range topics {
		j := i + rand.IntN(len(topics)-i)
		topics[i], topics[j] = topics[j], topics[i]
	}

	for i, segmentName := range segmentNames {
		selected[segmentName] = topics[i%len(topics)]
	}
	return selected
}

func podcastSegmentNames(state SeasonState) []string {
	if state.Phase == PhaseSeasonComplete {
		return []string{"Intro", "Season in Review", "Last Place Game", "Third Place Game", "Championship Game"}
	}
	return []string{"Intro", "Best Team", "Worst Team", "Power Ranking", "Matchup Preview", "Final Take"}
}

func nonIntroSegmentNames(state SeasonState) []string {
	var names []string
	for _, name := range podcastSegmentNames(state) {
		if !strings.EqualFold(name, "Intro") {
			names = append(names, name)
		}
	}
	return names
}

func commercialPlacementDescription(state SeasonState) string {
	if state.Phase == PhaseSeasonComplete {
		return "first ad break after Intro; second ad break after Third Place Game"
	}
	return "first ad break after Intro; second ad break after Matchup Preview, immediately before Final Take"
}

func hasCommercialAfterSegment(state SeasonState, segmentName string, segmentNames []string) bool {
	if state.Phase == PhaseSeasonComplete {
		return strings.EqualFold(segmentName, "Intro") || strings.EqualFold(segmentName, "Third Place Game")
	}
	if strings.EqualFold(segmentName, "Intro") {
		return true
	}
	if containsSegmentName(segmentNames, "Matchup Preview") {
		return strings.EqualFold(segmentName, "Matchup Preview")
	}
	if containsSegmentName(segmentNames, "Power Ranking") {
		return strings.EqualFold(segmentName, "Power Ranking")
	}
	return strings.EqualFold(segmentName, "Worst Team")
}

func containsSegmentName(segmentNames []string, name string) bool {
	for _, segmentName := range segmentNames {
		if strings.EqualFold(segmentName, name) {
			return true
		}
	}
	return false
}

func podcastPhaseInstructions(state SeasonState) string {
	switch state.Phase {
	case PhaseDraft:
		return `Pre-draft episode assignment:
	- Intro: frame the pre-draft stakes and explain that this episode is about last year's bottom, last year's top, keeper decisions, power ranking, and matchup preview.
	- Best Team: focus on last season's first-place team, why that roster/owner succeeded, what can carry forward, and which NFL news affects the repeat case.
	- Worst Team: focus on last season's last-place team, why it failed, what must change, and which NFL news creates either danger or opportunity.
	- Power Ranking: rank every team by current team strength, not by simply repeating the standings. Ground the ranking in reality using prior results, scoring profile, keeper value, roster context, draft posture, manager decisions, injury/news risk, and current NFL context.
	- Matchup Preview: preview the first upcoming matchups if available; if current-matchups is unavailable, preview the most important likely early-season clashes using schedule, rosters, and current NFL context.
	- Final Take: recommend which keepers people should pick for the season. Use keeper-info, prior standings/results, roster context, and current NFL news to support the keeper takes.`
	case PhasePostDraft:
		return `Post-draft episode assignment:
	- Intro: frame the league immediately after the draft and preview draft winners, draft disasters, power ranking, season predictions, and matchup preview.
	- Best Team: determine the best draft in the league. Use draft results, roster construction, value, positional scarcity, keeper context, and current NFL news.
	- Worst Team: determine the worst draft in the league. Call out reaches, roster holes, fragile NFL situations, injury/news risk, and missed opportunities.
	- Power Ranking: rank every team after the draft by current team strength, not by simply repeating the draft order, standings, or projections. Ground the ranking in reality using roster construction, keeper value, projected scoring, positional depth, upside, risk, manager decisions, and current NFL context.
	- Matchup Preview: preview upcoming matchups from current-matchups when available, including projected totals, key starters, lineup risks, and NFL news that could swing each matchup.
	- Final Take: deliver season predictions for the fantasy league, including projected contenders, collapse candidates, sleeper teams, and NFL news that could swing the standings.`
	case PhaseRegularSeason:
		return `Regular-season episode assignment:
	- Intro: frame the current week around scoreboard pressure, standings movement, urgent roster decisions, power ranking, and matchup preview.
	- Best Team: analyze the team that scored the highest that week, how they succeeded, which lineup choices worked, and which NFL news confirms or complicates the success.
	- Worst Team: analyze the team that scored the lowest that week, how they failed, which starts/benches hurt them, and which NFL news explains the damage.
	- Power Ranking: rank every team by current team strength, not by simply repeating the standings. Standings matter, but the analysis must go deeper: weekly scoring, roster quality, lineup decisions, momentum, injuries, schedule context, sustainability, and current NFL context.
	- Matchup Preview: preview upcoming matchups from current-matchups, including projected totals, key starters, lineup risks, swing players, and NFL news that could decide each matchup.
	- Final Take: recommend trades, free-agent moves, starts, sits, and bench decisions for league teams. Use top moves, roster context, matchup results, and current NFL news.`
	case PhasePostSeason:
		return `Post-season episode assignment:
	- Intro: frame the playoff stakes, bracket pressure, elimination danger, payout implications, power ranking, and matchup preview.
	- Best Team: analyze the playoff-week high scorer, how they succeeded, which lineup choices worked, and which NFL news confirms or complicates the success.
	- Worst Team: analyze the playoff-week low scorer, how they failed, which starts/benches hurt them, and which NFL news explains the damage.
	- Power Ranking: rank the remaining contenders and consolation spoilers by current team strength, not by simply repeating playoff seeds or standings. Ground the ranking in reality using bracket position, scoring form, roster health, matchup difficulty, lineup decisions, sustainability, and current NFL context.
	- Matchup Preview: preview upcoming playoff and consolation matchups from current-matchups, including projected totals, key starters, lineup risks, swing players, and payout implications.
	- Final Take: recommend trades where still relevant, free-agent moves, starts, sits, and bench decisions for playoff teams and consolation spoilers. Use playoff bracket context, matchup results, and current NFL news.`
	case PhaseSeasonComplete:
		return `Post-championship episode assignment:
	- Intro: frame the completed season, champion, final standings, payouts, last-place punishment, and the show roadmap.
	- Season in Review: briefly go team by team through season highlights and lowlights. Keep each team concise; this is a league-wide review, not six separate deep dives.
	- Last Place Game: break down the last-place game, explain how it happened, identify the worst decisions and roster failures, and roast the last-place team.
	- Third Place Game: break down the third-place game and celebrate the third-place team for finishing strong.
	- Championship Game: break down the first-place game and celebrate the league winner, including the roster choices and NFL developments that validated the title run.`
	default:
		return `General episode assignment:
	- Intro: frame the current league stakes.
	- Best Team: identify and analyze the strongest team or performance for this phase.
	- Worst Team: identify and analyze the weakest team or performance for this phase.
	- Power Ranking: rank every team by current team strength, not by simply repeating the standings. Ground the ranking in reality using available league data, scoring profile, roster quality, manager decisions, trend lines, risk, and current NFL news.
	- Matchup Preview: preview upcoming matchups using available league data and current NFL news.
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
