export function findScrollParent(
  element: HTMLElement | null,
): HTMLElement | null {
  let current = element;
  while (current) {
    const style = window.getComputedStyle(current);
    if (/auto|scroll|overlay/.test(style.overflowY)) {
      return current;
    }
    current = current.parentElement;
  }
  return null;
}

export function getScrollTop(element: HTMLElement | null) {
  return element?.scrollTop ?? 0;
}

export function scrollToTop(
  element: HTMLElement | null,
  behavior: ScrollBehavior = "smooth",
) {
  element?.scrollTo({ behavior, top: 0 });
}
