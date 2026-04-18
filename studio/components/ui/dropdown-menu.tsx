"use client";

import {
  useState,
  useRef,
  useEffect,
  useCallback,
  createContext,
  useContext,
  type ReactNode,
} from "react";
import { cn } from "@/lib/utils";
import { useEscapeKey } from "@/lib/hooks/use-escape-key";

interface DropdownMenuContextValue {
  open: boolean;
  setOpen: (open: boolean) => void;
}

const DropdownMenuContext = createContext<DropdownMenuContextValue>({
  open: false,
  setOpen: () => {},
});

export function DropdownMenu({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const closeMenu = useCallback(() => setOpen(false), []);
  useEscapeKey(closeMenu, open);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <DropdownMenuContext.Provider value={{ open, setOpen }}>
      <div ref={ref} className="relative shrink-0">
        {children}
      </div>
    </DropdownMenuContext.Provider>
  );
}

export function DropdownMenuTrigger({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const { open, setOpen } = useContext(DropdownMenuContext);

  const handleClick = useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      e.stopPropagation();
      setOpen(!open);
      props.onClick?.(e);
    },
    [open, setOpen, props]
  );

  return (
    <button type="button" {...props} className={className} onClick={handleClick}>
      {children}
    </button>
  );
}

export function DropdownMenuContent({
  children,
  align = "end",
  className,
}: {
  children: ReactNode;
  align?: "start" | "end";
  className?: string;
}) {
  const { open } = useContext(DropdownMenuContext);

  if (!open) return null;

  return (
    <div
      className={cn(
        "absolute top-full z-50 mt-1 w-40 overflow-hidden rounded-md border border-border bg-card shadow-lg",
        align === "end" ? "right-0" : "left-0",
        className
      )}
    >
      {children}
    </div>
  );
}

export function DropdownMenuItem({
  children,
  onClick,
  className,
  variant = "default",
}: {
  children: ReactNode;
  onClick?: () => void;
  className?: string;
  variant?: "default" | "destructive";
}) {
  const { setOpen } = useContext(DropdownMenuContext);

  return (
    <button
      type="button"
      className={cn(
        "flex w-full items-center gap-2 px-3 py-2 text-sm transition-colors hover:bg-accent cursor-pointer",
        variant === "destructive" ? "text-destructive" : "text-foreground",
        className
      )}
      onClick={(e) => {
        e.stopPropagation();
        setOpen(false);
        onClick?.();
      }}
    >
      {children}
    </button>
  );
}
