# keyforge-card-search

The KeyForge card search at search.keyforge.cards.

- Go, templ, and Datastar.
- The card data and the query engine come from
  [keyforge-data](https://github.com/dmikalova/keyforge-data).
- Cloud Run, with the images in the public bucket of the keyforge-cards stack.

Read [CONTEXT.md](CONTEXT.md) for the vocabulary. Read [docs/adr](docs/adr) for
the design decisions. Read [docs/todo.md](docs/todo.md) for the work plan.

The card text, images, and icons are the property of Ghost Galaxy. This project
is a fan project and is not affiliated with Ghost Galaxy.

## Routes

| Route            | Purpose                                              |
| ---------------- | ---------------------------------------------------- |
| `GET /?q=`       | The search page with the first batch and the facets  |
| `GET /batch`     | The next batch, as a Datastar event stream           |
| `GET /random?q=` | A redirect to one random card of the query           |
| `GET /syntax`    | The query syntax                                     |
| `GET /api/cards` | The results as JSON, with absolute image URLs        |
| `GET /static/*`  | The embedded CSS, JS, fonts, and images              |
| `GET /health`    | The health check                                     |

Run it locally with `go run ./cmd/search` and open <http://localhost:8080>.
