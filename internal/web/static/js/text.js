// The text folding of the search bar. It must match query.Fold and
// query.Normalize in keyforge-data: internal/web/text_test.go runs this
// file against them. search.js reads it as globalThis.kfText.
globalThis.kfText = (() => {
  "use strict";

  // pairs maps the special characters of KeyForge text to ASCII before the
  // diacritics are removed, as foldPairs in query/fold.go does.
  const pairs = {
    "’": "'",
    "‘": "'",
    "ʼ": "'",
    "“": '"',
    "”": '"',
    "Æ": "ae",
    "æ": "ae",
    "Œ": "oe",
    "œ": "oe",
    "ß": "ss",
    "Ø": "o",
    "ø": "o",
    "Ł": "l",
    "ł": "l",
    "×": "x",
    "‐": "-",
    "‑": "-",
    "–": "-",
    "—": "-",
    "−": "-",
    " ": " ",
    "﻿": "",
    "​": "",
    "…": "...",
  };
  const pairRe = new RegExp(`[${Object.keys(pairs).join("")}]`, "g");

  // fold returns the text in lower case without diacritics: "Fábio" is
  // "fabio", "Æmber" is "aember", and "Ol’" is "ol'".
  const fold = (s) =>
    s.replace(pairRe, (c) => pairs[c])
      .normalize("NFD")
      .replace(/\p{Mn}/gu, "")
      .normalize("NFC")
      .toLowerCase();

  // norm returns the folded text with only the letters and digits.
  const norm = (s) => fold(s).replace(/[^a-z0-9]/g, "");

  return { fold, norm };
})();
