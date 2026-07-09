package anki

// Card templates for the vocab2anki Cloze note type (sentence mining). The front
// is a real sentence with the target word blanked out via Anki cloze deletion;
// the back reveals the word and supporting info (IPA, audio, definition, more
// examples, source).

const frontTemplate = `<div class="sentence">{{cloze:Text}}</div>`

const backTemplate = `<div class="sentence">{{cloze:Text}}</div>
<hr id="answer">
<div class="headword">{{Word}}{{#IPA}} <span class="ipa">{{IPA}}</span>{{/IPA}}</div>
{{Audio}}
{{#Definition}}<div class="definition">{{Definition}}</div>{{/Definition}}
{{#Examples}}<div class="examples">{{Examples}}</div>{{/Examples}}
{{#Source}}<div class="source">{{Source}}</div>{{/Source}}`

const cardCSS = `.card {
  font-family: -apple-system, "Helvetica Neue", Arial, sans-serif;
  font-size: 21px;
  text-align: center;
  color: #1a1a1a;
  background: #fafafa;
  padding: 24px;
}
.sentence { max-width: 34em; margin: 0 auto; line-height: 1.5; }
.cloze { font-weight: 700; color: #2d6cdf; }
.headword { font-size: 26px; font-weight: 700; margin: 6px 0 2px; }
.ipa { color: #888; font-size: 18px; font-weight: 400; }
.definition { text-align: left; margin: 12px auto; max-width: 34em; line-height: 1.5; font-size: 18px; }
.definition i { color: #888; }
.examples { text-align: left; margin: 12px auto; max-width: 34em; font-size: 17px; }
.examples ul { padding-left: 1.2em; }
.examples li { margin: 6px 0; line-height: 1.45; }
.examples b { color: #c0392b; }
.source { margin-top: 16px; font-size: 12px; }
.source a { color: #3498db; text-decoration: none; }
hr#answer { margin: 18px 0; border: none; border-top: 1px solid #ddd; }

.nightMode.card { color: #eee; background: #1e1e1e; }
.nightMode .cloze { color: #6ea8ff; }
.nightMode .ipa { color: #999; }
.nightMode .definition i { color: #999; }
.nightMode .examples b { color: #ff6b6b; }
.nightMode hr#answer { border-top-color: #444; }`
