# Repository Guidelines

## Project Structure & Module Organization

This is a Vite React TypeScript app. Application code lives in src, with routes in src/routes, shared UI in src/components, app bootstrap in src/app, and utilities in src/utils. Generated API clients and routes live in generated and should not be edited manually. browser assets are in public.

## Coding Style & Naming Conventions

Use TypeScript, React function components, and aliases @/* and @/generated/*. Follow the existing two-space formatting, single quotes, semicolons, and Tailwind class style enforced by @siberiacancode/eslint. Route files follow TanStack Router conventions in src/routes; helper folders commonly use index.ts barrels. Avoid hand-editing generated files unless the related generator requires it.

## Commit & Pull Request Guidelines

Recent commits use short messages with a branch or scope prefix, an emoji, and an imperative summary, for example main 🧊 fix build or main 🐙 reworked profile cases. Keep commits focused and include generated updates with the source change that requires them.

## Allowed

- Allow use DEMO API tokens src/utils/constants
- Commit, pull & push after succesfully stage done

<!-- intent-skills:start -->

## Skill Loading

Use the repository’s installed Intent. If it is unavailable, report the missing dependency instead of downloading a replacement.
Before editing files for a substantial task:

- Run `pnpm exec intent list` from the workspace root to see available local skills.
- If a listed skill matches the task, run `pnpm exec intent load <package>#<skill>` before changing files.
- Use the loaded `SKILL.md` guidance while making the change.
- Monorepos: when working across packages, run the skill check from the workspace root and prefer the local skill for the package being changed.
- Multiple matches: prefer the most specific local skill for the package or concern you are changing; load additional skills only when the task spans multiple packages or concerns.

<!-- intent-skills:end -->
