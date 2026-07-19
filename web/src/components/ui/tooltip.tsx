import * as React from 'react';
import { Tooltip as BaseTooltip } from '@base-ui/react/tooltip';

import { cn } from '@/lib/utils';

function TooltipProvider({
  delayDuration = 150,
  ...props
}: Omit<React.ComponentProps<typeof BaseTooltip.Provider>, 'delay'> & { delayDuration?: number }) {
  return <BaseTooltip.Provider data-slot="tooltip-provider" delay={delayDuration} {...props} />;
}

function Tooltip(props: React.ComponentProps<typeof BaseTooltip.Root>) {
  return <BaseTooltip.Root data-slot="tooltip" {...props} />;
}

function TooltipTrigger({
  asChild,
  children,
  ...props
}: React.ComponentProps<typeof BaseTooltip.Trigger> & { asChild?: boolean }) {
  return (
    <BaseTooltip.Trigger
      data-slot="tooltip-trigger"
      render={asChild && React.isValidElement(children) ? children as React.ReactElement<Record<string, unknown>> : undefined}
      {...props}
    >
      {asChild ? undefined : children}
    </BaseTooltip.Trigger>
  );
}

function TooltipContent({
  className,
  sideOffset = 4,
  children,
  ...props
}: React.ComponentProps<typeof BaseTooltip.Popup> & { sideOffset?: number }) {
  return (
    <BaseTooltip.Portal>
      <BaseTooltip.Positioner sideOffset={sideOffset} className="z-50">
        <BaseTooltip.Popup
          data-slot="tooltip-content"
          className={cn(
            'max-w-72 rounded-md border border-border bg-popover px-3 py-2 text-xs leading-4 text-popover-foreground shadow-md outline-none',
            'origin-[var(--transform-origin)] transition-[transform,opacity] data-[starting-style]:scale-95 data-[starting-style]:opacity-0 data-[ending-style]:scale-95 data-[ending-style]:opacity-0',
            className,
          )}
          {...props}
        >
          {children}
        </BaseTooltip.Popup>
      </BaseTooltip.Positioner>
    </BaseTooltip.Portal>
  );
}

export { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger };
