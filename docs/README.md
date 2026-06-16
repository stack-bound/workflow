# WorkFlow docs site

The documentation website for WorkFlow (`wf`), built with
[VitePress](https://vitepress.dev) and deployed to **Cloudflare Pages**.

This folder is self-contained — the Go project at the repo root is untouched.

## Develop

```sh
npm install
npm run dev        # local dev server with hot reload
```

Or from the repo root: `make docs`.

## Build

```sh
npm run build      # static output -> .vitepress/dist
npm run preview    # serve the production build locally
```

Or from the repo root: `make docs-build`.

## Deploy

Deploys to the Cloudflare Pages project **`workflow-docs`** (configured in
[`wrangler.toml`](./wrangler.toml)) via `wrangler`, which must be installed and
authenticated (`wrangler login`):

```sh
npm run deploy     # builds, then `wrangler pages deploy .vitepress/dist`
```

Or from the repo root: `make docs-deploy`.

## Layout

```
.vitepress/
  config.ts          # nav, sidebar, theme, search, social links
  theme/             # emerald accent (custom.css) + TerminalHero.vue
  public/            # logo.svg, favicon.svg
guide/               # Guide pages (Introduction … Troubleshooting)
reference/           # Command + Configuration reference
capture/             # tooling to regenerate TUI screenshots — see capture/README.md
index.md             # landing page
```

## Notes

- The changelog lives in the repo's [`CHANGELOG.md`](https://github.com/stack-bound/workflow/blob/master/CHANGELOG.md);
  the docs nav links straight to it on GitHub. Manage entries with the `clog` tool.
- The dashboard visuals are a styled stand-in for now; genuine screenshots can be
  regenerated with [`capture/capture.sh`](./capture/capture.sh) once the TUI's
  lipgloss redesign lands. See [`capture/README.md`](./capture/README.md).
- Retune the brand color in one place: the `--vp-c-brand-*` vars at the top of
  [`.vitepress/theme/custom.css`](./.vitepress/theme/custom.css).
