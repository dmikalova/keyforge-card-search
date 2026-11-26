# The text query is canonical

## Context

The search needs boolean logic, a URL that users can share, a query box that
users can edit, and controls that users can click. Deck search uses the same
language with more fields. Ratings come later.

## Decision

The text query is the source of truth. The URL holds it. The builder writes
terms to it.

Grammar:

- `a b` and `a and b` mean AND. `or` means OR. `not a` and `-a` mean NOT.
  Parentheses group terms.
- `"..."` is an exact phrase. There is no regex.
- Numeric operators: `= != < <= > >=` and ranges (`p:2..4`).
- Field names are not case-sensitive. Each field has one alias. An unknown
  field gives an error at the token, with a "did you mean" suggestion.
- Enum values match exactly. Text values match a substring.

| Field   | Alias | Type                                          |
| ------- | ----- | --------------------------------------------- |
| name    | n     | fuzzy text                                    |
| text    | x     | text                                          |
| flavor  | fl    | text                                          |
| house   | h     | enum                                          |
| set     | s     | enum (code, slug, or name)                    |
| type    | t     | enum                                          |
| rarity  | r     | enum                                          |
| keyword | k     | enum, with a value (`k:assault>=2`)           |
| trait   | tr    | enum                                          |
| number  | #     | number or string (`#001`, `#S01`)             |
| power   | p     | number or X (`p:x`)                           |
| armor   | ar    | number                                        |
| aember  | a     | number (bonus aember)                         |
| enhance | en    | number, total or per icon (`en:capture>=1`)   |

Flags (`is:`): `anomaly` (per printing), `reprint`, `new` (`-is:reprint`),
`reference`, `tokencreature`, `gigantic`, `eviltwin`, and the houseless kind
labels (`skybeast`, `revenant` …).

Directives: `sort:` (name, set, house, type, rarity, power, armor, aember,
enhance, random), `order:` (asc, desc), `unique:prints`, `seed:`.

Rules:

- Name search is a fuzzy subsequence match. It matches the folded name and the
  aliases. If the query has only name terms and no `sort:`, the match quality
  sets the order.
- `is:reprint` is relative to the selected sets. `s:gr -is:reprint` gives the
  cards that are new in Grim Reminders.
- `-is:reprint` uses the printing in the first set where the card appears.
- The default sort is house, then name. Reference cards and houseless cards
  sort after all other cards, unless the query asks for them.
- Power X does not match a numeric comparison.

## Consequences

- A user can copy the URL, edit the query, or click controls. All three give
  the same state.
- The builder cannot show some queries. It shows "custom query" for these.
- Deck search adds fields to the same registry.
