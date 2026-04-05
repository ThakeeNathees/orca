import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// Mock ReactFlow — it requires a DOM measurement API that jsdom doesn't have.
vi.mock("@xyflow/react", () => ({
  ReactFlowProvider: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ReactFlow: () => <div data-testid="react-flow" />,
  Background: () => null,
  useNodesState: () => [[], vi.fn(), vi.fn()],
  useEdgesState: () => [[], vi.fn(), vi.fn()],
  useReactFlow: () => ({ screenToFlowPosition: vi.fn() }),
  Position: { Top: "top", Bottom: "bottom", Left: "left", Right: "right" },
  Handle: () => null,
  BaseEdge: () => null,
  getSmoothStepPath: () => ["", 0, 0],
  useConnection: () => ({}),
  MarkerType: { ArrowClosed: "arrowclosed" },
}));

// Mock next/dynamic — just render nothing for dynamically imported components.
vi.mock("next/dynamic", () => ({
  default: () => () => <div data-testid="dynamic-stub" />,
}));

// Mock Monaco editor
vi.mock("@monaco-editor/react", () => ({
  default: () => <div data-testid="monaco-stub" />,
}));

import Home from "./page";

describe("Page smoke test", () => {
  it("renders without crashing", () => {
    const { container } = render(<Home />);
    expect(container).toBeTruthy();
  });

  it("renders the nav sidebar", () => {
    render(<Home />);
    // NavSidebar renders a <aside> element
    const sidebars = screen.getAllByRole("complementary");
    expect(sidebars.length).toBeGreaterThanOrEqual(1);
  });
});
