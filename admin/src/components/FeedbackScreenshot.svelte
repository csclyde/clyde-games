<script>
 export let src;
 let dialog;
</script>

<button class="thumbnail" type="button" aria-label="Open feedback screenshot" on:click={() => dialog.showModal()}>
 <img {src} alt="Feedback screenshot" loading="lazy" />
</button>

<dialog bind:this={dialog} aria-label="Feedback screenshot">
 <div class="toolbar">
  <a href={src} target="_blank" rel="noopener noreferrer">Open full size</a>
  <form method="dialog"><button type="submit">Close</button></form>
 </div>
 <img class="full" {src} alt="Feedback screenshot" loading="lazy" />
</dialog>

<style>
 .thumbnail { display: block; margin: .45rem 0; width: min(162px, 100%); min-width: 0; max-width: 162px; padding: 0; overflow: hidden; cursor: zoom-in; background: transparent; border: 1px solid var(--line); border-radius: 5px; }
 .thumbnail img { display: block; width: 100%; height: 90px; object-fit: contain; }
 .thumbnail:focus-visible, .toolbar :is(a,button):focus-visible { outline: 2px solid var(--olive); outline-offset: 2px; }
 dialog { color: var(--text, #fff); background: #202428; padding: .75rem; border: 1px solid #585e63; border-radius: 8px; max-width: min(1100px, calc(100vw - 2rem)); max-height: 94vh; }
 dialog::backdrop { background: rgba(0, 0, 0, .76); }
 .toolbar { display: flex; justify-content: space-between; gap: 1rem; align-items: center; margin-bottom: .6rem; }
 .toolbar a { color: inherit; }
 .toolbar form { margin: 0; }
 .full { display: block; max-width: 100%; max-height: 78vh; object-fit: contain; margin: auto; }
</style>
