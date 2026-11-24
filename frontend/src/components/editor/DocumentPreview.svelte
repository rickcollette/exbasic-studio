<script lang="ts">
  import { tick } from 'svelte';
  import DOMPurify from 'dompurify';
  import { renderDocumentMarkdown } from '../../lib/markdown';
  import { isMermaidPath } from '../../lib/fileTypes';
  import { imageMime, isImagePath } from '../../lib/fileTypes';
  import { native } from '../../lib/native';
  import { parentOf } from '../../lib/stores/workspace.svelte';

  let { path, text }: { path: string; text: string } = $props();
  let host = $state<HTMLElement>();
  let error = $state('');
  let sequence = 0;

  function workspacePath(documentPath: string, source: string): string | null {
    if (!source || /^(?:[a-z][a-z0-9+.-]*:|\/|#)/i.test(source)) return null;
    let decoded: string;
    try { decoded = decodeURIComponent(source.split(/[?#]/, 1)[0]); } catch { return null; }
    const parts = parentOf(documentPath).split('/').filter(Boolean);
    for (const part of decoded.replaceAll('\\', '/').split('/')) {
      if (!part || part === '.') continue;
      if (part === '..') { if (!parts.length) return null; parts.pop(); } else parts.push(part);
    }
    return parts.join('/');
  }

  $effect(() => {
    const source = text;
    const file = path;
    const target = host;
    if (!target) return;
    const current = ++sequence;
    error = '';
    if (isMermaidPath(file)) {
      const diagram = document.createElement('div');
      diagram.className = 'mermaid';
      diagram.textContent = source;
      target.replaceChildren(diagram);
    } else {
      target.innerHTML = renderDocumentMarkdown(source);
      for (const image of Array.from(target.querySelectorAll<HTMLImageElement>('img[data-workspace-image]'))) {
        const imagePath = workspacePath(file, image.dataset.workspaceImage ?? '');
        if (!imagePath || !isImagePath(imagePath)) { image.replaceWith(document.createTextNode(image.alt ? `[Image: ${image.alt}]` : '[Image]')); continue; }
        void native().studio.ReadBinaryFile(imagePath).then(data => {
          if (current === sequence) image.src = `data:${imageMime(imagePath)};base64,${data}`;
        }).catch(() => { if (current === sequence) image.replaceWith(document.createTextNode(image.alt ? `[Missing image: ${image.alt}]` : `[Missing image: ${imagePath}]`)); });
      }
    }
    void tick().then(async () => {
      if (current !== sequence) return;
      const nodes = Array.from(target.querySelectorAll<HTMLElement>('.mermaid, pre code.language-mermaid'));
      if (!nodes.length) return;
      for (const node of nodes) {
        if (node.matches('pre code')) {
          const diagram = document.createElement('div');
          diagram.className = 'mermaid';
          diagram.textContent = node.textContent ?? '';
          node.parentElement?.replaceWith(diagram);
        }
      }
      try {
        const mermaid = (await import('mermaid')).default;
        const theme = document.querySelector('.workbench')?.classList.contains('theme-light') ? 'default' : 'dark';
        mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme, suppressErrorRendering: true });
        await mermaid.run({ nodes: Array.from(target.querySelectorAll<HTMLElement>('.mermaid')) });
        // Mermaid emits SVG. Sanitize once more before it remains in the document.
        for (const svg of Array.from(target.querySelectorAll('svg'))) {
          const clean = DOMPurify.sanitize(svg.outerHTML, { USE_PROFILES: { svg: true, svgFilters: true } });
          svg.outerHTML = clean;
        }
      } catch (reason) {
        if (current === sequence) error = reason instanceof Error ? reason.message : String(reason);
      }
    });
  });
</script>

<div class="document-preview" class:diagram={isMermaidPath(path)}>
  {#if error}<div class="preview-error" role="alert"><i aria-hidden="true" class="codicon codicon-error"></i><span>{error}</span></div>{/if}
  <article class="markdown-body" aria-label={`${path} preview`} bind:this={host}></article>
</div>
