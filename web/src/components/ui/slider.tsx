import * as React from 'react';
import { Slider as BaseSlider } from '@base-ui/react/slider';

import { cn } from '@/lib/utils';

function Slider({ className, ...props }: BaseSlider.Root.Props<readonly number[]>) {
  return (
    <BaseSlider.Root
      data-slot="slider"
      className={cn('relative w-full touch-none select-none data-[disabled]:opacity-50', className)}
      {...props}
    >
      <BaseSlider.Control className="flex w-full items-center py-1.5">
        <BaseSlider.Track data-slot="slider-track" className="relative h-1.5 w-full grow rounded-full bg-muted">
          <BaseSlider.Indicator data-slot="slider-range" className="rounded-full bg-primary" />
          <BaseSlider.Thumb
            data-slot="slider-thumb"
            className="size-4 rounded-full border border-primary/60 bg-background shadow-sm outline-none transition-[color,box-shadow] hover:border-primary focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[disabled]:pointer-events-none"
          />
        </BaseSlider.Track>
      </BaseSlider.Control>
    </BaseSlider.Root>
  );
}

export { Slider };
