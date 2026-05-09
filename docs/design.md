# Design

## What it is

Drop in PDFs, notes, and links; ask grounded questions and get answers with page-level citations.

Single user. Single Go binary. Tiny corpus (think 5–20 materials).

## Architecture

```
   ┌─────────────────┐   REST    ┌──────────────────────────────┐
   │  React SPA      │ ────────► │  Go server (one binary)      │
   │  (embedded)     │           │                              │
   └─────────────────┘           │  ┌────────────────────────┐  │
                                 │  │ Agent loop (goroutine) │──┼──► Anthropic
                                 │  │  tools: ls, read,      │  │   (tool use)
                                 │  │         submit_response│  │
                                 │  └────────┬───────────────┘  │
                                 │           │ reads via VFS    │
                                 │  ┌────────▼───────────────┐  │
                                 │  │ Materials              │  │
                                 │  │  notes / webpages /pdf │  │
                                 │  └────────┬───────────────┘  │
                                 │   SQLite  │   Filesystem     │
                                 │  (meta +  │   blobs (bytes,  │
                                 │   events) │    OCR md)       │
                                 └───────────┼──────────────────┘
                                             │
                       typed note ──────────► │   (markdown as-is)
                       .md upload ──────────► │   (markdown as-is)
                       URL ─► pure.md ───────►│   (HTML → markdown)
                       PDF ─► Mistral OCR ───►│   (pages → markdown)
```

- The user adds a material — typed note, uploaded `.md` file, URL, or PDF. URLs go through pure.md; PDFs are split per page and OCR'd by Mistral; markdown is stored as-is. Bytes land on disk; metadata lands in SQLite.
- "Ask" spawns a background agent loop. The agent gets a workspace map up front and three tools: `ls`, `read`, `submit_response`. Each step is appended to a per-request event log so the UI can poll the trace and the user can close the tab.
- The agent reads materials through a small read-only VFS (`/notes/foo.md`, `/notes/paper/page_3.pdf`) — one stable namespace over the underlying material IDs and blob keys.

## Example: what the agent actually sees

Say the user has added four materials about transformers. The agent's first user message looks like this:

```
Workspace map (every material; titles in quotes):
/notes/attention-is-all-you-need/  "Attention Is All You Need" (pdf, 11 pages)
/notes/karpathy-nanogpt.md         "nanoGPT walkthrough"
/webpages/illustrated-transformer.md  "The Illustrated Transformer"  source=https://jalammar.github.io/illustrated-transformer/
/notes/my-reading-notes.md         "My reading notes"

Why does the original Transformer use sinusoidal positional encodings instead of learned ones, and what tradeoff does that make?
```

From here the agent typically calls `read /notes/attention-is-all-you-need/page_4.pdf` (PDFs come back as native PDF blocks so it can see equations), maybe `read /webpages/illustrated-transformer.md` for plain-language framing, then ends with `submit_response` — citing those exact paths in `used_items` and listing anything it couldn't find under `missing_context`.

## Why these choices

**No vector search.** With a handful of materials, the agent can just `read` what it needs. Embeddings would add an index, a chunking strategy, and a layer the user can't see through. Citations stay honest because the agent literally had to call `read` to see the content.

**VFS paths, not IDs.** The agent reasons better with `/notes/transformer-paper/page_3.pdf` than with `mat_1731...`. The VFS is also the one place that owns slug uniqueness and page-range checks, so handlers and prompts stay clean.

**Structured output is the contract.** The terminal `submit_response` tool *requires* `answer`, `used_items[*].why_relevant`, `next_actions`, and `missing_context[*]`. Citations and gap-acknowledgement are schema fields, not prompt wishes. The agent cannot end a turn without naming sources.

**Background goroutine + polled events, no SSE.** Agent turns can take 30+ seconds. Persisting every step to `request_events` and polling means: the user can close the tab, the trace is replayable from history, and a server restart marks in-flight requests as errored instead of leaving silent ghosts. The agent goroutine uses `context.Background()` on purpose — a client disconnect must not cancel an LLM call already in flight.
