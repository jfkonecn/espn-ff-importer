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
	ID     string               `json:"id"`
	Output []responseOutputItem `json:"output"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func GenerateTranscript(apiKey, model, aiRoot string, state SeasonState) (PodcastScript, error) {
	aiFiles, err := ListAIFiles(aiRoot)
	if err != nil {
		return PodcastScript{}, err
	}

	episodeID := DefaultEpisodeID(state)
	if model == "" {
		model = "gpt-4.1"
	}

	systemPrompt := `You are creating a fantasy football podcast called Slop Take. Write in the spirit of loud, confrontational sports-talk radio: clipped cadence, sharp resets, scoreboard justice, call-out energy, and memorable recurring phrases. Do not claim to be Jim Rome or imitate any living broadcaster verbatim.

The episode must have exactly four main segments in this order: Intro, Best Team, Worst Team, Final Take. Put one fantasy-football-themed fake commercial read after the Intro and another fake commercial read immediately before Final Take. Invent a list of potential fake fantasy-football sponsors, then select two for the reads.

Final Take must analyze the whole league using league data and current NFL context. Use web search to look up the latest NFL news before writing the Final Take. Do not invent specific breaking news; rely on searched current context when making NFL-news claims.

Return only valid JSON matching the requested schema. The transcript should be ready for text-to-speech: no markdown tables, no stage directions, and no citations.`

	userPrompt := fmt.Sprintf(`Season state:
%s

Available files under ai/:
%s

Use the read_ai_file tool to inspect league data in ai/ before writing. Use web search for current NFL news and injury context. Produce all podcast metadata. Use episodeId %q and audioFile %q. Keep the full transcript under roughly 2,200 words.`, mustJSON(state), strings.Join(aiFiles, "\n"), episodeID, episodeID+".mp3")

	input := []responseInputItem{
		{Role: "system", Content: []responseContent{{Type: "input_text", Text: systemPrompt}}},
		{Role: "user", Content: []responseContent{{Type: "input_text", Text: userPrompt}}},
	}

	previousResponseID := ""
	for range 8 {
		result, err := createResponse(apiKey, model, input, previousResponseID, true)
		if err != nil {
			return PodcastScript{}, err
		}
		previousResponseID = result.ID

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

				contents, err := SafeReadAIFile(aiRoot, args.Path)
				if err != nil {
					contents = "Error: " + err.Error()
				}
				toolOutputs = append(toolOutputs, responseInputItem{Type: "function_call_output", CallID: item.CallID, Output: contents})
			}
		}

		if len(toolOutputs) == 0 {
			text := outputText(result.Output)
			if text == "" {
				return PodcastScript{}, fmt.Errorf("OpenAI response did not include transcript JSON")
			}
			var script PodcastScript
			if err := json.Unmarshal([]byte(text), &script); err != nil {
				return PodcastScript{}, fmt.Errorf("failed to parse transcript JSON: %w", err)
			}
			fillScriptDefaults(&script, state, episodeID)
			return script, nil
		}

		input = toolOutputs
	}

	return PodcastScript{}, fmt.Errorf("OpenAI tool loop exceeded maximum iterations")
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
	_, err = io.Copy(file, resp.Body)
	return err
}

func createResponse(apiKey, model string, input []responseInputItem, previousResponseID string, includeTools bool) (responseAPIResult, error) {
	payload := map[string]any{
		"model": model,
		"input": input,
		"text": map[string]any{
			"format": podcastScriptSchema(),
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

func outputText(output []responseOutputItem) string {
	for _, item := range output {
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				return content.Text
			}
		}
	}
	return ""
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
