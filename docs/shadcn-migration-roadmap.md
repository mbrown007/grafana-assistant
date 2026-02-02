# shadcn/ui Migration Roadmap — Remaining 20%

## What's Already Done

**Infrastructure:** Tailwind CSS, CSS variables (light/dark), ThemeProvider with localStorage, `cn()` utility, `@` path alias, PostCSS config.

**Installed shadcn components (8):** Button, Input, Avatar, DropdownMenu, Switch, Card, Collapsible, Table.

**Converted in ChatPanel.tsx:** Header buttons, send button, chat input, Flavio avatar, settings dropdown, dark mode toggle, remaining buttons (chips + history actions).

---

## Phase 1 — Replace remaining raw `<button>` elements in ChatPanel.tsx ✅

No new components to install. Just swap the 7 remaining `<button>` elements for the existing `Button` component.

### Task 1.1: Suggestion chips + retry button

**File:** `frontend/src/components/ChatPanel.tsx`

Replace the "Retry login" button and 3 suggestion chip buttons (currently `<button className="chip">`) with:

```tsx
<Button variant="outline" size="sm" className="rounded-full" ...>
```

This gives the pill shape via `rounded-full` while using the standard Button. After converting, remove the `.chip` and `.chip-row` CSS classes from `styles.css` and replace `.chip-row` usage with `<div className="flex flex-wrap gap-2 justify-center">`.

### Task 1.2: History panel buttons

**File:** `frontend/src/components/ChatPanel.tsx`

| Current | Replace with |
|---------|-------------|
| `<button className="icon-button">` (close history, line ~478) | `<Button variant="ghost" size="icon">` |
| `<button className="icon-button danger">` (delete chat, line ~507) | `<Button variant="ghost" size="icon" className="text-destructive hover:text-destructive hover:bg-destructive/10">` |
| Plain `<button>` (history item select, line ~501) | Keep as-is or wrap in a styled `<button>` — this is a full-width text button, not a good fit for Button component |

After converting, remove `.icon-button` and `.icon-button.danger` CSS from `styles.css`.

### Task 1.3: Verify + clean up CSS

After all button swaps:
1. `npm run build` — confirm no errors
2. `npm test` — confirm tests pass
3. Delete these CSS classes from `styles.css`: `.chip`, `.chip-row`, `.icon-button`, `.icon-button.danger`, `.ghost-button` (unused), `.hide-chat-button` (unused)

---

## Phase 2 — Install Card, Collapsible, and Table components ✅

### Task 2.1: Install new shadcn components

```bash
npx shadcn@latest add card collapsible table
```

This adds files to `frontend/src/components/ui/` and installs `@radix-ui/react-collapsible` as a new dependency.

### Task 2.2: Convert tool call `<details>/<summary>` to Collapsible

**File:** `frontend/src/components/ChatPanel.tsx` (lines ~421-436)

Current pattern:
```tsx
<details className="tool-call">
  <summary><Wrench size={14} /> {call.tool}</summary>
  <div className="tool-body">...</div>
</details>
```

Replace with:
```tsx
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from './ui/collapsible';

<Collapsible>
  <CollapsibleTrigger className="flex items-center gap-2 w-full p-2 text-sm font-medium hover:bg-accent rounded">
    <Wrench size={14} /> {call.tool}
  </CollapsibleTrigger>
  <CollapsibleContent className="p-2 pt-0">
    ...
  </CollapsibleContent>
</Collapsible>
```

Wrap each collapsible in `<Card className="overflow-hidden">` to replace the `.tool-call` border/background styling.

Remove from `styles.css`: `.tool-call`, `.tool-body`, `.tool-label`, `.tool-calls` container styles.

### Task 2.3: Convert artifact tables

**File:** `frontend/src/components/Artifact.tsx` (lines ~206-231)

Replace raw `<table>/<thead>/<tbody>/<tr>/<th>/<td>` with shadcn Table components:

