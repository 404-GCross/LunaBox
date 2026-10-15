import { BetterSwitch } from "./better/BetterSwitch";

interface SettingSwitchRowProps {
  id: string;
  label: string;
  hint: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function SettingSwitchRow({
  id,
  label,
  hint,
  checked,
  onCheckedChange,
  disabled = false,
}: SettingSwitchRowProps) {
  const textClass = disabled
    ? "text-brand-400 dark:text-brand-500"
    : "text-brand-700 dark:text-brand-300";

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-4">
        <div className="flex-1 space-y-2">
          <label
            htmlFor={id}
            className={`block cursor-pointer text-sm font-medium ${textClass}`}
          >
            {label}
          </label>
          <p className="text-xs text-brand-500 dark:text-brand-400">{hint}</p>
        </div>
        <BetterSwitch
          id={id}
          checked={checked}
          onCheckedChange={onCheckedChange}
          disabled={disabled}
        />
      </div>
    </div>
  );
}
