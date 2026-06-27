import type { ReactNode } from "react";
import { BetterDropdownMenu } from "./BetterDropdownMenu";

export interface BetterDataTableColumnFilterOption {
  value: string;
  label: string;
  icon?: string;
  iconColor?: string;
  pillColor?: string;
}

export interface BetterDataTableColumnFilter {
  value: string;
  options: BetterDataTableColumnFilterOption[];
  onChange: (value: string) => void;
  title?: string;
  allLabel?: string;
  pill?: boolean;
  align?: "start" | "end";
}

export interface BetterDataTableColumn<T> {
  key: string;
  header: ReactNode;
  className?: string;
  headerClassName?: string;
  cellClassName?: string;
  filter?: BetterDataTableColumnFilter;
  render: (row: T, index: number) => ReactNode;
}

interface BetterDataTableProps<T> {
  rows: T[];
  columns: BetterDataTableColumn<T>[];
  rowKey: (row: T, index: number) => string;
  empty?: ReactNode;
  maxHeightClassName?: string;
  rowClassName?: (row: T, index: number) => string;
}

function FilterHeader({
  header,
  filter,
}: {
  header: ReactNode;
  filter: BetterDataTableColumnFilter;
}) {
  const isActive = filter.value !== "";
  const allLabel = filter.allLabel ?? "All";
  const items = [
    {
      key: "__all__",
      label: allLabel,
      icon: filter.value === "" ? "i-mdi-check" : "i-mdi-filter-variant",
      iconColor: filter.value === "" ? "text-success-500" : "text-brand-400",
      pill: filter.pill,
      onClick: () => filter.onChange(""),
    },
    ...filter.options.map(option => ({
      key: option.value,
      label: option.label,
      icon:
        filter.value === option.value
          ? "i-mdi-check"
          : (option.icon ?? "i-mdi-filter-variant"),
      iconColor:
        filter.value === option.value
          ? "text-success-500"
          : (option.iconColor ?? "text-brand-400"),
      pill: filter.pill,
      pillColor: option.pillColor,
      onClick: () => filter.onChange(option.value),
    })),
  ];

  return (
    <div className="flex w-full items-center gap-1.5">
      <div className="min-w-0 flex-1 truncate">{header}</div>
      <BetterDropdownMenu
        align={filter.align ?? "start"}
        menuWidth="min-w-[200px]"
        title={filter.title}
        items={items}
        trigger={(
          <button
            type="button"
            aria-label={filter.title ?? "Filter"}
            className={[
              "relative inline-flex h-6 w-6 shrink-0 items-center justify-center",
              "rounded-md transition-colors",
              "hover:bg-brand-100/70 dark:hover:bg-brand-700/50",
              isActive
                ? "text-success-600 dark:text-success-400"
                : "text-brand-500 dark:text-brand-400",
            ].join(" ")}
          >
            <div
              className={[
                "text-sm",
                isActive ? "i-mdi-filter" : "i-mdi-filter-outline",
              ].join(" ")}
            />
            {isActive && (
              <span className="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-success-500" />
            )}
          </button>
        )}
      />
    </div>
  );
}

export function BetterDataTable<T>({
  rows,
  columns,
  rowKey,
  empty,
  maxHeightClassName = "max-h-[400px]",
  rowClassName,
}: BetterDataTableProps<T>) {
  return (
    <div
      className={[
        maxHeightClassName,
        "overflow-auto rounded-lg border border-brand-200",
        "bg-white/80 dark:border-brand-700 dark:bg-brand-900/50",
        "data-glass:bg-white/10 data-glass:dark:bg-black/10",
      ].join(" ")}
    >
      {rows.length === 0 ? (
        <div className="p-8 text-center text-sm text-brand-400 dark:text-brand-500">
          {empty}
        </div>
      ) : (
        <table className="w-full table-fixed border-separate border-spacing-0">
          <thead className="sticky top-0 z-10 bg-brand-50/95 backdrop-blur dark:bg-brand-800/95 data-glass:bg-white/20 data-glass:dark:bg-black/30">
            <tr>
              {columns.map(column => (
                <th
                  key={column.key}
                  className={[
                    "border-b border-brand-200 px-3 py-2 text-left text-xs font-semibold",
                    "text-brand-600 dark:border-brand-700 dark:text-brand-300",
                    column.className,
                    column.headerClassName,
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  {column.filter ? (
                    <FilterHeader
                      header={column.header}
                      filter={column.filter}
                    />
                  ) : (
                    column.header
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-brand-100 dark:divide-brand-800">
            {rows.map((row, index) => (
              <tr
                key={rowKey(row, index)}
                className={[
                  "transition-colors hover:bg-brand-50/80 dark:hover:bg-brand-800/70",
                  rowClassName?.(row, index),
                ]
                  .filter(Boolean)
                  .join(" ")}
              >
                {columns.map(column => (
                  <td
                    key={column.key}
                    className={[
                      "px-3 py-2 align-middle text-sm text-brand-700 dark:text-brand-300",
                      column.className,
                      column.cellClassName,
                    ]
                      .filter(Boolean)
                      .join(" ")}
                  >
                    {column.render(row, index)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
