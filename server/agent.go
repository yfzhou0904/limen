package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultModel = "claude-sonnet-4-6"
	maxTurns     = 16
)

const systemPrompt = `You assist the user with their personal workspace of saved materials (notes, webpages, OCR'd PDFs).

Tools:
- 'ls' lists a directory in the read-only VFS.
- 'read' reads a file. .md returns text; .pdf returns the original page as a PDF document (use when figures or equations matter).
- 'submit_response' ends the turn with the structured answer. Call it exactly once.

VFS:
  /notes/<slug>.md              (typed or uploaded markdown note)
  /notes/<slug>/page_{N}.md     (PDF page, OCR'd)
  /notes/<slug>/page_{N}.pdf    (PDF page, original bytes)
  /webpages/<slug>.md           (source URL noted in the workspace map)

The first user message includes a complete map of every material in the workspace, so you don't need to 'ls' to discover what exists.

In submit_response: ground claims in materials you read (otherwise put them in missing_context). Each used_item has a 'material_id' ("<dir>/<slug>" exactly as in the workspace map, e.g. "notes/attention-is-all-you-need") and an optional 'page' (1-indexed, for citing a specific PDF page; null for whole-material or non-PDF references).`

type AgentResponse struct {
	Answer         string        `json:"answer"`
	UsedItems      []UsedItem    `json:"used_items"`
	NextActions    []string      `json:"next_actions"`
	MissingContext []MissingItem `json:"missing_context"`
}

type UsedItem struct {
	// On input from the agent: "<dir>/<slug>" (e.g. "notes/attention-is-all-you-need").
	// After resolveUsedTitles: rewritten to the material's DB id.
	MaterialID  string `json:"material_id"`
	Title       string `json:"title"`
	Page        int    `json:"page,omitempty"`
	WhyRelevant string `json:"why_relevant"`
}

type MissingItem struct {
	What       string `json:"what"`
	Suggestion string `json:"suggestion"`
}

// OnEvent is called once per agent step. Caller decides how to persist.
type OnEvent func(kind string, payload any)

func runAgent(ctx context.Context, question string, emit OnEvent) (*AgentResponse, error) {
	if emit == nil {
		emit = func(string, any) {}
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	model := getenv("ANTHROPIC_MODEL", defaultModel)

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	overview, _ := vfsOverview(ctx)
	firstUserText := overview + "\n\n" + question

	tools := []anthropic.ToolUnionParam{
		{OfTool: &anthropic.ToolParam{
			Name:        "ls",
			Description: anthropic.String("List entries at a directory path in the workspace VFS."),
			Strict:      anthropic.Bool(true),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"path": map[string]any{"type": "string", "description": "Absolute path, e.g. /notes or /notes/textbook"},
				},
				Required:    []string{"path"},
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "read",
			Description: anthropic.String("Read a file in the workspace VFS. Returns text for .md paths or a PDF document for .pdf paths."),
			Strict:      anthropic.Bool(true),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"path": map[string]any{"type": "string", "description": "Absolute path to a .md or .pdf file"},
				},
				Required:    []string{"path"},
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}},
		{OfTool: &anthropic.ToolParam{
			Name:         "submit_response",
			Description:  anthropic.String("Terminal tool. Submit the final structured response. Call exactly once."),
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
			Strict:       anthropic.Bool(true),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"answer": map[string]any{"type": "string"},
					"used_items": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"material_id":  map[string]any{"type": "string", "description": "Material reference \"<dir>/<slug>\" exactly as it appears in the workspace map (e.g. \"notes/attention-is-all-you-need\")."},
								"page":         map[string]any{"type": []string{"integer", "null"}, "description": "1-indexed page number for PDF citations; null for non-PDF or whole-material references."},
								"why_relevant": map[string]any{"type": "string"},
							},
							"required":             []string{"material_id", "page", "why_relevant"},
							"additionalProperties": false,
						},
					},
					"next_actions": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"missing_context": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"what":       map[string]any{"type": "string"},
								"suggestion": map[string]any{"type": "string"},
							},
							"required":             []string{"what", "suggestion"},
							"additionalProperties": false,
						},
					},
				},
				Required:    []string{"answer", "used_items", "next_actions", "missing_context"},
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}},
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(firstUserText)),
	}

	var lastAssistantText string

	for turn := 0; turn < maxTurns; turn++ {
		markLastMessageForCache(messages)
		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: 4096,
			System:    []anthropic.TextBlockParam{{Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
			Tools:     tools,
			Messages:  messages,
		})
		if err != nil {
			return nil, fmt.Errorf("anthropic call: %w", err)
		}

		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(resp.Content))
		for _, b := range resp.Content {
			assistantBlocks = append(assistantBlocks, b.ToParam())
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		var turnText strings.Builder
		for _, b := range resp.Content {
			if b.Type == "text" {
				if turnText.Len() > 0 {
					turnText.WriteString("\n")
				}
				turnText.WriteString(b.Text)
			}
		}
		if t := strings.TrimSpace(turnText.String()); t != "" {
			lastAssistantText = t
			emit("assistant_text", map[string]any{"text": t})
		}

		var toolResults []anthropic.ContentBlockParamUnion
		var submitted *AgentResponse
		for _, b := range resp.Content {
			if b.Type != "tool_use" {
				continue
			}
			tu := b.AsToolUse()
			emit("tool_use", map[string]any{"id": tu.ID, "name": tu.Name, "input": json.RawMessage(tu.Input)})

			switch tu.Name {
			case "submit_response":
				var r AgentResponse
				if err := json.Unmarshal(tu.Input, &r); err != nil {
					emit("tool_result", map[string]any{"id": tu.ID, "error": err.Error()})
					toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, "invalid payload: "+err.Error(), true))
					continue
				}
				if err := resolveUsedTitles(ctx, &r); err != nil {
					emit("tool_result", map[string]any{"id": tu.ID, "error": err.Error()})
					toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, err.Error(), true))
					continue
				}
				submitted = &r
				emit("tool_result", map[string]any{"id": tu.ID, "ok": true})
				toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, "ok", false))
			case "ls":
				var args struct{ Path string }
				_ = json.Unmarshal(tu.Input, &args)
				entries, err := vfsLs(ctx, args.Path)
				if err != nil {
					emit("tool_result", map[string]any{"id": tu.ID, "error": err.Error()})
					toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, err.Error(), true))
					continue
				}
				out, _ := json.Marshal(entries)
				emit("tool_result", map[string]any{"id": tu.ID, "entries": entries})
				toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, string(out), false))
			case "read":
				var args struct{ Path string }
				_ = json.Unmarshal(tu.Input, &args)
				fc, err := vfsRead(ctx, args.Path)
				if err != nil {
					emit("tool_result", map[string]any{"id": tu.ID, "error": err.Error()})
					toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, err.Error(), true))
					continue
				}
				if fc.Kind == filePDF {
					emit("tool_result", map[string]any{"id": tu.ID, "path": args.Path, "bytes": len(fc.Bytes), "kind": "pdf"})
					doc := anthropic.DocumentBlockParam{
						Source: anthropic.DocumentBlockParamSourceUnion{
							OfBase64: &anthropic.Base64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(fc.Bytes)},
						},
					}
					toolResults = append(toolResults, anthropic.ContentBlockParamUnion{
						OfToolResult: &anthropic.ToolResultBlockParam{
							ToolUseID: tu.ID,
							Content:   []anthropic.ToolResultBlockParamContentUnion{{OfDocument: &doc}},
						},
					})
				} else {
					emit("tool_result", map[string]any{"id": tu.ID, "path": args.Path, "bytes": len(fc.Text), "kind": "md"})
					toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, fc.Text, false))
				}
			default:
				emit("tool_result", map[string]any{"id": tu.ID, "error": "unknown tool: " + tu.Name})
				toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, "unknown tool: "+tu.Name, true))
			}
		}

		if submitted != nil {
			emit("final", map[string]any{"synthesized": false})
			return submitted, nil
		}

		if len(toolResults) == 0 {
			summary := lastAssistantText
			if summary == "" {
				summary = "(agent ended without producing a response)"
			}
			emit("final", map[string]any{"synthesized": true, "reason": "end_turn_without_submit"})
			return &AgentResponse{Answer: summary, UsedItems: []UsedItem{}, NextActions: []string{}, MissingContext: []MissingItem{}}, nil
		}

		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	summary := lastAssistantText
	if summary == "" {
		summary = fmt.Sprintf("(agent exceeded %d turns without submitting a response)", maxTurns)
	}
	emit("final", map[string]any{"synthesized": true, "reason": "max_turns"})
	return &AgentResponse{Answer: summary, UsedItems: []UsedItem{}, NextActions: []string{}, MissingContext: []MissingItem{}}, nil
}

