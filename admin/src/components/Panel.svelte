<script>
 export let title = '';
 export let subtitle = '';
 export let inset = true;
 let className = '';
 export { className as class };
</script>

<section class={`panel ${className}`} class:inset {...$$restProps}>
 {#if title || subtitle || $$slots.header || $$slots.actions}
  <header class="panel-header">
   <div class="panel-heading"><slot name="header">{#if title}<h2>{title}</h2>{/if}{#if subtitle}<p>{subtitle}</p>{/if}</slot></div>
   {#if $$slots.actions}<div class="panel-actions"><slot name="actions" /></div>{/if}
  </header>
 {/if}
 <div class="panel-content"><slot /></div>
</section>

<style>
 .panel {box-sizing:border-box;min-width:0;max-width:1368px;background:var(--surface);border:1px solid var(--line);border-radius:var(--radius);padding:16px;color:var(--text);}
 .inset {width:calc(100% - 32px);margin:12px auto;}
 .panel-header {display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:12px;}
 .panel-heading,.panel-content {min-width:0;}
 .panel-content :global(p) {text-align:left;}
 h2 {font-size:1.15rem;line-height:1.3;margin:0;text-align:left;}
 p {font-size:.8rem;color:var(--text-muted);margin:4px 0 0;text-align:left;}
 .panel-actions {display:flex;align-items:center;flex-wrap:wrap;gap:8px;}
 @media(max-width:650px){.inset{width:calc(100% - 16px);margin:10px auto;}.panel{padding:12px;}.panel-header{flex-wrap:wrap;}}
</style>
