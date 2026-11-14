import { registerHooks } from 'node:module';

// Node запускает исходные TS и generated-импорты с алиасами и Windows-разделителями.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('@/')) {
      const root = specifier.startsWith('@/generated/') ? '../' : '../src/';
      return nextResolve(new URL(`${root}${specifier.slice(2)}.ts`, import.meta.url).href, context);
    }
    if (specifier.startsWith('.') && !context.parentURL?.includes('/node_modules/')) {
      const path = specifier.replaceAll('\\', '/');
      return nextResolve(/\.(?:[cm]?js|tsx?)$/.test(path) ? path : `${path}.ts`, context);
    }
    return nextResolve(specifier, context);
  }
});
