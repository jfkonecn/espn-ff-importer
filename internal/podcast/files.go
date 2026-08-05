package podcast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadJSON(path string, target any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(contents, target)
}

func WriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(path, contents, 0644)
}

func SafeReadAIFile(aiRoot, requestedPath string) (string, error) {
	if requestedPath == "" {
		return "", fmt.Errorf("path is required")
	}

	cleanRoot, err := filepath.Abs(aiRoot)
	if err != nil {
		return "", err
	}

	path := requestedPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(aiRoot, path)
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside %s", requestedPath, aiRoot)
	}

	contents, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func ListAIFiles(aiRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(aiRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(aiRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func DefaultEpisodeID(state SeasonState) string {
	switch state.Phase {
	case PhaseDraft:
		return fmt.Sprintf("%d_Draft", state.Season)
	case PhasePostDraft:
		return fmt.Sprintf("%d_PostDraft", state.Season)
	case PhasePostSeason:
		return fmt.Sprintf("%d_PostSeason_Week%d", state.Season, state.Week)
	case PhaseSeasonComplete:
		return fmt.Sprintf("%d_SeasonComplete", state.Season)
	default:
		if state.Week > 0 {
			return fmt.Sprintf("%d_Week%d", state.Season, state.Week)
		}
		return fmt.Sprintf("%d_Season", state.Season)
	}
}
