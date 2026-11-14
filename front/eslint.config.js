//  @ts-check
import { eslint } from '@siberiacancode/eslint';

export default eslint(
  {
    react: true,
    tailwind: {
      settings: {
        entryPoint: 'src/styles.css'
      }
    },
    typescript: true,
    pnpm: false,
    ignores: ['src/routeTree.gen.ts', '.agents/*', 'generated/*']
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
  }
);
