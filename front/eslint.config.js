//  @ts-check
import { eslint } from '@siberiacancode/eslint'
import { tanstackConfig } from '@tanstack/eslint-config'

export default eslint(
  {
    react: true,
    tailwind: true,
    typescript: true,
    pnpm: true,
  },
  ...tanstackConfig,
  {
    rules: {
      'import/no-cycle': 'off',
      'import/order': 'off',
      'sort-imports': 'off',
      '@typescript-eslint/array-type': 'off',
      '@typescript-eslint/require-await': 'off',
      'pnpm/json-enforce-catalog': 'off',
    },
  },
  {
    ignores: ['eslint.config.js', 'prettier.config.js'],
  },
)
