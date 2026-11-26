// The zoom view, the card size, and the keyboard shortcuts of the search
// page. Datastar loads the batches. This file only reacts to the user.
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

  const zoom = () => document.getElementById("zoom");

  // The server sends the card view (Datastar). The page only remembers the
  // tile that opened it, and closes it.
  let opener = null;

  const closeZoom = () => {
    const z = zoom();
    if (!z || z.hidden) return false;
    z.hidden = true;
    if (opener && document.contains(opener)) opener.focus();
    return true;
  };

  // showPrinting shows the image and the flavor text of one printing.
  const showPrinting = (btn) => {
    const z = zoom();
    const img = document.getElementById("zoom-img");
    if (img && btn.dataset.img) img.src = btn.dataset.img;
    const fl = document.getElementById("zoom-flavor");
    if (fl) fl.textContent = btn.dataset.flavor || "";
    for (const b of z.querySelectorAll(".printing")) {
      b.classList.toggle("active", b === btn);
    }
  };

  // The live search calls these from the server's scripts.
  window.kfsearch = {
    opened() {
      const z = zoom();
      if (!z) return;
      z.hidden = false;
      z.querySelector(".zoom-close")?.focus({ preventScroll: true });
    },
    // update puts a new query in the URL and the title. It drops the place
    // in the results (at=N), because the results are new.
    update({ url, title, fuzzy }) {
      const now = new URL(location.href);
      const next = new URL(url, location.href);
      if (now.search !== next.search) {
        history.replaceState(history.state, "", next);
        window.scrollTo(0, 0);
      }
      if (title) document.title = title;
      const box = document.getElementById("fuzzy");
      if (box) box.checked = fuzzy;
    },
  };

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
      document.getElementById("random")?.click();
    },
    "?": () => {
      location.href = "/syntax";
    },
    "+": () => setSize(current() + step),
    "=": () => setSize(current() + step),
    "-": () => setSize(current() - step),
  };

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
      const q = document.getElementById("q");
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
  });

  // On a page without live search (the syntax page), the Fuzzy box sends
  // the query form, so the server puts the box's state in the query.
  document.addEventListener("change", (e) => {
    if (e.target.id === "fuzzy" && !document.getElementById("results")) {
      e.target.form.requestSubmit();
    }
  });

  // The place in the results. The page keeps the index of the top tile in
  // the URL (at=N), so a reload or a back navigation comes back to it. The
  // server renders the tiles up to that index. The parameter is not part of
  // the query text.
  if ("scrollRestoration" in history) history.scrollRestoration = "manual";

  const topTile = () => {
    for (const t of document.querySelectorAll(".tile")) {
      if (t.getBoundingClientRect().bottom > 0) return t;
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

  const restorePlace = () => {
    const at = new URLSearchParams(location.search).get("at");
    if (!at || moved) return;
    const t = document.querySelector(`.tile[data-i="${CSS.escape(at)}"]`);
    if (t) window.scrollTo(0, t.getBoundingClientRect().top + window.scrollY);
  };

  document.addEventListener("DOMContentLoaded", () => {
    const n = load();
    if (n) setSize(n);
    restorePlace();
    document.fonts?.ready.then(restorePlace);
  });
  window.addEventListener("load", restorePlace);
})();