```tsx
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from './ui/table';
```

Map the existing markup 1:1. Keep the inline `style={{ textAlign: column.align }}` for now — it's dynamic and fine.

Remove from `styles.css`: `.artifact-table` and its child selectors.

### Task 2.4: Convert artifact cards and metric cards

**File:** `frontend/src/components/Artifact.tsx`

Replace `<div className="artifact-card">` with:
```tsx
import { Card, CardHeader, CardContent } from './ui/card';

<Card>
  <CardHeader>
    <CardTitle>{title}</CardTitle>
    {subtitle && <CardDescription>{subtitle}</CardDescription>}
  </CardHeader>
  <CardContent>...</CardContent>
</Card>
```

For metric cards (`<div className="metric-card metric-blue">`), use:
```tsx
<Card className="border-l-4 border-l-blue-500">
  <CardContent className="flex items-center justify-between p-3">
    ...
  </CardContent>
</Card>
```

Color mapping for `border-l-*`: blue-500, green-500, red-500, amber-500, purple-500.

Remove from `styles.css`: `.metric-card`, `.metric-blue` through `.metric-purple`, `.artifact-card`, `.artifact-header`.

### Task 2.5: Verify

1. `npm run build`
2. `npm test`
3. Visually check: tool call expand/collapse, artifact tables, metric cards in light and dark mode

---

## Phase 3 — Convert message bubbles and history panel ✅

### Task 3.1: Message bubbles to Card

**File:** `frontend/src/components/ChatPanel.tsx`

Replace `<div className="message-bubble">` with `<Card className="max-w-[85%] p-3">`. Use a conditional class for user messages: `className="max-w-[85%] p-3 bg-primary/10 border-primary/20"`.

Remove from `styles.css`: `.message-bubble`, `.chat-message.user .message-bubble`.

### Task 3.2: History panel — install Sheet (optional)

The history panel currently uses a custom CSS transform slide-in. You can optionally replace it with shadcn Sheet:

```bash
npx shadcn@latest add sheet
```

Replace the `<div className="history-panel">` with:
```tsx
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from './ui/sheet';

<Sheet open={showHistory} onOpenChange={setShowHistory}>
  <SheetContent side="right">
    <SheetHeader>
      <SheetTitle>Previous chats</SheetTitle>
      <SheetDescription>Pick a conversation or delete it.</SheetDescription>
    </SheetHeader>
    ...
  </SheetContent>
</Sheet>
```

This gives you accessibility (focus trap, Escape to close, overlay) for free. Remove from `styles.css`: `.history-panel`, `.history-panel.open`, `.history-header`, `.history-body`, `.history-title`, `.history-subtitle`.

### Task 3.3: History items

Each history item can use Card:
```tsx
<Card className="flex items-center justify-between p-3">
  <button onClick={...} className="text-left flex-1">
    <p className="text-sm font-semibold">{title}</p>
    <p className="text-xs text-muted-foreground mt-1">{when}</p>
  </button>
  <Button variant="ghost" size="icon" ...><Trash2 size={14} /></Button>
</Card>
```

Remove from `styles.css`: `.history-item`, `.history-item-title`, `.history-item-meta`, `.history-list`, `.history-status`.

### Task 3.4: Verify

1. `npm run build && npm test`
2. Test: open/close history panel, select a conversation, delete a conversation, send messages in light and dark mode

---

## Phase 4 — Inline Tailwind for remaining typography and layout classes ✅

This phase replaces custom CSS classes with Tailwind utility classes directly in JSX. Work through one component at a time.

### Task 4.1: ChatPanel.tsx layout classes

Replace these custom classes with inline Tailwind:

