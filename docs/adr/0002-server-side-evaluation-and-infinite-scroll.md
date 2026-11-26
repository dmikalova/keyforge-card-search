# Server-side evaluation and infinite scroll

## Context

Datastar sends HTML from the server. Client-side search in Go would need a
WASM file of 2–5 MB. The card data is about 7,500 cards, so it fits in memory.
Deck search cannot run in the client (about 4M decks).

All card images are 300×420 webp, about 34 KB each. The originals have the same
size. They are in the public bucket with a one-year immutable cache header.

## Decision

- Evaluate queries on the server, in memory, with the keyforge-data
  evaluator. The site reads the card dataset from the public bucket
  (`DATASET_URL`) at start and checks it every 5 minutes with an ETag. A new
  dataset needs no deploy. On an error the site keeps the last good dataset.
- Datastar requests a batch of about 60 results. The grid has a sentinel. An
  IntersectionObserver asks for the next batch when the sentinel comes within
  two viewports. Images use `loading="lazy"` and fixed-aspect placeholders.
- Remove far-away tiles from the DOM only if the grid has more than 2,000
  tiles.
- Do not make thumbnails. The images are already small.
- Do not use a CDN until the bucket egress is about 450 GB a month. Below that
  amount, the load balancer cost is more than the egress savings.
- Do not use a service worker in 1.0. The HTTP cache keeps the images and the
  assets.

## Consequences

- Each new query needs the server.
- The zoom view and the largest card size cannot be sharper than 300 px
  wide.
