# Todo

The work plan for gallery.keyforge.cards. This work starts after phase 5 of the
keyforge-data todo. The vocabulary is in [../CONTEXT.md](../CONTEXT.md).

## Setup

- [x] Add `go.mod` (`github.com/dmikalova/gallery.keyforge.cards`),
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
- [x] Write the name match with match quality. Fuzzy matching is gone: a
  name term is a substring match.
- [x] Add golden tests and a Go fuzz test for the parser. Add table tests for the
  evaluator with a small set of fixture cards.

## Site 1.0

- [x] Read the keyforge-data dataset from the public bucket at run time, and
  reload it when it changes.
- [x] Comma lists, card-level terms, and match tiers.
- [x] No overlays on the tiles. The anomaly rarity icon is in the card view
  and the rarity facet only.
- [x] Page layout with the keyforge.cards purple and orange theme.
- [x] The query box. Show parse errors at the token.
- [x] The search bar is one text input. The results change as the user types.
  A focus or a click opens the completion list with every field and its
  description. After `field:` the list shows the values. The list completes
  the word at the caret, also in the middle of the query. The chips are gone.
- [x] The facet values toggle their terms, with spread bars for each facet.
- [x] Keyword, ability (`ab:`), and trait facets.
- [x] The card view shows every field of the card, the sets, the related
  cards, and links to the cards that the text names. The URL holds the open
  card (`card=ID`), so a reload and the back button work.
- [x] The header stays on the screen, and the card view opens below it.
- [x] `sort:` takes a comma list of keys, with `-` for a descending key.
- [x] The house facet puts every card without a house in one bucket, Other
  (`h:none`). `group:` and `is:houseless` select one part of it.
- [x] House, type, rarity, and set icons in the facets and the card view. The site serves the icons at /icons/ from the public bucket.
- [ ] Minimum and maximum for power, armor, and aember. Sort and order
  controls in the page.
- [x] The grid with batches, the sentinel, and lazy images (ADR 0002).
- [x] The card size slider.
- [x] The zoom view (click or tap to open and close).
- [x] The result count and the facets: house, type, rarity, and set counts, and
  power and armor histograms.
- [x] `sort:random` with a seed in the URL. The random card button.
- [x] JSON export of the results, with absolute image URLs.
- [x] Keyboard shortcuts: `/`, `Esc`, the arrow keys and `hjkl`, `Enter`, `r`, `?`,
  `+`, `-`. (`b` opened the builder, which is gone.)
- [x] The `/syntax` page, linked from a `?` bubble that opens it as a modal.
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
