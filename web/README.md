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
| `npm run check:bundle-size`             | Fails if the initial JS payload exceeds 250 KiB gzipped                               |
| `npm run check:dist-secrets`            | Greps `dist/` for secret-shaped strings                                               |
| `npm run check:dist-msw`                | Fails if MSW (dev/test-only) leaked into a production build                           |

All of the above run in CI (`.github/workflows/ci.yml`'s `frontend` job)
on every PR.
