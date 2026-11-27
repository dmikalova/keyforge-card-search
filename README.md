# gallery.keyforge.cards

The KeyForge card gallery at gallery.keyforge.cards.

- Go, templ, and Datastar.
- The card data and the query engine come from
  [keyforge-data](https://github.com/dmikalova/keyforge-data).
- Cloud Run, with the images in the public bucket of the keyforge-cards stack.

Read [CONTEXT.md](CONTEXT.md) for the vocabulary. Read [docs/adr](docs/adr) for
the design decisions. Read [docs/todo.md](docs/todo.md) for the work plan.

The card text, images, and icons are the property of Ghost Galaxy. This project
is a fan project and is not affiliated with Ghost Galaxy.

## Routes

| Route             | Purpose                                                       |
| ----------------- | ------------------------------------------------------------- |
| `GET /?q=`        | The search page with the first batch and the facets           |
| `GET /batch`      | The next batch, as a Datastar event stream                    |
| `GET /results`    | A search bar operation (type, Enter, facet), as events        |
| `GET /card`       | The card view, or a random card of the query, as events       |
| `GET /random?q=`  | A redirect to one random card of the query, without JS        |
| `GET /syntax`     | The query syntax                                              |
| `GET /api/cards`  | The results as JSON, with absolute image URLs                 |
| `GET /static/*`   | The embedded CSS, JS, fonts, and images                       |
| `GET /icons/*`    | The published icons, served from this origin for CSS masks    |
| `GET /health`     | The dataset version and card count, as JSON                   |

Run it locally with `go run ./cmd/search` and open <http://localhost:8080>.
Without `DATASET_URL`, it serves the hand-written fixture. With the real data:

```sh
DATASET_URL=https://storage.googleapis.com/mklv-keyforge-cards-public/data/latest.json go run ./cmd/search
```
