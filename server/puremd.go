package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// fetchPureMD fetches a URL through pure.md and returns the markdown body.
func fetchPureMD(ctx context.Context, target string) (string, error) {
	apiKey := getenv("PURE_MD_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("PURE_MD_API_KEY not set")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://pure.md/"+target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-puremd-api-token", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("pure.md http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("pure.md status %d: %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}
