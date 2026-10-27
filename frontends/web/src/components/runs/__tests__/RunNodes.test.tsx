import { create } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RunNodeExecutionSchema } from "@/gen/reliant/v1/run_pb";
import { RunNodeList, RunNodes } from "../RunNodes";

const query = vi.hoisted(() => ({ isLoading: false, isError: false, data: undefined as unknown, refetch: vi.fn() }));
vi.mock("@/hooks/run-service-hooks_gen", () => ({ useGetRun: () => query }));

describe("run node diagnostics", () => {
 it("shows failed command details and keeps repeated attempts separate", () => {
  render(<RunNodeList truncated nodes={[
   create(RunNodeExecutionSchema, { id: "a", nodeId: "gate", workflowId: "root", status: "failed", attempt: 1, durationMs: 25n, exitCode: 2, error: "compile failed", stdoutTail: "building", stderrTail: "token=[REDACTED]" }),
   create(RunNodeExecutionSchema, { id: "b", nodeId: "review", status: "skipped" }),
   create(RunNodeExecutionSchema, { id: "c", nodeId: "gate", status: "completed", attempt: 2 }),
  ]} />);
  expect(screen.getByText("compile failed")).toBeInTheDocument();
  expect(screen.getByText("25 ms")).toBeInTheDocument();
  expect(screen.getByText("exit 2")).toBeInTheDocument();
  expect(screen.getByText("building")).toBeInTheDocument();
  expect(screen.getByText("token=[REDACTED]")).toBeInTheDocument();
  expect(screen.getByText("skipped")).toBeInTheDocument();
  expect(screen.getAllByText("gate")).toHaveLength(2);
  expect(screen.getByText(/Older node attempts omitted/)).toBeInTheDocument();
 });
 it("has an honest empty state", () => {
  render(<RunNodeList truncated={false} nodes={[]} />);
  expect(screen.getByText("No recorded node executions yet.")).toBeInTheDocument();
 });
 it("surfaces a read failure instead of calling it an empty run", () => {
  query.isError = true;
  render(<RunNodes workflowId="root" />);
  expect(screen.getByRole("alert")).toHaveTextContent("Could not load node executions");
  query.isError = false;
 });
});
