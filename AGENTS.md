# keyforge-card-search

Conventions for this repository. The vocabulary is in [CONTEXT.md](CONTEXT.md).
The decisions are in [docs/adr](docs/adr). The work plan is in
[docs/todo.md](docs/todo.md).

## Validation

Run `mage ci:fix && mage ci:check` before you call work done. It must print
`ALL GREEN`. The build step also fails if a `.templ` file changed without
`mage templ`.

## Package layout

| Path               | Role                                                         |
| ------------------ | ------------------------------------------------------------ |
| `cmd/search`       | The Cloud Run entrypoint: start the dataset loader, serve    |
| `internal/catalog` | The dataset loader (`DATASET_URL`, ETag), and image URLs     |
| `internal/fixture` | The hand-written dataset for local work and tests            |
| `internal/search`  | The query fields, the default printing, the sort, the facets |
| `internal/server`  | The routes: page, batch, random, syntax, JSON export         |
| `internal/web`     | templ pages, view models, and the embedded static files      |

## Rules

- **The query is the state.** Every page state is in `?q=`. Do not add page
  state that the URL does not hold. `sort:random` gets a `seed:` by redirect.
- **The query language is in keyforge-data.** Add a field in
  `internal/search/fields.go`. Change the grammar in keyforge-data/query.
- **The fixture is not card data.** `internal/fixture/testdata/cards.json` is
  hand-written for the special cases. Some of its numbers and texts are made
  up. The site serves it only when `DATASET_URL` is not set.
- **Do not embed the dataset.** The site reads it from the public bucket, so
  a new dataset needs no deploy.
- **Commit the generated templ code.** Run `mage templ` after you edit a
  `.templ` file.
- **Keep the JavaScript small.** Datastar loads the batches.
  `internal/web/static/js/search.js` holds only the zoom view, the card size,
  and the keyboard shortcuts.
