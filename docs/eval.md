# Evaluation

How we judge whether the system is actually useful, given that the corpus is small and the agent is the retrieval mechanism.

## Three dimensions, scored per case

For each test case, score 0 / 0.5 / 1 on each dimension by reading the trace (`request_events`) and the final `submit_response`.

| Dimension | 1 (pass) | 0.5 | 0 |
|---|---|---|---|
| **Answer quality** | Correct, grounded in cited materials, no fabricated facts. | Mostly correct but adds claims not supported by what was read. | Wrong, or the agent invented sources. |
| **Context selection** | `used_items` contains every gold material; nothing irrelevant. | Misses one expected material, or pads with unrelated ones. | Misses the obviously-relevant material; cites paths not in workspace. |
| **Failure handling** | When info is genuinely missing, `missing_context` says so specifically and `next_actions` are useful. | Vague "more info needed" without naming what. | Pretends the workspace was sufficient when it wasn't, or returns empty boilerplate. |

## Test cases (against the curated LLM workspace)

Each case is a fixed prompt + a gold expectation. Pass criteria = ≥0.5 on every dimension and 1 on at least two.

1. **Well-grounded factual.** *"Why is standard attention O(n²) in sequence length?"*
   Gold `used_items`: Attention Is All You Need (specific page with the QK^T formula). Gold `missing_context`: empty.

2. **Synthesis across materials** (mirrors the homework use case).
   *"Summarize my current understanding of long-context inference cost. What's missing? What should I read or do next?"*
   Gold `used_items`: ≥3 of {FlashAttention-2, vLLM, prompt caching, Character.AI blog}. Gold `next_actions`: 2–3 concrete reads or experiments. Gold `missing_context`: at least one real gap (e.g. "no benchmarks on your specific hardware").

3. **Out-of-scope question.** *"What's the latest on Mamba and state-space models?"*
   Workspace has nothing on SSMs. Gold: short answer or refusal; `missing_context` names the missing topic; `next_actions` suggests adding an SSM paper. Failure mode to catch: agent fabricates an answer from related material.

4. **Figure / equation question.** *"Walk me through the FlashAttention tiling diagram."*
   Gold trace: agent calls `read` on a `.pdf` page (not just the OCR `.md`), since the diagram is visual. Gold `used_items`: cites `notes/flashattention-2/page_N` with a real page number.

5. **Partial-coverage comparison.** *"How does vLLM's PagedAttention compare to FlashAttention?"*
   Gold `used_items`: both papers. Gold answer: real comparison, not a side-by-side restatement. Gold `missing_context` if no benchmark numbers are present in the corpus.

## Failure modes to watch for in the trace

- **No `submit_response`** — agent ends with prose. Caught by the loop's `synthesized: true, reason: end_turn_without_submit` event.
- **`max_turns` exhaustion** — `reason: max_turns`. Usually means the agent is stuck reading too much.
- **Hallucinated citation** — `used_items[*].path` resolves to nothing in `loadVFS`. `resolveUsedTitles` will leave `Title` and `MaterialID` blank; the UI renders unresolved citations as broken.
- **Empty `missing_context` on a gappy answer** — symptom of overconfidence. Hard to detect mechanically; caught by manual review against case 3 / 5.
- **Single-material bias** — agent reads one file and answers; check `used_items.length` against gold for synthesis cases.
- **Wrong modality** — agent reads `page_N.md` for a figure-heavy question instead of `page_N.pdf`. Caught by case 4.

## How to run

1. Load the 8-material LLM workspace (see `docs/dataset.md`).
2. For each case, paste the prompt into Ask, wait for `ready`, then open the request to view the trace.
3. Score the three dimensions by hand. Total score = average across cases.
4. Re-run after any prompt or tool change. The trace is fully replayable from `request_events`, so old runs stay auditable.

## If this scaled up

Swap manual scoring for an LLM grader: feed it the case prompt, the gold expectations, and the agent's `submit_response`, and ask for the same three 0/0.5/1 scores plus a short note. The rubric stays the same; only the grader changes.
