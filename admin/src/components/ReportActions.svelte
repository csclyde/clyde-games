<script>
    export let label;
</script>

<div class="report-actions" role="group" aria-label={label}>
    <div class="primary-actions"><slot name="primary" /></div>
    {#if $$slots.secondary}
        <div class="secondary-actions"><slot name="secondary" /></div>
    {/if}
</div>

<style>
    .report-actions {
        display: grid;
        gap: 8px;
        width: 100%;
        min-width: 0;
    }

    .primary-actions {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 6px;
    }

    .secondary-actions {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
    }

    .primary-actions :global(button),
    .secondary-actions :global(button) {
        box-sizing: border-box;
        border: 1px solid var(--line-strong);
        border-radius: 5px;
        font-family: inherit;
        line-height: 1.2;
        cursor: pointer;
    }

    .primary-actions :global(button) {
        min-height: 44px;
        padding: 8px 12px;
        font-size: .9rem;
        font-weight: 700;
    }

    .secondary-actions :global(button) {
        min-height: 32px;
        min-width: 44px;
        padding: 5px 8px;
        background: var(--surface);
        color: var(--text);
        font-size: .75rem;
    }

    .primary-actions :global(.resolve-action) {
        color: white;
        background: #23633e;
        border-color: #23633e;
    }

    .primary-actions :global(.ignore-action) {
        color: white;
        background: #a03538;
        border-color: #a03538;
    }

    .primary-actions :global(.resolve-action:hover:not(:disabled)) { background: #194a2e; }
    .primary-actions :global(.ignore-action:hover:not(:disabled)) { background: #7f252a; }
    .secondary-actions :global(button:hover:not(:disabled)) { background: var(--paper-warm); }

    .report-actions :global(button:focus-visible) {
        outline: 2px solid var(--forest);
        outline-offset: 2px;
    }

    .report-actions :global(button:disabled) {
        cursor: default;
        opacity: .55;
    }

    @media (max-width: 820px), (pointer: coarse) {
        .primary-actions :global(button) { min-height: 48px; }
        .secondary-actions :global(button) { min-height: 44px; }
    }
</style>
