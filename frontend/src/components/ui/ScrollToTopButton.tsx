interface ScrollToTopButtonProps {
  label: string;
  onClick: () => void;
  visible: boolean;
}

export function ScrollToTopButton({
  label,
  onClick,
  visible,
}: ScrollToTopButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      className={`glass-btn-neutral fixed bottom-8 right-8 z-40 flex h-11 w-11 items-center justify-center rounded-full border border-brand-200 bg-white/92 text-brand-700 shadow-lg shadow-black/10 transition-all duration-200 hover:-translate-y-0.5 hover:bg-brand-100 focus:outline-none focus:ring-4 focus:ring-brand-300/50 dark:border-brand-700 dark:bg-brand-800/92 dark:text-brand-200 dark:hover:bg-brand-700 dark:focus:ring-brand-600/40 data-glass:bg-white/35 data-glass:dark:bg-black/35 ${
        visible
          ? "translate-y-0 opacity-100"
          : "pointer-events-none translate-y-2 opacity-0"
      }`}
    >
      <div className="i-mdi-arrow-up text-xl" />
    </button>
  );
}
