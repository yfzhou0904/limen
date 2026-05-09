# Evaluation

How I'd judge whether this is actually useful.

## Three things to score per question

- **Answer quality** — correct, grounded in what was read, no fabrication.
- **Context selection** — `used_items` contains the obviously-relevant materials, nothing irrelevant.
- **Failure handling** — when info is missing, `missing_context` says what specifically; `next_actions` are useful.

## Cases (against the LLM workspace)

1. **Factual.** *"Why is attention O(n²)?"* → cites the Attention paper, `missing_context` empty.
2. **Synthesis.** *"Summarize what I know about long-context inference cost; what's missing?"* → cites ≥3 materials, real gap in `missing_context`, 2–3 concrete `next_actions`.
3. **Out-of-scope.** *"What's the latest on Mamba?"* → refuses or gives a short answer; flags the missing topic. Watch: agent must not fabricate from related material.

## What to watch for in the trace

- **No `submit_response`** — event `reason: end_turn_without_submit`.
- **`max_turns` hit** — agent is reading too much.
- **Hallucinated citation** — `used_items[*].path` doesn't resolve in the VFS (UI shows a broken citation).
- **Empty `missing_context` on a clearly partial answer** — overconfidence.
- **Wrong modality** — agent reads `page_N.md` when the question needs the `.pdf` figure.

Score by hand from `request_events`. The rubric is small enough to swap manual scoring for an LLM grader later if it ever matters.
