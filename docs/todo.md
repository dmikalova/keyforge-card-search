# Todo

The work plan for search.keyforge.cards. This work starts after phase 5 of the
keyforge-data todo. The vocabulary is in [../CONTEXT.md](../CONTEXT.md).

## Setup

- [x] Add `go.mod` (`github.com/dmikalova/keyforge-card-search`),
  `mklv.config.json` (`kind: cloudrun`), `lefthook.jsonc`, `paseo.json`,
  `AGENTS.md`, the Dockerfile, and the magefiles with the project-standards
  `ci` import.
- [ ] Add the infra stack `infrastructure/gcp/apps/search` with the
  `cloud-run-app` module and `app_name = "search"` in the keyforge-cards zone.
  Do not add a database.

## Query engine (in keyforge-data/query)

- [x] Write the lexer, the parser, and the AST (ADR 0001).
- [x] Write the field registry and register the card fields (`internal/search`).
- [x] Write the in-memory evaluator.
- [x] Write the fuzzy name match with match quality.
- [x] Add golden tests and a Go fuzz test for the parser. Add table tests for the
  evaluator with a small set of fixture cards.

## Site 1.0

- [ ] Replace the fixture catalog with the keyforge-data dataset (after phase 4
  of the keyforge-data todo).
- [x] Page layout with the keyforge.cards purple and orange theme.
- [x] The query box. Show parse errors at the token.
- [ ] Autocomplete after `field:` and for field names.
- [ ] The builder. A stub with house links is in place:
  - House icons and set icons, with all, none, and "exclude reprints".
  - Type and rarity chips.
  - Minimum and maximum for power, armor, and aember.
  - Keyword and trait lists, and the enhance count.
  - Name, text, and flavor boxes.
  - Sort and order.
- [x] The grid with batches, the sentinel, and lazy images (ADR 0002).
- [x] The card size slider.
- [x] The zoom view (click or tap to open and close).
- [x] The result count and the facets: house, type, rarity, and set counts, and
  power and armor histograms.
- [x] `sort:random` with a seed in the URL. The random card button.
- [x] JSON export of the results, with absolute image URLs.
- [x] Keyboard shortcuts: `/`, `Esc`, the arrow keys and `hjkl`, `Enter`, `r`, `?`,
  `+`, `-`, `b`.
- [x] The `/syntax` page, linked from a `?` bubble.
- [ ] A smoke e2e test with go-rod: load `/`, run a query, and count the tiles.

## Launch

- Add a link from the keyforge.cards landing page.
- Add a link in the keyfordle footer.

## Later

- `/card/{id}` pages with all printings, all languages, and variant groups.
  Then add external links to the tiles.
- The artist field (after the OCR pass in keyforge-data).
- Card ratings as a new numeric field.
- A table view and CSV export.
- Theme support.
