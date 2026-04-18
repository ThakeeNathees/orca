"use client"

import * as React from "react"
import { cn } from "@/lib/utils"

interface SliderProps {
  className?: string
  value?: number[]
  defaultValue?: number[]
  min?: number
  max?: number
  step?: number
  onValueChange?: (value: number[]) => void
}

function Slider({
  className,
  value,
  defaultValue,
  min = 0,
  max = 100,
  step = 1,
  onValueChange,
}: SliderProps) {
  const current = value?.[0] ?? defaultValue?.[0] ?? min
  const pct = ((current - min) / (max - min)) * 100

  return (
    <div className={cn("relative flex w-full touch-none items-center select-none", className)}>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={current}
        onChange={(e) => onValueChange?.([Number(e.target.value)])}
        className="orca-slider h-1 w-full cursor-pointer appearance-none bg-muted outline-none"
        style={{
          // Two-segment track: filled portion uses the Linear accent
          // (violet), remainder uses the app's muted surface so the thumb
          // reads against a consistent background. Tokens sourced from
          // globals.css.
          background: `linear-gradient(to right, var(--accent) 0%, var(--accent) ${pct}%, var(--bg-secondary) ${pct}%, var(--bg-secondary) 100%)`,
        }}
      />
    </div>
  )
}

export { Slider }
