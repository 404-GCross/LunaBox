export type ModalScopeOptions = {
  /** The overlay host must be a sibling of the content it blocks. */
  host: HTMLElement;
  panel: HTMLElement;
  onDismiss: () => void;
};

type Entry = ModalScopeOptions & { release: () => void };
const stacks = new WeakMap<HTMLElement, Entry[]>();
const inertElements = new WeakMap<
  HTMLElement,
  { count: number; inert: boolean; hidden: string | null }
>();

function makeInert(element: HTMLElement) {
  let saved = inertElements.get(element);
  if (!saved) {
    saved = {
      count: 0,
      inert: element.inert,
      hidden: element.getAttribute("aria-hidden"),
    };
    inertElements.set(element, saved);
    element.inert = true;
    element.setAttribute("aria-hidden", "true");
  }
  saved.count++;
  return () => {
    if (--saved.count > 0)
      return;
    element.inert = saved.inert;
    if (saved.hidden === null)
      element.removeAttribute("aria-hidden");
    else element.setAttribute("aria-hidden", saved.hidden);
    inertElements.delete(element);
  };
}

function updateStack(host: HTMLElement, stack: Entry[]) {
  for (const entry of stack) entry.release();
  const top = stack.at(-1);
  if (!top)
    return;
  const releases: Array<() => void> = [];
  // Stop at the host's parent: titlebars outside this content region stay active.
  for (const sibling of host.parentElement?.children ?? []) {
    if (sibling !== host && sibling instanceof HTMLElement)
      releases.push(makeInert(sibling));
  }
  for (const child of host.children) {
    if (!child.contains(top.panel) && child instanceof HTMLElement)
      releases.push(makeInert(child));
  }
  top.release = () => {
    releases
      .splice(0)
      .reverse()
      .forEach(release => release());
  };
}

/** Mount content-scoped dismissal and inertness. Focus management belongs to the UI adapter. */
export function mountModalScope({
  host,
  panel,
  onDismiss,
}: ModalScopeOptions): () => void {
  if (!host.contains(panel))
    throw new Error("The modal panel must be inside its overlay host");
  const document = host.ownerDocument;
  const stack = stacks.get(host) ?? [];
  stacks.set(host, stack);
  const entry: Entry = { host, panel, onDismiss, release: () => {} };
  stack.push(entry);
  updateStack(host, stack);
  let startedOutside = false;
  const isTop = () => stack.at(-1) === entry;
  const isBackdrop = (event: Event) => {
    const target = event.composedPath()[0];
    return (
      target instanceof Node && host.contains(target) && !panel.contains(target)
    );
  };
  const onPointerDown = (event: PointerEvent) => {
    startedOutside = event.button === 0 && isTop() && isBackdrop(event);
  };
  const onClick = (event: MouseEvent) => {
    const dismiss
      = startedOutside && isTop() && isBackdrop(event) && !event.defaultPrevented;
    startedOutside = false;
    if (dismiss)
      onDismiss();
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (
      isTop()
      && event.key === "Escape"
      && !event.defaultPrevented
      && event.target instanceof Node
      && host.contains(event.target)
    ) {
      event.preventDefault();
      event.stopPropagation();
      onDismiss();
    }
  };
  const observer = new MutationObserver(() => {
    if (isTop())
      updateStack(host, stack);
  });
  observer.observe(host, { childList: true });
  if (host.parentElement)
    observer.observe(host.parentElement, { childList: true });
  document.addEventListener("pointerdown", onPointerDown, true);
  document.addEventListener("click", onClick);
  document.addEventListener("keydown", onKeyDown);
  let disposed = false;
  return () => {
    if (disposed)
      return;
    disposed = true;
    observer.disconnect();
    document.removeEventListener("pointerdown", onPointerDown, true);
    document.removeEventListener("click", onClick);
    document.removeEventListener("keydown", onKeyDown);
    entry.release();
    stack.splice(stack.indexOf(entry), 1);
    updateStack(host, stack);
    if (stack.length === 0)
      stacks.delete(host);
  };
}