| CSS class | Replace with (Tailwind) |
|-----------|------------------------|
| `.chat-header` | `className="w-full max-w-[360px] px-4 py-3 border-b border-border flex items-center justify-between"` |
| `.chat-header-actions` | `className="inline-flex items-center gap-2"` |
| `.chat-title` | `className="text-base font-semibold truncate"` |
| `.chat-subtitle` | `className="text-xs text-muted-foreground mt-0.5"` |
| `.chat-messages` | `className="flex-1 w-full max-w-[360px] overflow-y-auto px-4 py-3 flex flex-col gap-4"` |
| `.chat-empty` | `className="flex-1 grid place-items-center"` |
| `.chat-input` | `className="w-full max-w-[360px] px-4 py-3 border-t border-border flex gap-2"` |
| `.chat-error` | `className="text-red-400"` |
| `.streaming-indicator` | `className="inline-flex items-center gap-2 text-xs text-muted-foreground mt-2"` + keep the spin animation on the Loader2 via `className="animate-spin"` |

### Task 4.2: Artifact.tsx typography classes

| CSS class | Replace with (Tailwind) |
|-----------|------------------------|
| `.metric-label` | `className="text-xs text-muted-foreground"` |
| `.metric-value` | `className="text-xl font-semibold mt-1"` |
| `.metric-change` | `className="flex items-center gap-1 text-xs text-muted-foreground mt-1"` |
| `.metric-change-label` | `className="opacity-70"` |
| `.artifact-title` | `className="text-base font-semibold"` |
| `.artifact-subtitle` | `className="text-sm text-muted-foreground"` |
| `.artifact-description` | `className="text-xs text-muted-foreground mt-1"` |
| `.artifact-empty` | `className="text-center text-muted-foreground py-4"` |
| `.artifact-text` | `className="m-0 text-muted-foreground"` |
| `.artifact-section-title` | `className="font-semibold mb-2"` |
| `.metric-grid` | `className="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-3"` |

### Task 4.3: MarkdownContent.tsx classes

| CSS class | Replace with (Tailwind) |
|-----------|------------------------|
| `.inline-code` | `className="bg-muted px-1.5 py-0.5 rounded text-sm break-all"` |
| `.code-block` | `className="bg-muted rounded-lg p-4 overflow-x-auto max-w-full"` |
| `.md-paragraph` | `className="my-2"` |
| `.md-list` | `className="my-2 ml-5"` |
| `.md-sublist` | `className="mt-1 ml-4"` |
| `.md-heading` | `className="mt-5 mb-2 font-semibold"` (add `text-xl`/`text-lg`/`text-base` per level) |

### Task 4.4: Clean up styles.css

After inlining all the above, `styles.css` should only contain:
- CSS variable definitions (`:root` and `.dark` blocks)
- Tailwind `@tailwind` directives
- App-shell layout (`.app-shell`, `.app-main`, `.grafana-pane`, `.chat-pane`) — these are structural and fine as CSS
- `.chat-launcher` button styles (if still used)
- Any animation keyframes not covered by Tailwind

Delete everything else. Target: reduce `styles.css` from ~775 lines to ~150 lines.

### Task 4.5: Final verification

1. `npm run build && npm test`
2. Visual check every screen: empty state, active chat, tool calls expanded, artifacts (tables, metrics, reports), history panel — in both light and dark mode
3. `go build ./...` (backend unchanged, but confirm nothing broke)

---

## Component install cheatsheet

```bash
# Phase 2
npx shadcn@latest add card collapsible table

# Phase 3 (optional)
npx shadcn@latest add sheet
```

## Files touched per phase

| Phase | Files modified | Files added |
|-------|---------------|-------------|
| 1 | ChatPanel.tsx, styles.css, ChatPanel.test.tsx | — |
| 2 | ChatPanel.tsx, Artifact.tsx, styles.css | ui/card.tsx, ui/collapsible.tsx, ui/table.tsx |
| 3 | ChatPanel.tsx, styles.css | ui/sheet.tsx (optional) |
| 4 | ChatPanel.tsx, Artifact.tsx, MarkdownContent.tsx, styles.css | — |
