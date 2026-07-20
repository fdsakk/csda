import { useI18n, type Lang } from '@/lib/i18n';
import { cn } from '@/lib/utils';

const OPTIONS: { value: Lang; label: string }[] = [
  { value: 'en', label: 'EN' },
  { value: 'pl', label: 'PL' },
];

export function LanguageToggle() {
  const { lang, setLang } = useI18n();
  return (
    <div className="inline-flex flex-none rounded-full bg-muted p-0.5" role="group" aria-label="Language">
      {OPTIONS.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={lang === option.value}
          className={cn(
            'rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors',
            lang === option.value ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
          )}
          onClick={() => setLang(option.value)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
