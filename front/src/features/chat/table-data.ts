interface HastNode {
  children?: HastNode[];
  tagName?: string;
  type: string;
  value?: string;
}

function text(node: HastNode): string {
  if (node.type === 'text') return node.value ?? '';
  if (node.tagName === 'br') return '\n';
  return (node.children ?? []).map(text).join('');
}

function collect(node: HastNode, tag: string, out: HastNode[] = []) {
  if (node.tagName === tag) out.push(node);
  else for (const child of node.children ?? []) collect(child, tag, out);
  return out;
}

export function extractTable(node: unknown) {
  if (!node || typeof node !== 'object') return undefined;
  const rows = collect(node as HastNode, 'tr').map((row) =>
    (row.children ?? [])
      .filter((cell) => cell.tagName === 'th' || cell.tagName === 'td')
      .map((cell) => text(cell).trim())
  );
  const [columns, ...body] = rows;
  if (!columns?.length) return undefined;
  return {
    columns,
    rows: body.map((row) => columns.map((_, index) => row[index] ?? ''))
  };
}
