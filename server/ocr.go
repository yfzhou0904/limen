package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

type ocrPage struct {
	Index    int
	Markdown string
}

func mistralOCR(ctx context.Context, pdf []byte) ([]ocrPage, error) {
	apiKey := getenv("MISTRAL_API_KEY", "")
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	body, _ := json.Marshal(map[string]any{
		"model": "mistral-ocr-latest",
		"document": map[string]string{
			"type":         "document_url",
			"document_url": "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdf),
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mistral.ai/v1/ocr", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mistral http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("mistral status %d: %s", resp.StatusCode, string(respBody))
	}
	var decoded struct {
		Pages []struct {
			Index    int    `json:"index"`
			Markdown string `json:"markdown"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("mistral decode: %w", err)
	}
	pages := make([]ocrPage, 0, len(decoded.Pages))
	for _, p := range decoded.Pages {
		pages = append(pages, ocrPage{Index: p.Index, Markdown: p.Markdown})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Index < pages[j].Index })
	return pages, nil
}
