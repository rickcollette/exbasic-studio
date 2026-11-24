import { describe, expect, it } from 'vitest';
import { renderManual, renderDocumentMarkdown, slugify, highlightMatches, chapterTitle } from './markdown';

describe('manual rendering', () => {
  it('renders headings with anchors, tables and code blocks', () => {
    const { html, headings } = renderManual('# Title\n\n## `ABS` function\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n```\nPRINT 1\n```\n\n## Reserved words\n## Reserved words\n');
    expect(headings.map(heading => heading.id)).toEqual(['title', 'abs-function', 'reserved-words', 'reserved-words-1']);
    expect(html).toContain('<h2 id="abs-function"><code>ABS</code> function</h2>');
    expect(html).toContain('<table>');
    expect(html).toContain('<pre><code>PRINT 1');
  });

  it('turns chapter links into in-manual navigation and neutralizes external ones', () => {
    const { html } = renderManual('See [Expressions](05-expressions.md#abs), [above](#title), [-l](19-compiler-reference.md#-l) and [site](https://example.com/x).');
    expect(html).toContain('data-chapter="05-expressions.md" data-anchor="abs"');
    expect(html).toContain('data-chapter="" data-anchor="title"');
    expect(html).toContain('data-anchor="-l"');
    // The external target is shown as hover text only, never as a link.
    expect(html).not.toMatch(/href="[^#"]/);
    expect(html).toContain('<span class="external-link" title="https://example.com/x">site</span>');
  });

  it('strips scripts, event handlers and raw HTML', () => {
    const { html } = renderManual('<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n[x](javascript:alert(1))\n\n<a href="https://evil" onclick="x()">y</a>\n\n**ok**');
    expect(html).not.toMatch(/<script|onerror|onclick|javascript:|<img|evil/i);
    expect(html).toContain('<strong>ok</strong>');
  });

  it('computes GitHub-style slugs', () => {
    expect(slugify('Reserved words')).toBe('reserved-words');
    expect(slugify('-l')).toBe('-l');
    expect(slugify('DIM, DATA & more!')).toBe('dim-data--more');
    expect(slugify('MID$ statement')).toBe('mid-statement');
  });

  it('highlights and clears search matches', () => {
    const root = document.createElement('div');
    root.innerHTML = renderManual('PRINT prints. Use print often.\n\n`PRINT`').html;
    expect(highlightMatches(root, 'print')).toBe(4);
    expect(root.querySelectorAll('mark.search-hit')).toHaveLength(4);
    expect(root.textContent).toContain('PRINT prints. Use print often.');
    expect(highlightMatches(root, '')).toBe(0);
    expect(root.querySelectorAll('mark')).toHaveLength(0);
    expect(highlightMatches(root, 'x')).toBe(0);
  });

  it('formats chapter titles', () => {
    expect(chapterTitle('05-expressions.md')).toBe('5. Expressions');
    expect(chapterTitle('A-keyword-reference.md')).toBe('Appendix A. Keyword Reference');
    expect(chapterTitle('README.md')).toBe('README');
  });

  it('renders safe workspace Markdown including Mermaid code fences', () => {
    const html = renderDocumentMarkdown('# Diagram\n\n```mermaid\ngraph TD; A-->B\n```\n\n![local](img/graph.png)\n\n![remote](https://example.com/tracker.png)\n\n<script>alert(1)</script>\n\n[site](https://example.com)');
    expect(html).toContain('language-mermaid');
    expect(html).toContain('data-workspace-image="img/graph.png"');
    expect(html).not.toContain('src=');
    expect(html).toContain('href="https://example.com"');
    expect(html).not.toMatch(/<script|alert\(1\)/i);
  });
});
