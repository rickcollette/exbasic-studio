import type { Extension } from '@codemirror/state';
import { json } from '@codemirror/lang-json';
import { markdown } from '@codemirror/lang-markdown';
import { javascript } from '@codemirror/lang-javascript';
import { html } from '@codemirror/lang-html';
import { css } from '@codemirror/lang-css';
import { xml } from '@codemirror/lang-xml';
import { yaml } from '@codemirror/lang-yaml';
import { python } from '@codemirror/lang-python';
import { cpp } from '@codemirror/lang-cpp';
import { rust } from '@codemirror/lang-rust';
import { java } from '@codemirror/lang-java';
import { sql } from '@codemirror/lang-sql';

const extension = (path: string) => path.toLowerCase().split('/').pop()?.split('.').pop() ?? '';

/** Image formats browsers can display directly in the normal image viewer. */
export function isImagePath(path: string): boolean {
  return /\.(png|jpe?g|gif|webp|bmp|svg|ico|avif)$/i.test(path);
}

export function imageMime(path: string): string {
  const ext = extension(path);
  return ({ jpg: 'image/jpeg', jpeg: 'image/jpeg', svg: 'image/svg+xml', ico: 'image/x-icon' } as Record<string, string>)[ext] ?? `image/${ext || 'png'}`;
}

export function isMarkdownPath(path: string): boolean { return /\.(md|markdown|mdown|mkd)$/i.test(path); }
export function isMermaidPath(path: string): boolean { return /\.(mmd|mermaid)$/i.test(path); }
export function isJsonPath(path: string): boolean { return /\.(json|jsonc|webmanifest|map)$/i.test(path); }
export function hasDocumentPreview(path: string): boolean { return isMarkdownPath(path) || isMermaidPath(path); }

/** Language support for the text formats commonly found in Studio projects. */
export function languageFor(path: string): Extension | null {
  const name = path.toLowerCase().split('/').pop() ?? path.toLowerCase();
  const ext = extension(path);
  if (isJsonPath(path)) return json();
  if (isMarkdownPath(path) || isMermaidPath(path)) return markdown();
  if (['js', 'mjs', 'cjs', 'jsx'].includes(ext)) return javascript({ jsx: ext === 'jsx' });
  if (['ts', 'mts', 'cts', 'tsx'].includes(ext)) return javascript({ typescript: true, jsx: ext === 'tsx' });
  if (['html', 'htm', 'svelte', 'vue'].includes(ext)) return html();
  if (['css', 'scss', 'less'].includes(ext)) return css();
  if (['xml', 'svg', 'xsl', 'xslt', 'plist', 'csproj'].includes(ext)) return xml();
  if (['yaml', 'yml'].includes(ext)) return yaml();
  if (['py', 'pyw'].includes(ext)) return python();
  if (['c', 'h', 'cc', 'cpp', 'cxx', 'hh', 'hpp', 'hxx'].includes(ext)) return cpp();
  if (ext === 'rs') return rust();
  if (['java', 'gradle'].includes(ext)) return java();
  if (['sql', 'ddl', 'dml'].includes(ext)) return sql();
  // Common files without an extension still remain editable as plain text.
  if (name === 'package.json' || name === 'tsconfig.json') return json();
  return null;
}

export function fileTypeLabel(path: string): string {
  if (isMermaidPath(path)) return 'Mermaid diagram';
  if (isMarkdownPath(path)) return 'Markdown';
  if (isJsonPath(path)) return 'JSON';
  if (isImagePath(path)) return 'Image';
  const ext = extension(path);
  return ext ? `${ext.toUpperCase()} file` : 'Text file';
}
