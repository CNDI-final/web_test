package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"web_test/internal/logger"
	"web_test/pkg/models"
)

func FetchGitHubInfo(owner, repo string) (models.WorkerResponse, error) {
	logger.GitHubLog.Infof("Fetching %s/%s", owner, repo)
	client := &http.Client{Timeout: 5 * time.Second}

	var prs []models.PullRequest
	prURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&per_page=100", owner, repo)
	req, _ := http.NewRequest("GET", prURL, nil)
	req.Header.Set("User-Agent", "Go-Worker")
	res, err := client.Do(req)
	if err != nil {
		return models.WorkerResponse{}, err
	}
	defer res.Body.Close()
	json.NewDecoder(res.Body).Decode(&prs)

	relURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=1", owner, repo)
	reqRel, _ := http.NewRequest("GET", relURL, nil)
	reqRel.Header.Set("User-Agent", "Go-Worker")
	resRel, err := client.Do(reqRel)
	var verStr string
	if err == nil {
		var rels []models.Release
		defer resRel.Body.Close()
		json.NewDecoder(resRel.Body).Decode(&rels)
		if len(rels) > 0 {
			name := rels[0].Name
			if name == "" {
				name = rels[0].TagName
			}
			verStr = " | Ver: " + name
		}
	}

	resp := models.WorkerResponse{
		Summary: fmt.Sprintf("[%s/%s] PRs: %d%s", owner, repo, len(prs), verStr),
		PRs:     prs,
	}
	return resp, nil
}
