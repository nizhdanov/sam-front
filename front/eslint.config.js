//  @ts-check
import { eslint } from '@siberiacancode/eslint';

export default eslint(
  {
    react: true,
    tailwind: {
      settings: {
        entryPoint: 'src/routes/styles.css'
      }
    },
    typescript: true,
    pnpm: false,
    ignores: ['src/routeTree.gen.ts', '.agents/*', 'generated/*', 'AGENTS.md']
  },
  {
    rules: {
      'import/no-cycle': 'off',
      'import/order': 'off',
      'sort-imports': 'off',
      'ts/array-type': 'off',
      'ts/require-await': 'off',
      'pnpm/json-enforce-catalog': 'off',
      'siberiacancode/function-component-definition': 'off'
    }
  },
  {
    files: ['tests/**/*.test.mjs'],
    rules: {
      'test/no-import-node-test': 'off'
    }
  }
);
