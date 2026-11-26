// The card view, the search bar, the card size, and the keyboard shortcuts
// of the search page. Datastar loads the results and the card view. This
// file only reacts to the user.
(() => {
  "use strict";

  const sizeKey = "kfsearch.size";
  const minSize = 120;
  const maxSize = 300;
  const step = 20;

  const load = () => {
    try {
      return Number(localStorage.getItem(sizeKey)) || 0;
    } catch {
      return 0;
    }
  };

  const save = (n) => {
    try {
      localStorage.setItem(sizeKey, String(n));
    } catch {
      // Storage can be off. The size then lasts for this page only.
    }
  };

  const setSize = (n) => {
    const size = Math.min(maxSize, Math.max(minSize, n));
    document.documentElement.style.setProperty("--tile", `${size}px`);
    const input = document.getElementById("size");
    if (input) {
      input.value = String(size);
      // The filled part of the track, for the browsers without a progress
      // pseudo-element.
      const pct = ((size - minSize) / (maxSize - minSize)) * 100;
      input.style.setProperty("--fill", `${pct}%`);
    }
    save(size);
  };

  const current = () => load() || 180;

  const $ = (id) => document.getElementById(id);
  const zoom = () => $("zoom");
  const input = () => $("q");

  // ---- The card view -------------------------------------------------

  // The URL holds the open card (card=ID), so a reload and the back button
  // work. A card view that the page opened puts a history entry, so back
  // closes it or goes to the card before.
  let opener = null;
  let fromHistory = false;

  const cardParam = () => new URLSearchParams(location.search).get("card");

  const withCard = (id) => {
    const url = new URL(location.href);
    if (id) url.searchParams.set("card", id);
    else url.searchParams.delete("card");
    return url;
  };

  const hideZoom = () => {
    const z = zoom();
    if (!z || z.hidden) return false;
    z.hidden = true;
    if (opener && document.contains(opener)) opener.focus();
    return true;
  };

  const closeZoom = () => {
    const z = zoom();
    if (!z || z.hidden) return false;
    // Each card viewed from the card view is a history entry, so back and
    // forward step between the cards. Closing goes back past all of them in
    // one step, to the results.
    if (history.state && history.state.kfcard) {
      history.go(-(history.state.depth || 1));
    } else {
      history.replaceState(history.state, "", withCard(""));
      hideZoom();
    }
    return true;
  };

  // openCard asks the server for the card view of a card.
  const openCard = (id, p) => {
    const b = $("cardload");
    if (!b || !id) return;
    b.dataset.id = id;
    b.dataset.p = p == null ? "" : String(p);
    b.click();
  };

  // showPrinting shows the image and the flavor text of one printing.
  const showPrinting = (btn) => {
    const z = zoom();
    const img = $("zoom-img");
    if (img && btn.dataset.img) img.src = btn.dataset.img;
    for (const f of z.querySelectorAll(".flavor[data-print]")) {
      f.hidden = f.dataset.print !== btn.dataset.print;
    }
    for (const b of z.querySelectorAll(".printing")) {
      b.classList.toggle("active", b === btn);
    }
  };

  window.addEventListener("popstate", () => {
    const id = cardParam();
    const z = zoom();
    if (!id) {
      hideZoom();
      return;
    }
    if (z && !z.hidden && z.dataset.card === id) return;
    fromHistory = true;
    openCard(id);
  });

  // ---- The live search calls these from the server's scripts --------

  window.kfsearch = {
    opened(id) {
      const z = zoom();
      if (!z) return;
      // One modal at a time: the card view replaces the syntax modal.
      const sm = document.getElementById("syntax-modal");
      if (sm) sm.hidden = true;
      const wasOpen = !z.hidden;
      z.hidden = false;
      if (id && cardParam() !== id && !fromHistory) {
        // depth counts the card entries since the results, so closing can go
        // back to the results in one step.
        const depth = wasOpen && history.state && history.state.kfcard
          ? (history.state.depth || 1) + 1
          : 1;
        history.pushState({ kfcard: true, depth }, "", withCard(id));
      }
      fromHistory = false;
      z.querySelector(".zoom-close")?.focus({ preventScroll: true });
    },
    // update puts a new query in the URL and the title. It drops the place
    // in the results (at=N) and closes the card view, because the results
    // are new.
    update({ url, title }) {
      const now = new URL(location.href);
      const next = new URL(url, location.href);
      if (now.searchParams.get("q") !== next.searchParams.get("q")) {
        history.replaceState(null, "", next);
        hideZoom();
        window.scrollTo(0, 0);
      }
      if (title) document.title = title;
    },
  };

  // ---- Autocomplete ----------------------------------------------------

  // The search bar completes the word at the caret. A focus or a click
  // opens the list: with no word at the caret, it shows every field with a
  // short description. After "field:" it shows the values of the field.

  let acData = null;
  const acFields = () => {
    if (acData) return acData;
    try {
      acData = JSON.parse($("acdata")?.dataset.json || "{}").fields || [];
    } catch {
      acData = [];
    }
    return acData;
  };

  const fold = (s) =>
    s.toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "")
      .replace(/æ/g, "ae");

  const opRe = /(!=|<=|>=|=|:|<|>)/;
  const empty = { items: [], at: -1, start: 0, end: 0 };
  let ac = empty;

  const closeAC = () => {
    const ul = $("ac");
    if (ul) {
      ul.hidden = true;
      ul.replaceChildren();
    }
    input()?.setAttribute("aria-expanded", "false");
    ac = empty;
  };

  // label is the text of a field in the list: "house:" or "is:".
  const label = (f) => `${f.name}:`;

  // fieldItems are the fields whose name or alias starts with the prefix.
  const fieldItems = (p) =>
    acFields()
      .filter((f) => !p || f.name.startsWith(p) || (f.alias && f.alias.startsWith(p)))
      .map((f) => ({
        text: label(f),
        insert: label(f),
        note: f.alias ? `${f.alias}: · ${f.desc || ""}` : f.desc || "",
        field: true,
      }));

  // opItems are the operators that can follow a term.
  const opItems = (p) =>
    [
      { text: "or", insert: "or ", note: "Either side matches" },
      { text: "not", insert: "not ", note: "The next term does not match" },
      { text: "(", insert: "(", note: "Start a group" },
    ].filter((o) => !p || o.text.startsWith(p));

  // numItems are the comparisons that can follow a field or a key that
  // takes a number. base is the text before the operator ("power",
  // "keyword:assault"), and rangeBase is the text before a range.
  const numItems = (base, rangeBase) =>
    [
      ["=", "Equal to"],
      ["!=", "Not equal to"],
      ["<", "Less than"],
      ["<=", "Less than or equal to"],
      [">", "More than"],
      [">=", "More than or equal to"],
    ]
      .map(([o, note]) => ({ text: o, insert: base + o, note, num: true }))
      .concat([
        { text: ":2..4", insert: rangeBase, note: "A range: type the low and the high number", num: true },
      ]);

  // norm is a value without case and punctuation, as the query compares it.
  const norm = (s) => s.toLowerCase().replace(/[^a-z0-9]/g, "");

  // wordAt returns the word around the caret: its start and end, and the
  // part before the caret. A word ends at a space or a parenthesis.
  const wordAt = (q) => {
    const caret = q.selectionStart ?? q.value.length;
    const before = q.value.slice(0, caret);
    const after = q.value.slice(caret);
    const start = caret - (before.match(/[^\s()]*$/)?.[0].length ?? 0);
    const end = caret + (after.match(/^[^\s()]*/)?.[0].length ?? 0);
    return { start, end, word: q.value.slice(start, caret), caret };
  };

  // suggest fills the list for the word at the caret.
  const suggest = () => {
    const q = input();
    if (!q || document.activeElement !== q) return closeAC();
    const { start, end, word } = wordAt(q);
    const neg = word.startsWith("-") ? 1 : 0;
    const w = word.slice(neg);
    let items = [];
    let from = start + neg;
    const op = w.match(opRe);
    if (op) {
      const name = w.slice(0, op.index).toLowerCase();
      const f = acFields().find((x) => x.name === name || x.alias === name);
      const rest = w.slice(op.index + op[0].length);
      // After "power:" or "keyword:assault", where a number goes, suggest
      // the comparisons.
      if (f && op[0] === ":" && rest === "" && f.num) {
        const base = w.slice(0, op.index);
        ac = { items: numItems(base, base + ":"), at: -1, start: start + neg, end };
        return render();
      }
      if (f && op[0] === ":" && rest && (f.numkeys || []).includes(norm(rest))) {
        ac = { items: numItems(w, w + ":"), at: -1, start: start + neg, end };
        return render();
      }
      if (!f || !f.values) return closeAC();
      let vs = op.index + op[0].length;
      if (f.list) {
        const comma = w.lastIndexOf(",");
        if (comma >= vs) vs = comma + 1;
      }
      if (f.name === "sort" && w[vs] === "-") vs++;
      const p = fold(w.slice(vs));
      // A value matches by its query text or by its label: "worlds" finds
      // wc, whose label is "Worlds Collide".
      const label = (v) => (f.labels && f.labels[v]) || "";
      const starts = f.values.filter((v) => fold(v).startsWith(p) || fold(label(v)).startsWith(p));
      const has = f.values.filter((v) => !starts.includes(v) &&
        (fold(v).includes(p) || fold(label(v)).includes(p)));
      items = starts.concat(has).filter((v) => fold(v) !== p)
        .map((v) => ({ text: v, insert: v, note: label(v) }));
      from = start + neg + vs;
    } else {
      const p = w.toLowerCase();
      const before = q.value.slice(0, start).trim();
      // After a term, the operators come first. At the start of the query
      // or after "(", "or", or "not", only the fields make sense.
      const afterTerm = before !== "" && !/(^|\s)(or|and|not)$|\($/i.test(before);
      items = fieldItems(p);
      if (afterTerm && !neg) items = opItems(p).concat(items);
    }
    if (items.length === 0) return closeAC();
    // No item is chosen at first, so Enter still runs the query. Tab takes
    // the first item, and the arrow keys choose one.
    ac = { items, at: -1, start: from, end };
    render();
  };

  const render = () => {
    const ul = $("ac");
    if (!ul) return;
    ul.replaceChildren(
      ...ac.items.map((it, i) => {
        const li = document.createElement("li");
        li.role = "option";
        li.id = `ac-${i}`;
        li.dataset.i = String(i);
        li.setAttribute("aria-selected", String(i === ac.at));
        const t = document.createElement("span");
        t.className = "ac-text";
        t.textContent = it.text;
        li.append(t);
        if (it.note) {
          const n = document.createElement("span");
          n.className = "ac-note";
          n.textContent = it.note;
          li.append(n);
        }
        return li;
      }),
    );
    ul.hidden = false;
    const q = input();
    q?.setAttribute("aria-expanded", "true");
    if (ac.at >= 0) {
      q?.setAttribute("aria-activedescendant", `ac-${ac.at}`);
      ul.children[ac.at]?.scrollIntoView({ block: "nearest" });
    } else {
      q?.removeAttribute("aria-activedescendant");
    }
  };

  // accept puts the chosen completion in place of the word at the caret.
  const accept = (i) => {
    const q = input();
    const it = ac.items[i];
    if (!q || it == null) return;
    q.value = q.value.slice(0, ac.start) + it.insert + q.value.slice(ac.end);
    const caret = ac.start + it.insert.length;
    q.setSelectionRange(caret, caret);
    closeAC();
    q.dispatchEvent(new Event("input", { bubbles: true }));
    // The list stays open with the next suggestions: after a field, an
    // operator, or a value such as keyword:assault that takes a number. After
    // a comparison the user types the number, so the list stays closed.
    if (!it.num) suggest();
  };

  // ---- Keys and clicks -------------------------------------------------

  const tiles = () => Array.from(document.querySelectorAll(".tile"));

  // move gives focus to the tile dx columns and dy rows away.
  const move = (dx, dy) => {
    const all = tiles();
    if (all.length === 0) return;
    let i = all.indexOf(document.activeElement);
    if (i < 0) {
      all[0].focus();
      return;
    }
    const top = all[0].offsetTop;
    let cols = all.findIndex((t) => t.offsetTop !== top);
    if (cols < 0) cols = all.length;
    i = Math.min(all.length - 1, Math.max(0, i + dx + dy * cols));
    all[i].focus();
    all[i].scrollIntoView({ block: "nearest" });
  };

  const typing = (el) =>
    el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement;

  const keys = {
    ArrowLeft: () => move(-1, 0),
    ArrowRight: () => move(1, 0),
    ArrowUp: () => move(0, -1),
    ArrowDown: () => move(0, 1),
    h: () => move(-1, 0),
    l: () => move(1, 0),
    k: () => move(0, -1),
    j: () => move(0, 1),
    r: () => {
      opener = null;
      $("random")?.click();
    },
    "?": () => openSyntax(),
    "+": () => setSize(current() + step),
    "=": () => setSize(current() + step),
    "-": () => setSize(current() - step),
  };

  // The search bar's keys run in the capture phase, before Datastar's
  // Enter handler, so the autocomplete list can take Enter and Tab.
  document.addEventListener("keydown", (e) => {
    if (e.target !== input()) return;
    const stop = () => {
      e.preventDefault();
      e.stopImmediatePropagation();
    };
    // A search input clears its text on Escape. Esc here only closes the
    // list, or leaves the bar, so the query stays.
    if (e.key === "Escape") e.preventDefault();
    const open = !$("ac")?.hidden && ac.items.length > 0;
    if (open) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        stop();
        const d = e.key === "ArrowDown" ? 1 : -1;
        const n = ac.items.length;
        ac.at = ac.at < 0 ? (d > 0 ? 0 : n - 1) : (ac.at + d + n) % n;
        render();
        return;
      }
      if (e.key === "Tab" || (e.key === "Enter" && ac.at >= 0)) {
        stop();
        accept(Math.max(ac.at, 0));
        return;
      }
      if (e.key === "Enter") closeAC();
      if (e.key === "Escape") {
        stop();
        closeAC();
        return;
      }
    } else if (e.key === "ArrowDown") {
      // ArrowDown opens a closed list.
      stop();
      suggest();
    }
  }, true);

  document.addEventListener("keydown", (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "Escape") {
      if (closeSyntax()) return;
      if (closeZoom()) return;
      if (typing(e.target)) e.target.blur();
      return;
    }
    if (typing(e.target)) return;
    // With the syntax modal open, the arrow keys scroll it, and the other
    // shortcuts wait.
    const sm = syntaxModal();
    if (sm && !sm.hidden) return;
    if (e.key === "/") {
      e.preventDefault();
      const q = input();
      q.focus();
      q.select();
      return;
    }
    const z = zoom();
    if (z && !z.hidden && e.key !== "?") {
      closeZoom();
    }
    const f = keys[e.key];
    if (f) {
      e.preventDefault();
      f();
    }
  });

  // The query syntax modal. It loads the text of /syntax/content one time,
  // so the modal and the /syntax page have one source.
  const syntaxModal = () => $("syntax-modal");
  let syntaxLoaded = false;
  let syntaxOpener = null;
  const openSyntax = async () => {
    const m = syntaxModal();
    if (!m) {
      location.href = "/syntax";
      return;
    }
    if (!syntaxLoaded) {
      try {
        const r = await fetch("/syntax/content");
        if (!r.ok) throw new Error(r.statusText);
        $("syntax-body").innerHTML = await r.text();
        syntaxLoaded = true;
      } catch {
        location.href = "/syntax";
        return;
      }
    }
    closeZoom();
    syntaxOpener = document.activeElement;
    m.hidden = false;
    m.querySelector(".zoom-close")?.focus({ preventScroll: true });
  };
  const closeSyntax = () => {
    const m = syntaxModal();
    if (!m || m.hidden) return false;
    m.hidden = true;
    syntaxOpener?.focus?.({ preventScroll: true });
    syntaxOpener = null;
    return true;
  };

  document.addEventListener("click", (e) => {
    const sm = syntaxModal();
    if (sm && !sm.hidden && sm.contains(e.target)) {
      // The backdrop or the close button closes the modal. A click in the
      // text does not, so the user can select text.
      if (e.target === sm || e.target.closest(".zoom-close")) closeSyntax();
      return;
    }
    const help = e.target.closest("a.help");
    if (help && !e.metaKey && !e.ctrlKey && !e.shiftKey && e.button === 0) {
      e.preventDefault();
      openSyntax();
      return;
    }
    const li = e.target.closest("#ac li");
    if (li) {
      e.preventDefault();
      accept(Number(li.dataset.i));
      input()?.focus();
      return;
    }
    const link = e.target.closest("a[data-card]");
    if (link && !(e.metaKey || e.ctrlKey || e.shiftKey)) {
      e.preventDefault();
      openCard(link.dataset.card);
      return;
    }
    const z = zoom();
    if (z && !z.hidden && z.contains(e.target)) {
      const btn = e.target.closest(".printing");
      if (btn) {
        showPrinting(btn);
        return;
      }
      // A click on the backdrop or the close button closes the view. A
      // click in the card's text does not, so the user can select text.
      if (e.target === z || e.target.closest(".zoom-close")) closeZoom();
      return;
    }
    const tile = e.target.closest(".tile");
    if (tile) opener = tile;
  });

  document.addEventListener("input", (e) => {
    if (e.target.id === "size") setSize(Number(e.target.value));
    if (e.target.id === "q") suggest();
  });

  // A focus, a click, or a caret move in the bar opens the list for the
  // word at the caret.
  document.addEventListener("focusin", (e) => {
    if (e.target.id === "q") suggest();
  });
  document.addEventListener("mouseup", (e) => {
    if (e.target.id === "q") suggest();
  });
  document.addEventListener("keyup", (e) => {
    if (e.target.id === "q" && ["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) {
      suggest();
    }
  });
  // A press on the list keeps the focus in the bar.
  document.addEventListener("mousedown", (e) => {
    if (e.target.closest("#ac")) e.preventDefault();
  });

  document.addEventListener("focusout", (e) => {
    if (e.target.id === "q") setTimeout(closeAC, 150);
  });

  // ---- The header height ----------------------------------------------

  // The header stays on the screen. The card view starts below it, so the
  // user can type a new query with a card open.
  const header = document.querySelector("header.top");
  const setTop = () => {
    if (header) {
      document.documentElement.style.setProperty("--top-h", `${header.offsetHeight}px`);
    }
  };
  if (header && "ResizeObserver" in window) new ResizeObserver(setTop).observe(header);

  // ---- The place in the results ----------------------------------------

  // The page keeps the index of the top tile in the URL (at=N), so a reload
  // or a back navigation comes back to it. The server renders the tiles up
  // to that index. The parameter is not part of the query text.
  if ("scrollRestoration" in history) history.scrollRestoration = "manual";

  const topH = () => (header ? header.offsetHeight : 0);

  const topTile = () => {
    const y = topH();
    for (const t of document.querySelectorAll(".tile")) {
      if (t.getBoundingClientRect().bottom > y) return t;
    }
    return null;
  };

  const savePlace = () => {
    const t = topTile();
    const at = t ? Number(t.dataset.i) : 0;
    const url = new URL(location.href);
    const old = url.searchParams.get("at") || "0";
    if (String(at) === old) return;
    if (at > 0) url.searchParams.set("at", String(at));
    else url.searchParams.delete("at");
    history.replaceState(history.state, "", url);
  };

  let pending = 0;
  window.addEventListener(
    "scroll",
    () => {
      if (pending) return;
      pending = setTimeout(() => {
        pending = 0;
        savePlace();
      }, 250);
    },
    { passive: true },
  );

  // moved is true after the user scrolls. A late layout change (a web
  // font, the facets) can move the grid, so the page restores the place
  // again when the fonts and images are loaded, unless the user moved.
  let moved = false;
  for (const ev of ["wheel", "touchmove", "keydown", "mousedown"]) {
    window.addEventListener(ev, () => {
      moved = true;
    }, { passive: true, once: true });
  }

  // restorePlace scrolls the tile at=N to the top, under the header. If the
  // results have fewer tiles now, it goes to the last tile.
  const restorePlace = () => {
    const at = Number(new URLSearchParams(location.search).get("at"));
    if (!at || moved) return;
    let t = document.querySelector(`.tile[data-i="${at}"]`);
    if (!t) {
      const all = tiles();
      t = all.filter((x) => Number(x.dataset.i) <= at).pop() || all.pop();
    }
    if (t) {
      window.scrollTo(0, t.getBoundingClientRect().top + window.scrollY - topH());
    }
  };

  document.addEventListener("DOMContentLoaded", () => {
    setTop();
    // A page that opens with a card (?card=ID) gets a results entry under
    // the card entry, so that closing the card view goes to the results.
    const z0 = zoom();
    const id0 = cardParam();
    if (id0 && z0 && !z0.hidden && !(history.state && history.state.kfcard)) {
      history.replaceState(null, "", withCard(""));
      history.pushState({ kfcard: true, depth: 1 }, "", withCard(id0));
    }
    setSize(current());
    // A new query replaces the results and the slider in them. Set the new
    // slider to the current size.
    const main = document.querySelector("main");
    if (main && "MutationObserver" in window) {
      new MutationObserver(() => {
        const s = document.getElementById("size");
        if (s && !s.style.getPropertyValue("--fill")) setSize(current());
      }).observe(main, { childList: true, subtree: true });
    }
    restorePlace();
    document.fonts?.ready.then(restorePlace);
  });
  window.addEventListener("load", restorePlace);
})();
