import { readdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

// Apicraft can emit Windows separators. Module specifiers must use forward slashes
// so the checked-in client also builds on Linux (including Docker and CI).
async function normalizeImports(directory) {
  let changed = 0;
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      changed += await normalizeImports(path);
      continue;
    }
    if (!entry.name.endsWith('.ts')) continue;

    const source = await readFile(path, 'utf8');
    const normalized = source.replace(
      /(\bfrom\s+)(['"])(\.[^'"\r\n]*)\2/g,
      (_, prefix, quote, specifier) =>
        `${prefix}${quote}${specifier.replace(/\\+/g, '/')}${quote}`
    );
    if (normalized !== source) {
      await writeFile(path, normalized);
      changed += 1;
    }
  }
  return changed;
}

const directory = fileURLToPath(new URL('../generated/api/', import.meta.url));
console.log(`Normalized API imports in ${await normalizeImports(directory)} files.`);