// resolveUsedTitles fills in Title for each used_item by looking up the
// (kind, slug) pair against the current VFS.
func resolveUsedTitles(ctx context.Context, r *AgentResponse) error {
	nodes, err := loadVFS(ctx)
	if err != nil {
		return err
	}
	mats, err := listMaterials()
	if err != nil {
		return err
	}
	idToTitle := map[string]string{}
	for _, m := range mats {
		idToTitle[m.ID] = m.Title
	}
	var bad []string
	for i := range r.UsedItems {
		ref := strings.Trim(r.UsedItems[i].MaterialID, "/")
		dir, slug, ok := strings.Cut(ref, "/")
		if !ok {
			bad = append(bad, r.UsedItems[i].MaterialID)
			continue
		}
		kinds := []string{}
		switch dir {
		case "notes":
			kinds = []string{"note", "pdf"}
		case "webpages":
			kinds = []string{"webpage"}
		}
		var match *vfsNode
	outer:
		for _, k := range kinds {
			for j := range nodes {
				if nodes[j].Kind == k && nodes[j].Slug == slug {
					match = &nodes[j]
					break outer
				}
			}
		}
		if match == nil {
			bad = append(bad, r.UsedItems[i].MaterialID)
			continue
		}
		r.UsedItems[i].MaterialID = match.ID
		r.UsedItems[i].Title = idToTitle[match.ID]
		if match.Ext != "pdf" {
			r.UsedItems[i].Page = 0
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("unknown material_id(s): %s — must be \"<dir>/<slug>\" exactly as listed in the workspace map", strings.Join(bad, ", "))
	}
	return nil
}

func markLastMessageForCache(msgs []anthropic.MessageParam) {
	for i := range msgs {
		for j := range msgs[i].Content {
			if cc := msgs[i].Content[j].GetCacheControl(); cc != nil {
				*cc = anthropic.CacheControlEphemeralParam{}
			}
		}
	}
	if len(msgs) == 0 {
		return
	}
	last := &msgs[len(msgs)-1]
	if len(last.Content) == 0 {
		return
	}
	if cc := last.Content[len(last.Content)-1].GetCacheControl(); cc != nil {
		*cc = anthropic.NewCacheControlEphemeralParam()
	}
}
