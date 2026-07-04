package anki

// Card front, back, and styling for the vocab2anki note type. The front shows
// the word and auto-plays its pronunciation; the back reveals the IPA,
// definition, examples, and the original context sentence.

const frontTemplate = `<div class="word">{{Word}}</div>
{{Audio}}`

const backTemplate = `{{FrontSide}}
<hr id="answer">
{{#IPA}}<div class="ipa">{{IPA}}</div>{{/IPA}}
<div class="definition">{{Definition}}</div>
{{#Examples}}<div class="examples">{{Examples}}</div>{{/Examples}}
{{#Context}}<div class="context"><span class="label">seen in</span>{{Context}}</div>{{/Context}}
{{#Source}}<div class="source">{{Source}}</div>{{/Source}}`

const cardCSS = `.card {
  font-family: -apple-system, "Helvetica Neue", Arial, sans-serif;
  font-size: 20px;
  text-align: center;
  color: #1a1a1a;
  background: #fafafa;
  padding: 24px;
}
.word { font-size: 34px; font-weight: 700; }
.ipa { color: #888; font-size: 18px; margin: 8px 0 16px; }
.definition { text-align: left; margin: 12px auto; max-width: 34em; line-height: 1.5; }
.examples { text-align: left; margin: 12px auto; max-width: 34em; }
.examples ul { padding-left: 1.2em; }
.examples li { margin: 6px 0; line-height: 1.45; }
.examples b, .context b { color: #c0392b; }
.context {
  text-align: left; margin: 16px auto; max-width: 34em;
  font-style: italic; color: #555; border-left: 3px solid #ddd; padding-left: 10px;
}
.context .label {
  display: block; font-style: normal; font-size: 12px;
  text-transform: uppercase; letter-spacing: 0.05em; color: #aaa; margin-bottom: 2px;
}
.source { margin-top: 16px; font-size: 12px; }
.source a { color: #3498db; text-decoration: none; }
hr#answer { margin: 18px 0; border: none; border-top: 1px solid #ddd; }

.nightMode.card { color: #eee; background: #1e1e1e; }
.nightMode .ipa { color: #999; }
.nightMode .context { color: #bbb; border-left-color: #444; }
.nightMode .examples b, .nightMode .context b { color: #ff6b6b; }
.nightMode hr#answer { border-top-color: #444; }`
