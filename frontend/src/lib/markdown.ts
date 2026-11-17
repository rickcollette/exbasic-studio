// Renders the bundled manual: Markdown to sanitized HTML with heading
// anchors, in-manual cross links and search highlighting.
import { Marked } from 'marked';
import DOMPurify from 'dompurify';

/** GitHub-style heading slug, as used by the manual's cross references. */
export function slugify(text: string): string {
  return text.trim().toLowerCase().replace(/[^\p{L}\p{N}\s_-]/gu, '').replace(/\s/g, '-');
}

/** Removes inline Markdown so a heading's slug matches its visible text. */
function plainText(markdown: string): string {
  return markdown.replace(/`([^`]*)`/g, '$1').replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/[*_]{1,3}([^*_]+)[*_]{1,3}/g, '$1');
}

export interface Heading { level: number; text: string; id: string }

export interface RenderedManual { html: string; headings: Heading[] }

const ALLOWED_TAGS = ['h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'ul', 'ol', 'li', 'pre', 'code', 'em', 'strong', 'del', 'blockquote', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a', 'br', 'hr', 'mark', 'span', 'kbd', 'sup', 'sub'];
const ALLOWED_ATTR = ['id', 'href', 'class', 'align', 'data-chapter', 'data-anchor', 'title'];

/**
 * Renders a chapter. Links to other chapters become buttons-in-disguise:
 * anchors carrying data-chapter/data-anchor that the viewer intercepts.
 * External links are kept as plain text targets and never navigated to.
 */
export function renderManual(markdown: string): RenderedManual {
  const headings: Heading[] = [];
  const used = new Map<string, number>();
  const marked = new Marked({ gfm: true, breaks: false });
  marked.use({
    renderer: {
      heading({ tokens, depth }) {
        const inner = this.parser.parseInline(tokens);
        const text = plainText(tokens.map(token => token.raw).join(''));
        let id = slugify(text);
        const seen = used.get(id) ?? 0;
        used.set(id, seen + 1);
        if (seen > 0) id = `${id}-${seen}`;
        headings.push({ level: depth, text, id });
        return `<h${depth} id="${id}">${inner}</h${depth}>\n`;
      },
      link({ href, tokens }) {
        const inner = this.parser.parseInline(tokens);
        const target = href ?? '';
        const local = /^([A-Za-z0-9._-]+\.md)?(?:#(.*))?$/.exec(target);
        if (local && (local[1] || local[2] !== undefined)) {
          const chapter = local[1] ?? '';
          const anchor = local[2] ?? '';
          return `<a href="#${encodeURIComponent(anchor)}" data-chapter="${encodeURIComponent(chapter)}" data-anchor="${encodeURIComponent(anchor)}">${inner}</a>`;
        }
        // An external reference: show where it points without making it
        // navigable from inside the application.
        const title = /^https?:\/\//i.test(target) ? ` title="${target.replace(/"/g, '&quot;')}"` : '';
        return `<span class="external-link"${title}>${inner}</span>`;
      },
      image() { return ''; },
      html() { return ''; },
    },
  });
  const raw = marked.parse(markdown, { async: false }) as string;
  // Only fragment links survive as URLs; the data attributes carry plain
  // chapter names and anchors, which the viewer validates before use.
  const html = DOMPurify.sanitize(raw, { ALLOWED_TAGS, ALLOWED_ATTR, ALLOW_DATA_ATTR: false, ALLOWED_URI_REGEXP: /^#/, ADD_URI_SAFE_ATTR: ['data-chapter', 'data-anchor', 'id', 'class', 'title'] });
  return { html, headings };
}

/** Wraps every occurrence of query in <mark> inside an element's text. */
export function highlightMatches(root: HTMLElement, query: string): number {
  const needle = query.trim().toLowerCase();
  for (const mark of Array.from(root.querySelectorAll('mark.search-hit'))) {
    mark.replaceWith(document.createTextNode(mark.textContent ?? ''));
  }
  root.normalize();
  if (needle.length < 2) return 0;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const nodes: Text[] = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) nodes.push(node as Text);
  let count = 0;
  for (const node of nodes) {
    const text = node.data;
    const lower = text.toLowerCase();
    let index = lower.indexOf(needle);
    if (index < 0) continue;
    const fragment = document.createDocumentFragment();
    let last = 0;
    while (index >= 0) {
      fragment.append(text.slice(last, index));
      const mark = document.createElement('mark');
      mark.className = 'search-hit';
      mark.textContent = text.slice(index, index + needle.length);
      fragment.append(mark);
      count++;
      last = index + needle.length;
      index = lower.indexOf(needle, last);
    }
    fragment.append(text.slice(last));
    node.replaceWith(fragment);
  }
  return count;
}

/** Chapter title for lists: "05-expressions.md" -> "5. Expressions". */
export function chapterTitle(file: string): string {
  const match = /^([0-9A-Z]+)-(.*)\.md$/.exec(file);
  if (!match) return file.replace(/\.md$/, '');
  const number = /^\d+$/.test(match[1]) ? String(Number(match[1])) : `Appendix ${match[1]}`;
  const words = match[2].split('-').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  return `${number}. ${words}`;
}
