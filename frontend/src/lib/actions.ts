// Small shared UI behaviours.

/**
 * Keyboard navigation for a tree or list whose rows are focusable elements
 * marked with data-row. Arrow keys move, Home/End jump, and the optional
 * handlers receive ArrowLeft/ArrowRight (collapse/expand).
 */
export function rowNavigation(node: HTMLElement, options: { onLeft?: (row: HTMLElement) => void; onRight?: (row: HTMLElement) => void } = {}) {
  let current = options;
  function rows(): HTMLElement[] { return Array.from(node.querySelectorAll<HTMLElement>('[data-row]')); }
  function keydown(event: KeyboardEvent) {
    const target = (event.target as HTMLElement | null)?.closest<HTMLElement>('[data-row]');
    if (!target || (event.target as HTMLElement).tagName === 'INPUT') return;
    const list = rows();
    const index = list.indexOf(target);
    let next = -1;
    switch (event.key) {
      case 'ArrowDown': next = Math.min(list.length - 1, index + 1); break;
      case 'ArrowUp': next = Math.max(0, index - 1); break;
      case 'Home': next = 0; break;
      case 'End': next = list.length - 1; break;
      case 'ArrowLeft': if (current.onLeft) { current.onLeft(target); event.preventDefault(); } return;
      case 'ArrowRight': if (current.onRight) { current.onRight(target); event.preventDefault(); } return;
      default: return;
    }
    event.preventDefault();
    list[next]?.focus();
  }
  // Roving tabindex: only one row is in the tab order.
  function sync() {
    const list = rows();
    const focused = list.find(row => row === document.activeElement) ?? list.find(row => row.getAttribute('aria-selected') === 'true') ?? list[0];
    for (const row of list) row.tabIndex = row === focused ? 0 : -1;
  }
  const observer = new MutationObserver(sync);
  observer.observe(node, { childList: true, subtree: true, attributes: true, attributeFilter: ['aria-selected'] });
  node.addEventListener('keydown', keydown);
  node.addEventListener('focusin', sync);
  sync();
  return {
    update(value: typeof options) { current = value; },
    destroy() { observer.disconnect(); node.removeEventListener('keydown', keydown); node.removeEventListener('focusin', sync); },
  };
}

/** Keeps Tab and Shift+Tab inside a modal element. */
export function trapFocus(node: HTMLElement) {
  function focusable(): HTMLElement[] {
    return Array.from(node.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter(element => !element.hasAttribute('disabled') && element.offsetParent !== null || element === document.activeElement);
  }
  function keydown(event: KeyboardEvent) {
    if (event.key !== 'Tab') return;
    const items = focusable();
    if (!items.length) { event.preventDefault(); return; }
    const first = items[0], last = items[items.length - 1];
    if (event.shiftKey && (document.activeElement === first || !node.contains(document.activeElement))) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }
  node.addEventListener('keydown', keydown);
  return { destroy() { node.removeEventListener('keydown', keydown); } };
}

export function hex(value: number, digits: number): string {
  return value.toString(16).toUpperCase().padStart(digits, '0');
}
