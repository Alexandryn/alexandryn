# Alexandryn — frontend

React + TypeScript + Tailwind, built with Vite. See
[`.claude/specs/`](../.claude/specs/) — `frontend-tooling.md`,
`frontend-design-tokens.md`, `frontend-component-primitives.md`,
`frontend-generated-covers.md`, `frontend-shell-and-routing.md`,
`frontend-accessibility.md` — and the governing
[`architecture-frontend.md`](../.claude/specs/architecture-frontend.md)
for what this app actually is and why each tool below was chosen.

## Commands

| Command                                 | Does                                                                                  |
| --------------------------------------- | ------------------------------------------------------------------------------------- |
| `npm run dev`                           | Vite dev server with HMR                                                              |
| `npm run build`                         | Typecheck (`tsc -b`) + production build → `dist/`                                     |
| `npm run lint`                          | ESLint (`typescript-eslint`, `jsx-a11y`, `react-hooks`, `react-refresh`, `storybook`) |
| `npm run format` / `format:check`       | Prettier                                                                              |
| `npm test`                              | Vitest + React Testing Library                                                        |
| `npm run storybook` / `build-storybook` | Component preview environment                                                         |
| `npx playwright test`                   | E2E + real-browser `axe-core` + the generated-cover benchmark (three projects)        |
| `npm run tokens:generate`               | Regenerate `theme.css` / `tokens.css` / `breakpoints.ts` from `.design-reference/`    |
| `npm run tokens:check-contrast`         | WCAG AA contrast over every meaningful token pair                                     |
| `npm run mocks:gen-fixtures`            | Regenerate the contract-covered MSW fixtures from `api/openapi.yaml`                  |
| `npm run check:token-styling`           | Fails on a raw hex/px value outside the token set                                     |
| `npm run check:a11y-tabindex`           | Fails on a positive `tabindex`                                                        |
| `npm run check:a11y-hidden-text`        | Fails on `display:none` / hand-rolled clip-rect on text                               |
| `npm run check:bundle-size`             | Fails if the initial JS payload exceeds 250 KiB gzipped (currently ~104 KiB)          |
| `npm run check:dist-secrets`            | Greps `dist/` for secret-shaped strings                                               |
| `npm run check:dist-msw`                | Fails if MSW (dev/test-only) leaked into a production build                           |

All of the above run in CI (`.github/workflows/ci.yml`'s `frontend` job)
on every PR, plus `npm audit --audit-level=high` and a staleness gate on
the generated token and fixture files.
