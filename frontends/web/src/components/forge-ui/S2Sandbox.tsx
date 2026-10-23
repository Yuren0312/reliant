/**
 * The R5 S2 primitives in ForgeTokenSandbox (/forge-token-sandbox): Toggle,
 * Input/Textarea, Dropdown and Modal, rendered the way their call sites use
 * them, OUTSIDE .forge-ui as they sit in the app. e2e/color-schemes.spec.ts
 * screenshots each region in every scheme × mode; the baselines were captured
 * on the pre-S2 components, so each S2 commit's image diff is exactly what it
 * changed.
 *
 *   s2-sites   switches (settings rows, compact rail/panel switches, a
 *              labelled switch) and text fields (label + description, an
 *              error, a search field with an icon, disabled, mono, textarea)
 *   s2-menus   ?view=s2-menus — RowMenu and MultiSelectFilter; the spec
 *              opens each one and captures the region with its menu
 *   s2-modal   ?view=s2-modal — a form dialog with header actions and a
 *              footer, opened from a button and captured as the viewport
 */
import { useState } from "react";
import { Search } from "lucide-react";

import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Modal } from "../ui/Modal";
import { Textarea } from "../ui/Textarea";
import { Toggle } from "../ui/Toggle";
import { RowMenu } from "../workflows/RowMenu";
import { MultiSelectFilter } from "../workflows/FilterMenu";

const noop = () => undefined;

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <h2 className="mb-3 text-sm font-medium text-foreground">{children}</h2>;
}

/** A settings row: title and description on the left, the switch on the right. */
function SettingRow({ title, checked, disabled }: { title: string; checked: boolean; disabled?: boolean }) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-border bg-card p-4">
      <div>
        <p className="text-sm font-medium text-foreground">{title}</p>
        <p className="mt-0.5 text-xs text-muted-foreground">Applies to every chat in this project.</p>
      </div>
      <Toggle checked={checked} onChange={noop} disabled={disabled} label={title} />
    </div>
  );
}

export function S2Sites() {
  return (
    <div className="space-y-8" data-testid="s2-sites">
      <div>
        <SectionTitle>Toggle — settings rows, compact switches, a labelled switch</SectionTitle>
        <div className="grid max-w-3xl grid-cols-2 gap-3">
          <SettingRow title="Response streaming" checked />
          <SettingRow title="Crash reporting" checked={false} />
          <SettingRow title="Always notify" checked disabled />
          <SettingRow title="Notify when unfocused" checked={false} disabled />
        </div>
        <div className="mt-3 flex items-center gap-6">
          <Toggle checked onChange={noop} srLabel="Start from chat" className="h-5 w-9 scale-90" />
          <Toggle checked={false} onChange={noop} srLabel="nightly enabled" className="h-5 w-9 flex-shrink-0 scale-90" />
          <div className="flex items-center gap-2">
            <Toggle checked onChange={noop} srLabel="Notify me when a run completes" />
            <span className="text-sm text-foreground" aria-hidden>
              Notify me when a run completes
            </span>
          </div>
        </div>
      </div>

      <div>
        <SectionTitle>Input / Textarea — MCP dialogs, token settings, search, confirm fields</SectionTitle>
        <div className="grid max-w-3xl grid-cols-2 gap-4">
          <Input label="Server Name" placeholder="my-custom-mcp" description="Unique identifier used in tool names" />
          <Input label="Header name" defaultValue="Authorization" state="error" error="A header name is required" />
          <Input placeholder="Search installed servers" leftIcon={<Search className="h-4 w-4" />} aria-label="Search installed MCP servers" />
          <Input defaultValue="rlnt_4be2d0a1c9f8e7d6" readOnly className="font-mono text-sm" aria-label="New token" />
          <Input placeholder="ada@example.com" disabled aria-label="Confirm email" />
          <Input placeholder="Token name (e.g. my-server)" aria-label="Token name" />
          <Textarea
            className="col-span-2"
            rows={3}
            placeholder="Describe the changes in this pull request"
            aria-label="Description"
          />
        </div>
      </div>
    </div>
  );
}

export function S2Menus() {
  const [states, setStates] = useState<string[]>(["failed", "live"]);
  return (
    <div className="flex h-[260px] items-start gap-16 pl-48" data-testid="s2-menus">
      <RowMenu
        label="Workflow actions"
        actions={[
          { label: "Duplicate", onSelect: noop },
          { label: "Rename", onSelect: noop },
          { label: "Delete", onSelect: noop, destructive: true },
        ]}
      />
      <MultiSelectFilter
        label="State"
        options={[
          { value: "failed", label: "Failed" },
          { value: "live", label: "Live" },
          { value: "done", label: "Done" },
        ]}
        value={states}
        onChange={setStates}
      />
    </div>
  );
}

export function S2Modal() {
  // Opened from a button rather than on load: an open dialog's scrim covers
  // the sandbox's scheme controls, which the spec drives first.
  const [open, setOpen] = useState(false);
  return (
    <div data-testid="s2-modal">
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-md border border-border bg-card px-3 py-1 text-sm text-foreground"
      >
        Open dialog
      </button>
      <Modal
        isOpen={open}
        onClose={() => setOpen(false)}
        title="Add custom MCP server"
        size="md"
        headerActions={<span className="text-xs text-muted-foreground">Step 1 of 2</span>}
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="outline">Cancel</Button>
            <Button variant="primary">Add server</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <Input label="Server Name" placeholder="my-custom-mcp" description="Unique identifier used in tool names" />
          <Input label="Command" placeholder="npx" />
          <Textarea rows={3} placeholder="Environment variables, one per line" aria-label="Environment" />
        </div>
      </Modal>
    </div>
  );
}
