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
    if (input) input.value = String(size);
    save(size);
  };

  const current = () => load() || 180;

  const $ = (id) => document.getElementById(id);
  const zoom = () => $("zoom");
  const input = () => $("q");

  // setSignals sets the search bar signals ($pos, $q). The query input
  // takes the kfset event and sets them in Datastar.
  const setSignals = (detail) => {
    window.dispatchEvent(new CustomEvent("kfset", { detail }));
  };

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
    if (history.state && history.state.kfcard) {
      history.back();
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
      z.hidden = false;
      if (id && cardParam() !== id && !fromHistory) {
        history.pushState({ kfcard: true }, "", withCard(id));
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
      placeInput();
    },
    // placeCaret puts the draft input in the gap between the chips that is
    // nearest to a click on the bar.
    placeCaret(evt) {
      const chips = Array.from(document.querySelectorAll("#chips .chip"));
      let pos = -1;
      for (const [i, c] of chips.entries()) {
        const r = c.getBoundingClientRect();
        const before = evt.clientY < r.top ||
          (evt.clientY <= r.bottom && evt.clientX < r.left + r.width / 2);
        if (before) {
          pos = i;
          break;
        }
      }
      setPos(pos);
      input()?.focus();
    },
    // pasted makes the pasted text into chips: it sends the bar, as Enter
    // does.
    pasted() {
      const q = input();
      if (!q || q.value.trim() === "") return;
      closeAC();
      $("bar")?.requestSubmit();
    },
  };

  // ---- The search bar --------------------------------------------------

  const chipCount = () => document.querySelectorAll("#chips .chip").length;

  const getPos = () => {
    const n = Number(input()?.dataset.pos ?? -1);
    return Number.isFinite(n) ? n : -1;
  };

  // setPos moves the draft input before the chip at pos (-1: the end).
  const setPos = (pos) => {
    const n = chipCount();
    const p = pos < 0 || pos >= n ? -1 : pos;
    setSignals({ pos: p });
    placeInput(p);
  };

  // placeInput sets the flex order of the input from $pos. Datastar does it
  // too; this covers the time before Datastar starts.
  const placeInput = (pos) => {
    const q = input();
    if (!q) return;
    const p = pos ?? getPos();
    q.style.order = p < 0 ? "100000" : String(2 * p - 1);
  };

  // selectAll moves the whole query into the input as text and selects it,
  // so the user can copy it. Enter makes it into chips again.
  const selectAll = () => {
    const q = input();
    const chips = Array.from(document.querySelectorAll("#chips .chip"))
      .map((c) => c.dataset.term);
    const p = getPos();
    const draft = q.value.trim();
    if (draft) chips.splice(p < 0 ? chips.length : p, 0, draft);
    const all = chips.join(" ");
    setSignals({ q: "", pos: -1 });
    q.value = all;
    q.dispatchEvent(new Event("input", { bubbles: true }));
    placeInput();
    q.select();
  };

  // caretKeys move the draft input across the chips with the arrow keys
  // when the caret is at the start or the end of the input.
  const caretKeys = (e) => {
    const q = input();
    if (e.target !== q) return false;
    const n = chipCount();
    const p = getPos() < 0 ? n : getPos();
    const atStart = q.selectionStart === 0 && q.selectionEnd === 0;
    const atEnd = q.selectionStart === q.value.length;
    if (e.key === "ArrowLeft" && atStart && p > 0 && q.value === "") {
      setPos(p - 1);
      return true;
    }
    if (e.key === "ArrowRight" && atEnd && p < n && q.value === "") {
      setPos(p + 1);
      return true;
    }
    return false;
  };

  // ---- Autocomplete ----------------------------------------------------

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
    s.toLowerCase().normalize("NFD").replace(/[̀-ͯ]/g, "")
      .replace(/æ/g, "ae");

  const opRe = /(!=|<=|>=|=|:|<|>)/;
  let ac = { items: [], at: -1, start: 0, end: 0, field: false };

  const closeAC = () => {
    const ul = $("ac");
    if (ul) {
      ul.hidden = true;
      ul.replaceChildren();
    }
    ac = { items: [], at: -1, start: 0, end: 0, field: false };
  };

  // suggest finds the completions of the word at the caret.
  const suggest = () => {
    const q = input();
    if (!q || document.activeElement !== q) return closeAC();
    const caret = q.selectionStart ?? q.value.length;
    const before = q.value.slice(0, caret);
    const m = before.match(/(^|[\s(])(-?)([^\s()]*)$/);
    if (!m) return closeAC();
    const word = m[3];
    const wordStart = caret - word.length;
    const fields = acFields();
    let items = [];
    let start = wordStart;
    let field = false;
    const op = word.match(opRe);
    if (op) {
      const name = word.slice(0, op.index).toLowerCase();
      const f = fields.find((x) => x.name === name || x.alias === name);
      if (!f || !f.values) return closeAC();
      let valueStart = op.index + op[0].length;
      if (f.list) {
        const comma = word.lastIndexOf(",");
        if (comma >= valueStart) valueStart = comma + 1;
      }
      let prefix = word.slice(valueStart);
      if (prefix.startsWith("-") && f.name === "sort") {
        prefix = prefix.slice(1);
        valueStart++;
      }
      const p = fold(prefix);
      const starts = f.values.filter((v) => fold(v).startsWith(p));
      const has = f.values.filter((v) => !fold(v).startsWith(p) && fold(v).includes(p));
      items = starts.concat(has).filter((v) => fold(v) !== p);
      start = wordStart + valueStart;
    } else {
      if (word.length === 0) return closeAC();
      const p = word.toLowerCase();
      // The operator words are not the start of a field name.
      if (["or", "and", "not"].includes(p)) return closeAC();
      for (const f of fields) {
        const label = f.name === "is" ? "is:" : `${f.name}:`;
        if (f.name.startsWith(p) || (f.alias && f.alias.startsWith(p) && f.alias !== p)) {
          items.push(label);
        }
      }
      field = true;
    }
    items = items.slice(0, 8);
    if (items.length === 0) return closeAC();
    // No item is chosen at first, so Enter still runs the query. Tab takes
    // the first item, and the arrow keys choose one.
    ac = { items, at: -1, start, end: caret, field };
    render();
  };

  const render = () => {
    const ul = $("ac");
    if (!ul) return;
    ul.replaceChildren(
      ...ac.items.map((v, i) => {
        const li = document.createElement("li");
        li.role = "option";
        li.textContent = v;
        li.dataset.i = String(i);
        li.setAttribute("aria-selected", String(i === ac.at));
        return li;
      }),
    );
    const q = input();
    ul.style.order = q.style.order;
    ul.hidden = false;
  };

  // accept puts the chosen completion in the input.
  const accept = (i) => {
    const q = input();
    const v = ac.items[i];
    if (!q || v == null) return;
    const field = ac.field;
    q.value = q.value.slice(0, ac.start) + v + q.value.slice(ac.end);
    const caret = ac.start + v.length;
    q.setSelectionRange(caret, caret);
    closeAC();
    q.dispatchEvent(new Event("input", { bubbles: true }));
    if (field) suggest();
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
    "?": () => {
      location.href = "/syntax";
    },
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
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "a") {
      stop();
      closeAC();
      selectAll();
      return;
    }
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
    }
    if (caretKeys(e)) stop();
  }, true);

  document.addEventListener("keydown", (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "Escape") {
      if (closeZoom()) return;
      if (typing(e.target)) e.target.blur();
      return;
    }
    if (typing(e.target)) return;
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

  document.addEventListener("click", (e) => {
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
    placeInput();
    const n = load();
    if (n) setSize(n);
    restorePlace();
    document.fonts?.ready.then(restorePlace);
  });
  window.addEventListener("load", restorePlace);
})();
