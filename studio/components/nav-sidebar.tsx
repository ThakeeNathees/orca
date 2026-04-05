"use client";

import { Home } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";

export function NavSidebar() {
  const currentView = useStudioStore((s) => s.currentView);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const isActive = currentView === "dashboard";

  return (
    <aside className="flex w-[52px] shrink-0 flex-col items-center border-r border-border bg-sidebar py-3 gap-4">
      {/* Dashboard nav item */}
      <Tooltip>
        <TooltipTrigger
          render={(props) => (
            <button
              {...props}
              type="button"
              onClick={(e) => {
                props.onClick?.(e);
                goToDashboard();
              }}
              className={cn(
                "flex size-9 items-center justify-center rounded-md transition-colors",
                isActive
                  ? "bg-sidebar-accent text-foreground"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                props.className
              )}
              aria-label="Dashboard"
            >
              <Home className="size-[18px]" />
            </button>
          )}
        />
        <TooltipContent side="right" sideOffset={8}>
          Dashboard
        </TooltipContent>
      </Tooltip>
    </aside>
  );
}
