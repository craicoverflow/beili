# Recipe Feedback & Fork

## Problem Statement
How might we turn "that was good, but too sweet" into a better version of the recipe, so my household's go-to meals get tuned to our taste each time we cook them?

## Recommended Direction
**Feedback-and-fork, built on a note-first foundation.** After marking a meal cooked, you describe what was off in a free-text box. The feedback is always saved, even if you never use the AI.

On request, the AI (Sonnet) returns a full revised ingredient and instruction list plus a short reason for each change. The app computes the diff in code by comparing the revised list to the original, so it never relies on the model to describe its own edits. You review the diff, accept or tweak it, and save it as a new meal linked to the original by `parent_meal_id`. The fork starts fresh: no ratings or cooked history are copied.

A meal with at least one fork is considered superseded and is excluded from the random picker and plan calendar. Superseded is derived from fork existence, not stored, so deleting the last fork brings the parent back automatically. The parent shows "N forks" and the fork shows "forked from X".

When revising a fork of a fork, the AI sees feedback from the whole lineage so it doesn't undo an earlier fix.

This fits the existing code. `internal/ai` already has a `Provider` interface (Gemini and Claude Haiku) with a `Standardize` method that turns text into structured ingredients and instructions; `Revise` has the same shape. The cooked log (`002`) is the natural place to attach feedback.

## Key Assumptions to Validate
- [ ] **The AI's fix for a taste complaint is usually right on the first try.** Run 10 real complaints against 5 of your own recipes with a throwaway script and count fixes you'd accept unedited. Aim for 7 of 10.
- [ ] **The revised list can be diffed against the original reliably.** Ingredients are free-form strings in `StringList`. Round-trip revised output through the ingredient parser and the `internal/scaling` tests; the fork must scale correctly at other serving sizes.
- [ ] **A fork is the right unit.** Use it for a month. If you end up with three or more forks of one recipe, add a promote or collapse step.
- [ ] **Hiding superseded parents matches how you cook.** With no undo in the MVP, watch whether you miss the original.

## MVP Scope
**In:**
- Migration `010`: a `meal_feedback` table (meal, free text, timestamp) and `parent_meal_id` on meals. Existing migrations are not modified. No stored `superseded` column.
- A free-text feedback box on the cooked-log flow, saved without needing the AI.
- A `Revise` method on the `Provider` interface using Sonnet, with its own model setting separate from the Haiku import model. Input: the recipe, the new feedback, and earlier feedback from the lineage. Output: metric units, normalised to `BASE_SERVINGS`.
- A diff review screen with the AI's one-line reason per change; editable before saving.
- Save as fork: a new meal with a lineage link and fresh history.
- Superseded parents excluded from the random picker and plan calendar; deleting the last fork restores the parent.
- Store-layer methods and HTMX handlers; edit `.templ` sources and run `make generate`.
- Tests for diff computation, fork creation, lineage feedback in the prompt, and picker and calendar exclusion.

## Not Doing (and Why)
- **Feedback chips** — Free text is how it will actually be used; add chips later if the same phrases keep recurring.
- **Undo for superseded parents** — Deriving superseded from fork existence means an undo needs a separate override. The original can still be opened directly; revisit if missed.
- **Versioned recipes with one identity** — Every query, the calendar, the shopping list and the FTS index would need to understand versions.
- **Queuing several notes before revising** — Adds state and UI; one piece of feedback per revision tests the core idea.
- **Cross-recipe learning ("this household likes less sweet")** — Needs a lot of data and a preference model. Revisit after a month of feedback.
- **Per-person feedback or variants** — The household is small; multi-user disagreement is a different problem.
- **Merging or collapsing forks** — Wait to see whether fork clutter is real.
- **Client-side quantity parsing** — Scaling stays in `internal/scaling`.
- **Unauthenticated write endpoints** — Béilí is public via the Cloudflare tunnel, so revise and fork endpoints go behind the existing auth.

## Open Questions
- None blocking. Resolved: forks start fresh; deleting a fork un-supersedes its parent; free text over chips; Sonnet for `Revise`; the AI sees lineage feedback.
