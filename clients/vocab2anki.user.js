// ==UserScript==
// @name         vocab2anki
// @namespace    https://github.com/xheiop/vocab2anki
// @version      1.0.0
// @description  Select an English word on any page and send it (with its sentence) to the local vocab2anki service, which enriches it and adds it to Anki.
// @match        *://*/*
// @grant        GM_xmlhttpRequest
// @connect      127.0.0.1
// @connect      localhost
// @run-at       document-idle
// ==/UserScript==

(function () {
  "use strict";

  const ENDPOINT = "http://127.0.0.1:8766/add";
  const HOTKEY = { alt: true, key: "a" }; // Alt+A sends the current selection

  // --- selection helpers ----------------------------------------------------

  // Returns the selected text trimmed, or "" if nothing/too long to be a word.
  function selectedWord() {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed) return "";
    const text = sel.toString().trim();
    // Accept a single word or short phrase; ignore paragraph selections.
    if (!text || text.length > 60 || text.split(/\s+/).length > 4) return "";
    return text;
  }

  // Best-effort: the sentence containing the selection, for context.
  function surroundingSentence(sel) {
    try {
      const node = sel.anchorNode;
      const container = node && node.nodeType === Node.TEXT_NODE ? node.parentElement : node;
      const block = container ? container.closest("p, li, td, div, span, article, section, body") : null;
      const full = (block ? block.innerText : sel.toString()) || "";
      const word = sel.toString().trim();
      const idx = full.indexOf(word);
      if (idx < 0) return "";
      // Expand to sentence boundaries around the match.
      let start = full.lastIndexOf(".", idx);
      start = Math.max(start, full.lastIndexOf("!", idx), full.lastIndexOf("?", idx), full.lastIndexOf("\n", idx));
      let end = full.length;
      for (const p of [".", "!", "?", "\n"]) {
        const e = full.indexOf(p, idx + word.length);
        if (e >= 0 && e < end) end = e;
      }
      return full.slice(start + 1, end + 1).trim().replace(/\s+/g, " ").slice(0, 400);
    } catch (e) {
      return "";
    }
  }

  // --- UI -------------------------------------------------------------------

  let button = null;

  function showButton(x, y) {
    hideButton();
    button = document.createElement("div");
    button.textContent = "+ Anki";
    Object.assign(button.style, {
      position: "absolute",
      left: `${x}px`,
      top: `${y}px`,
      zIndex: 2147483647,
      background: "#2d6cdf",
      color: "#fff",
      font: "12px/1 -apple-system, sans-serif",
      padding: "6px 9px",
      borderRadius: "6px",
      cursor: "pointer",
      boxShadow: "0 2px 8px rgba(0,0,0,.25)",
      userSelect: "none",
    });
    button.addEventListener("mousedown", (e) => {
      e.preventDefault();
      e.stopPropagation();
      send();
    });
    document.body.appendChild(button);
  }

  function hideButton() {
    if (button) {
      button.remove();
      button = null;
    }
  }

  function toast(message, ok) {
    const t = document.createElement("div");
    t.textContent = message;
    Object.assign(t.style, {
      position: "fixed",
      right: "16px",
      bottom: "16px",
      zIndex: 2147483647,
      background: ok ? "#1e824c" : "#c0392b",
      color: "#fff",
      font: "13px -apple-system, sans-serif",
      padding: "10px 14px",
      borderRadius: "8px",
      boxShadow: "0 2px 10px rgba(0,0,0,.3)",
      opacity: "0",
      transition: "opacity .15s",
    });
    document.body.appendChild(t);
    requestAnimationFrame(() => (t.style.opacity = "1"));
    setTimeout(() => {
      t.style.opacity = "0";
      setTimeout(() => t.remove(), 200);
    }, 2200);
  }

  // --- send -----------------------------------------------------------------

  function send() {
    const sel = window.getSelection();
    const word = selectedWord();
    if (!word) {
      toast("Select a word first", false);
      return;
    }
    const context = surroundingSentence(sel);
    hideButton();

    GM_xmlhttpRequest({
      method: "POST",
      url: ENDPOINT,
      headers: { "Content-Type": "application/json" },
      data: JSON.stringify({ word, context, source: location.href }),
      timeout: 5000,
      onload: (res) => {
        if (res.status >= 200 && res.status < 300) {
          toast(`Queued "${word}" → Anki`, true);
        } else {
          toast(`Service error (${res.status})`, false);
        }
      },
      onerror: () => toast("vocab2anki not running on :8766", false),
      ontimeout: () => toast("vocab2anki timed out", false),
    });
  }

  // --- events ---------------------------------------------------------------

  document.addEventListener("mouseup", (e) => {
    // Ignore clicks on our own button.
    if (button && e.target === button) return;
    setTimeout(() => {
      const word = selectedWord();
      if (word) {
        showButton(e.pageX + 8, e.pageY + 8);
      } else {
        hideButton();
      }
    }, 10);
  });

  document.addEventListener("mousedown", (e) => {
    if (button && e.target !== button) hideButton();
  });

  document.addEventListener("keydown", (e) => {
    if (e.altKey === HOTKEY.alt && e.key.toLowerCase() === HOTKEY.key) {
      if (selectedWord()) {
        e.preventDefault();
        send();
      }
    }
  });
})();
