# The text query is canonical

## Context

The search needs boolean logic, a URL that users can share, a query box that
users can edit, and controls that users can click. Deck search uses the same
language with more fields. Ratings come later.

## Decision

The text query is the source of truth. The URL holds it. The search bar shows
its top-level terms as chips, and the facets add and remove terms in it.

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
| name    | n     | name: a substring, ranked by match quality    |
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

Field `first` (no alias): the set of the card's first printing (`first:gr`).

Flags (`is:`): `anomaly`, `reprint`, `houseless`, `reference`, `tokencreature`,
`gigantic`, `eviltwin`, and the houseless kind labels (`skybeast`, `revenant`
…).

Field `group` (alias `g`): the group of a card without a house (`group:anomaly`).

Directives: `sort:` (a comma list of name, set, house, type, rarity, power,
armor, aember, enhance, with `-` for a descending key; or random alone),
`order:` (asc, desc), `unique:prints`, `seed:`.

A comma list is an OR of the values: `s:aoa,wc` is `(s:aoa or s:wc)`. Enum
fields, `is:`, and the keys of `k:` and `en:` take a list.

Rules:

- A term matches a card if one of the card's printings matches it. So
  `s:aoa s:wc` gives the cards with a printing in AoA and a printing in WC,
  and `s:aoa -s:wc` gives the cards in AoA that are not in WC.
- A result shows the best printing (the newest set, a normal printing before
  an anomaly) of the printings that the set, rarity, number, flavor,
  `is:anomaly`, and `is:reprint` terms outside a NOT select. If no printing is
  selected, it shows the best printing of all.
- `is:anomaly` gives the cards with an anomaly printing. `is:reprint` gives the
  cards in more than one set. `first:gr` gives the cards that are new in Grim
  Reminders.
- Name search is a substring match on the normalized name (no case,
  diacritics, spaces, or punctuation) and the aliases. The match tiers, from
  the best: the whole name, a prefix, a match at a word start, then a match
  inside a word. A shorter name ranks higher inside a tier.
- If the query has a name term or a bare word and no `sort:`, the name match
  quality sets the order.
- The default sort is house, then name. The cards without a house sort after
  the other cards, in the order of their groups (`card.HouselessGroups`).
  Reference cards are last.
- `sort:` keys sort in order: each key sorts the ties of the keys before it,
  and the default sort sorts the last ties. `order:desc` reverses the whole
  order.
- Power X does not match a numeric comparison.

## Consequences

- A user can copy the URL, edit the query, or click controls. All three give
  the same state.
- A chip is a top-level term in canonical form. A group in parentheses or an
  OR is one chip.
- Deck search adds fields to the same registry.
