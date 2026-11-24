<script>
 import {renderFeedbackMarkdown} from '../feedback-markdown.mjs';
 export let message='';
 export let discord=false;
 $: rendered=renderFeedbackMarkdown(message,{discord});
</script>
<div class="feedback-markdown">{@html rendered}</div>
<style>
 .feedback-markdown {min-width:0; overflow-wrap:anywhere; line-height:1.45;}
 .feedback-markdown :global(p) {text-align:left; margin:.25rem 0 .6rem; white-space:normal;}
 .feedback-markdown :global(p:last-child) {margin-bottom:0;}
 .feedback-markdown :global(h1),.feedback-markdown :global(h2),.feedback-markdown :global(h3) {font-size:1.05em; text-align:left; margin:.65rem 0 .3rem;}
 .feedback-markdown :global(pre) {overflow:auto; white-space:pre; max-width:100%; padding:.5rem; background:var(--surface);}
 .feedback-markdown :global(table) {display:block; max-width:100%; overflow:auto; text-align:left;}
 .feedback-markdown :global(.feedback-quote) {background:var(--paper-warm);border:1px solid var(--line);border-left:3px solid var(--line-strong);border-radius:5px;margin:.45rem 0;color:var(--text-soft);}
 .feedback-markdown :global(.feedback-quote > summary) {cursor:pointer; display:grid;grid-template-columns:.75rem minmax(0,1fr);align-items:start;gap:.35rem;padding:.4rem .55rem;list-style:none;}
 .feedback-markdown :global(.feedback-quote > summary::-webkit-details-marker) {display:none;}
 .feedback-markdown :global(.feedback-quote > summary::before) {content:"+";font-weight:700;}
 .feedback-markdown :global(.feedback-quote[open] > summary::before) {content:"−";}
 .feedback-markdown :global(.feedback-quote-preview) {display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;line-clamp:2;overflow:hidden;text-overflow:ellipsis;}
 .feedback-markdown :global(.feedback-quote[open] > summary > .feedback-quote-preview) {-webkit-line-clamp:1;line-clamp:1;}
 .feedback-markdown :global(.feedback-quote > blockquote) {margin:0;padding:.45rem .65rem .55rem;border-top:1px solid var(--line);}
</style>
