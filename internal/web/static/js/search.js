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

  const openZoom = (tile) => {
    const z = zoom();
    const src = tile.dataset.full;
    if (!z || !src) return;
    const img = z.querySelector("img");
    img.src = src;
    img.alt = tile.title;
    z.hidden = false;
    z.dataset.from = String(tiles().indexOf(tile));
  };

  const closeZoom = () => {
    const z = zoom();
    if (!z || z.hidden) return false;
    z.hidden = true;
    const t = tiles()[Number(z.dataset.from)];
    if (t) t.focus();
    return true;
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
      const q = new URLSearchParams(location.search).get("q") || "";
      location.href = `/random?q=${encodeURIComponent(q)}`;
    },
    "?": () => {
      location.href = "/syntax";
    },
    "+": () => setSize(current() + step),
    "=": () => setSize(current() + step),
    "-": () => setSize(current() - step),
    b: () => {
      const d = document.getElementById("builder");
      if (d) d.open = !d.open;
    },
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
      closeZoom();
      return;
    }
    const tile = e.target.closest(".tile");
    if (tile) openZoom(tile);
  });

  document.addEventListener("input", (e) => {
    if (e.target.id === "size") setSize(Number(e.target.value));
  });

  // The Fuzzy box sends the query form, so the server puts the box's state
  // in the query.
  document.addEventListener("change", (e) => {
    if (e.target.id === "fuzzy") e.target.form.requestSubmit();
  });

  document.addEventListener("DOMContentLoaded", () => {
    const n = load();
    if (n) setSize(n);
  });
})();
