# gallery.keyforge.cards

The vocabulary of the card gallery site. The card vocabulary (card, printing,
set, reference card …) is in the keyforge-data CONTEXT.md. This file is a
glossary only.

**Query**:
The text that describes a search. The query is the source of truth for the
page state, and the URL holds it (`?q=`).

**Term**:
One condition in a query: a field, an operator, and a value (`p>=3`), a flag
(`is:gigantic`), or a bare word.

**Bare word**:
A term without a field. It searches the name, the text, and the flavor text.

**Directive**:
A term that changes the display and does not filter: `sort:`, `order:`,
`unique:`, `seed:`.

**Builder**:
The panel of controls (house icons, set icons, number ranges, text boxes) that writes terms to the query. If the query has a structure that the
builder cannot show, the builder shows "custom query" and becomes read-only.

**Result**:
One tile in the grid. By default, a result is one card with its default
printing. With `unique:prints`, a result is one printing.

**Batch**:
The group of about 60 results that the server sends at one time. The grid
asks for the next batch when the sentinel comes within two viewports.
_Avoid_: page.

**Facet**:
A count of the results by one field (house, type, rarity, set), or a histogram
(power, armor). A click on a facet adds a term.

**Zoom view**:
The large view of one card. A click or tap opens it and closes it.
