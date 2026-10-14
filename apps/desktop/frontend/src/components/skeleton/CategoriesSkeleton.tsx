const CATEGORY_SKELETON_IDS = Array.from(
  { length: 8 },
  (_, index) => `category-skeleton-${index + 1}`,
);

export function CategoriesSkeleton() {
  return (
    <div className="h-full w-full animate-pulse overflow-y-auto scrollbar-stable px-6 py-7 lg:px-8 lg:py-9">
      <div className="mb-4 flex items-center gap-3">
        <div className="h-11 w-11 rounded-2xl bg-brand-200 data-glass:bg-white/8 dark:bg-brand-800 data-glass:dark:bg-black/12" />
        <div className="h-8 w-28 rounded-lg bg-brand-200 data-glass:bg-white/8 dark:bg-brand-800 data-glass:dark:bg-black/12" />
      </div>
      <div className="mb-6 h-10 w-full rounded-xl bg-brand-200 data-glass:bg-white/8 dark:bg-brand-800 data-glass:dark:bg-black/12" />
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(11rem,100%),1fr))] justify-items-center gap-4 pt-2">
        {CATEGORY_SKELETON_IDS.map(skeletonID => (
          <div
            key={skeletonID}
            className="w-full min-w-[min(11rem,100%)] max-w-48"
          >
            <div className="aspect-[4/5] rounded-2xl bg-brand-200 data-glass:bg-white/8 dark:bg-brand-800 data-glass:dark:bg-black/12" />
            <div className="mx-auto mt-3 h-5 w-2/3 rounded-md bg-brand-200 data-glass:bg-white/8 dark:bg-brand-800 data-glass:dark:bg-black/12" />
          </div>
        ))}
      </div>
    </div>
  );
}
