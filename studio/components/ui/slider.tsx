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
        className="orca-slider h-1 w-full cursor-pointer appearance-none rounded-full bg-muted outline-none"
        style={{
          background: `linear-gradient(to right, oklch(0.93 0 0) 0%, oklch(0.93 0 0) ${pct}%, oklch(0.22 0 0) ${pct}%, oklch(0.22 0 0) 100%)`,
        }}
      />
    </div>
  )
}

export { Slider }
