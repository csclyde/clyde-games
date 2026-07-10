# Shared frontend code

Put Svelte components used by both apps in `components/` and shared styles in `styles/`.
Apps can import shared components directly from `../shared/components` and copy shared
static styles into their own deployment output during the build.
