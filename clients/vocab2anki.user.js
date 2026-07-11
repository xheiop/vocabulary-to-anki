// ==UserScript==
// @name         vocab2anki
// @namespace    https://github.com/xheiop/vocab2anki
// @version      1.1.0
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
  // Option+A sends the current selection. e.code is used so the
  // physical key matches regardless of layout (Option+A types "å" on macOS).
  const HOTKEY_CODE = "KeyA";

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

  // True when the user is working in a text-entry context (input, textarea,
  // contenteditable). We stay out of the way there: no button, no hotkey.
  function inEditable() {
    const el = document.activeElement;
    if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.isContentEditable)) {
      return true;
    }
    const node = window.getSelection() && window.getSelection().anchorNode;
    const host = node && (node.nodeType === Node.TEXT_NODE ? node.parentElement : node);
    return !!(host && host.closest && host.closest("input, textarea, [contenteditable=''], [contenteditable=true]"));
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
      position: "fixed",
      left: `${Math.min(x + 10, window.innerWidth - 70)}px`,
      top: `${Math.min(y + 14, window.innerHeight - 36)}px`,
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
      if (e.button !== 0) return; // only plain left-click sends
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
    // Clear the selection so the highlight goes away once the word is sent.
    sel.removeAllRanges();

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
  // Design notes to minimize interference with normal page use:
  //   * only plain LEFT mouse-up shows the button — right-click (context menu,
  //     "Look Up", "Copy") and middle-click are never touched;
  //   * nothing happens inside inputs/textareas/contenteditable editors;
  //   * all listeners are passive observers except on our own button, so page
  //     handlers, drag & drop, and native selection toolbars work unchanged.

  document.addEventListener("mouseup", (e) => {
    if (e.button !== 0) return; // ignore right/middle button
    if (button && button.contains(e.target)) return; // our own button
    // Wait a tick: the browser finalizes the selection after mouseup.
    setTimeout(() => {
      const word = selectedWord();
      if (word && !inEditable()) {
        showButton(e.clientX, e.clientY);
      } else {
        hideButton();
      }
    }, 10);
  });

  // Any new press outside the button dismisses it (including right-click,
  // so it never overlaps a context menu).
  document.addEventListener("mousedown", (e) => {
    if (button && !button.contains(e.target)) hideButton();
  });

  // Keyboard deselection (Esc, arrow keys) removes the button too.
  document.addEventListener("selectionchange", () => {
    if (button) {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed) hideButton();
    }
  });

  // The button is viewport-fixed; hide it on scroll rather than letting it
  // drift away from the text it belongs to.
  document.addEventListener("scroll", () => hideButton(), { passive: true, capture: true });

  document.addEventListener("keydown", (e) => {
    // Exactly Alt (Option) + A: no Cmd/Ctrl combos, never inside text entry.
    if (!e.altKey || e.ctrlKey || e.metaKey || e.code !== HOTKEY_CODE) return;
    if (inEditable()) return;
    if (selectedWord()) {
      e.preventDefault();
      send();
    }
  });
})();
