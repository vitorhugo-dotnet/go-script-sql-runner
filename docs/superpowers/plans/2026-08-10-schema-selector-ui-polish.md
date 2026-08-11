# Schema Selector UI Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the runtime schema selector keyboard/scroll behavior, add a compact external GitHub link, and fix the Windows icon transparency without changing schema persistence or execution semantics.

**Architecture:** Keep `SchemaSelect` as a dependency-free controlled React component and add only local active-option state for keyboard navigation. Use the existing `window.runtime` Wails boundary for `BrowserOpenURL`, wrapped by a tiny frontend platform helper and injected into `App` for testing. Keep scrollbar styling in the existing global stylesheet and replace only the binary Windows icon asset.

**Tech Stack:** React 19.2.8, TypeScript 6.0.2, Tailwind CSS 4.3.3, Vitest 4.1.10, Testing Library, Wails v2.13.0, Windows `.ico` with alpha transparency.

## Global Constraints

- Do not add a third-party combobox or scrollbar dependency.
- The selected schema remains runtime-only and must not be persisted or auto-selected after connection.
- Keyboard navigation does not wrap at list boundaries.
- `Enter` commits only the active filtered schema; `Escape` closes without committing.
- The schema popup must never expose horizontal scrolling.
- Use Wails v2 `BrowserOpenURL` via `window.runtime`, not a shell command or platform-specific launcher.
- Preserve the existing logo artwork; only transparency/encoding of `build/windows/icon.ico` changes.

---

### Task 1: Keyboard-accessible schema combobox

**Files:**
- Modify: `frontend/src/components/SchemaSelect.test.tsx`
- Modify: `frontend/src/components/SchemaSelect.tsx`

**Interfaces:**
- Consumes: `SchemaSelectProps { schemas: string[]; value: string | null; onChange(schema: string | null): void; disabled: boolean }`
- Produces: the same public component API, plus internal `activeIndex: number | null`, stable listbox/option IDs, and `aria-activedescendant`.

- [ ] **Step 1: Write failing keyboard-navigation tests**

Add tests that render `schemas={['apollo', 'mysql', 'TestDB']}`, click the combobox, and assert:

```tsx
await user.keyboard('{ArrowDown}')
expect(input).toHaveAttribute('aria-activedescendant', optionIdFor('mysql'))

await user.keyboard('{ArrowUp}')
expect(input).toHaveAttribute('aria-activedescendant', optionIdFor('apollo'))

await user.keyboard('{Enter}')
expect(onChange).toHaveBeenLastCalledWith('apollo')
expect(input).toHaveValue('apollo')
expect(input).toHaveAttribute('aria-expanded', 'false')
```

Use the rendered option element IDs rather than hard-coding React `useId()` output:

```tsx
const apollo = screen.getByRole('option', { name: 'apollo' })
const mysql = screen.getByRole('option', { name: 'mysql' })
expect(apollo.id).not.toBe('')
expect(mysql.id).not.toBe('')
```

Add boundary assertions that repeated `ArrowUp` stays on the first option and repeated `ArrowDown` stays on the last option.

- [ ] **Step 2: Add failing tests for Escape, filtering, no results, and active-vs-selected state**

Cover these exact behaviors:

```tsx
await user.keyboard('{ArrowDown}{Escape}')
expect(onChange).not.toHaveBeenCalledWith('mysql')
expect(input).toHaveAttribute('aria-expanded', 'false')
```

After typing `test`, assert only `TestDB` remains, it becomes the active descendant, and `Enter` commits it. After typing a query with no matches, assert `No schemas found` is rendered and `Enter` does not call `onChange` with a schema value.

Also assert that keyboard highlight does not change `aria-selected` until `Enter` commits the option.

- [ ] **Step 3: Add a failing scroll-into-view test**

Mock the DOM method and verify keyboard navigation requests nearest scrolling:

```tsx
const scrollIntoView = vi.fn()
HTMLElement.prototype.scrollIntoView = scrollIntoView
await user.keyboard('{ArrowDown}')
expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest' })
```

Restore the original method after the test.

- [ ] **Step 4: Run the focused tests and verify RED**

Run:

```bash
cd frontend
npm test -- src/components/SchemaSelect.test.tsx
```

Expected: FAIL because the current component only handles `Escape` and has no active-option state.

- [ ] **Step 5: Implement minimal keyboard state and ARIA behavior**

Update `SchemaSelect.tsx` to use `useId`, `useRef`, and `activeIndex`:

```tsx
const [activeIndex, setActiveIndex] = useState<number | null>(null)
const listboxId = useId()
const activeOptionRef = useRef<HTMLButtonElement | null>(null)

const initialActiveIndex = () => {
  if (filtered.length === 0) return null
  const selectedIndex = value ? filtered.indexOf(value) : -1
  return selectedIndex >= 0 ? selectedIndex : 0
}
```

On focus/open, initialize the active index if none exists. On input changes, compute the next filtered list from the typed query and reset the active index to `0` when results exist or `null` otherwise. On `ArrowDown`/`ArrowUp`, call `preventDefault()`, open the list, and clamp the index with `Math.min`/`Math.max`. On `Enter`, call `select(filtered[activeIndex])` only when the index is valid. On `Escape`, close and clear only active navigation state, leaving the committed value untouched.

Expose the active option:

```tsx
aria-activedescendant={
  open && activeIndex !== null ? `${listboxId}-option-${activeIndex}` : undefined
}
```

Each option receives the corresponding `id`. Keep `aria-selected={schema === value}` tied only to the committed selection.

- [ ] **Step 6: Keep the highlighted option visible**

Add an effect:

```tsx
useEffect(() => {
  if (!open || activeIndex === null) return
  activeOptionRef.current?.scrollIntoView({ block: 'nearest' })
}, [activeIndex, open, filtered])
```

Assign `activeOptionRef` only to the currently highlighted option and add a visible `bg-slate-800 text-slate-50` class for that option.

- [ ] **Step 7: Run the focused tests and verify GREEN**

Run:

```bash
cd frontend
npm test -- src/components/SchemaSelect.test.tsx
```

Expected: PASS.

- [ ] **Step 8: Commit Task 1**

```bash
git add frontend/src/components/SchemaSelect.tsx frontend/src/components/SchemaSelect.test.tsx
git commit -m "feat: add keyboard schema navigation"
```

---

### Task 2: Remove horizontal scroll and style the schema scrollbar

**Files:**
- Modify: `frontend/src/components/SchemaSelect.tsx`
- Modify: `frontend/src/style.css`

**Interfaces:**
- Consumes: the listbox introduced/updated in Task 1.
- Produces: `.schema-options` scroll container with vertical-only scrolling and dark thin scrollbar styling.

- [ ] **Step 1: Add structural assertions to the component test**

After opening the selector, assert the listbox carries a stable styling hook:

```tsx
const listbox = screen.getByRole('listbox')
expect(listbox).toHaveClass('schema-options')
expect(listbox).toHaveClass('overflow-x-hidden')
expect(listbox).toHaveClass('overflow-y-auto')
```

Render a very long schema name and assert its option text container has `truncate` and `min-w-0` classes.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
cd frontend
npm test -- src/components/SchemaSelect.test.tsx
```

Expected: FAIL because the current listbox uses `overflow-auto` and has no `.schema-options` hook.

- [ ] **Step 3: Implement vertical-only overflow and truncation**

Change the listbox classes to include:

```text
schema-options overflow-x-hidden overflow-y-auto
```

Keep `max-h-48`, remove `overflow-auto`, and ensure each option is `min-w-0 overflow-hidden`. Wrap visible schema text in:

```tsx
<span className="block min-w-0 truncate">{schema}</span>
```

- [ ] **Step 4: Add scrollbar styling to `frontend/src/style.css`**

Add:

```css
.schema-options {
  scrollbar-width: thin;
  scrollbar-color: #475569 #020617;
}

.schema-options::-webkit-scrollbar {
  width: 8px;
}

.schema-options::-webkit-scrollbar-track {
  background: #020617;
  border-radius: 9999px;
}

.schema-options::-webkit-scrollbar-thumb {
  background: #475569;
  border: 2px solid #020617;
  border-radius: 9999px;
}

.schema-options::-webkit-scrollbar-thumb:hover {
  background: #64748b;
}
```

- [ ] **Step 5: Run the focused tests and frontend typecheck**

Run:

```bash
cd frontend
npm test -- src/components/SchemaSelect.test.tsx
npx tsc -b
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```bash
git add frontend/src/components/SchemaSelect.tsx frontend/src/components/SchemaSelect.test.tsx frontend/src/style.css
git commit -m "style: polish schema selector scrolling"
```

---

### Task 3: External GitHub footer link through Wails runtime

**Files:**
- Create: `frontend/src/platform/browser.ts`
- Create: `frontend/src/platform/browser.test.ts`
- Modify: `frontend/src/api/runner.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/MainView.test.tsx`

**Interfaces:**
- Produces: `openExternalUrl(url: string): void` calling `window.runtime.BrowserOpenURL(url)`.
- `AppProps` gains optional `openExternal?: (url: string) => void`, defaulting to `openExternalUrl`.

- [ ] **Step 1: Write a failing unit test for the browser boundary**

Create `frontend/src/platform/browser.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { openExternalUrl } from './browser'

describe('openExternalUrl', () => {
  afterEach(() => {
    window.runtime = undefined
  })

  it('opens URLs with the Wails BrowserOpenURL runtime', () => {
    const BrowserOpenURL = vi.fn()
    window.runtime = { EventsOn: vi.fn() as never, BrowserOpenURL }

    openExternalUrl('https://github.com/vitorhugo-dotnet/go-script-sql-runner')

    expect(BrowserOpenURL).toHaveBeenCalledWith(
      'https://github.com/vitorhugo-dotnet/go-script-sql-runner',
    )
  })
})
```

- [ ] **Step 2: Write a failing footer interaction test**

In `MainView.test.tsx`, render with a spy:

```tsx
const openExternal = vi.fn()
render(<App api={api as never} openExternal={openExternal} />)
const repositoryLink = screen.getByRole('link', {
  name: 'GitHub · vitorhugo-dotnet/go-script-sql-runner',
})
await user.click(repositoryLink)
expect(openExternal).toHaveBeenCalledWith(
  'https://github.com/vitorhugo-dotnet/go-script-sql-runner',
)
```

Assert the anchor still has the repository URL as `href`; the click handler must call `preventDefault()` before delegating to Wails so the WebView does not navigate.

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```bash
cd frontend
npm test -- src/platform/browser.test.ts src/components/MainView.test.tsx
```

Expected: FAIL because the boundary, prop, and footer do not exist yet.

- [ ] **Step 4: Extend the existing Wails runtime type**

In `frontend/src/api/runner.ts`, update the existing interface:

```ts
interface WailsRuntime {
  EventsOn(eventName: string, callback: (payload?: unknown) => void): () => void
  BrowserOpenURL(url: string): void
}
```

This matches Wails v2.13.0's runtime wrapper, whose `BrowserOpenURL(url)` delegates to `window.runtime.BrowserOpenURL(url)`.

- [ ] **Step 5: Implement the browser boundary**

Create `frontend/src/platform/browser.ts`:

```ts
export function openExternalUrl(url: string) {
  if (!window.runtime?.BrowserOpenURL) {
    throw new Error('Wails BrowserOpenURL runtime is unavailable')
  }
  window.runtime.BrowserOpenURL(url)
}
```

- [ ] **Step 6: Add the compact footer**

In `App.tsx`:

```tsx
const repositoryUrl = 'https://github.com/vitorhugo-dotnet/go-script-sql-runner'
```

Extend `AppProps` with `openExternal?: (url: string) => void`, default it to `openExternalUrl`, change the root grid rows to:

```text
grid-rows-[auto_auto_minmax(0,1fr)_132px_auto]
```

Add below the logs section:

```tsx
<footer className="flex items-center border-t border-slate-900 px-3 py-1 text-[10px] text-slate-600">
  <a
    href={repositoryUrl}
    className="transition hover:text-slate-400 focus:outline-none focus:ring-1 focus:ring-sky-600"
    onClick={(event) => {
      event.preventDefault()
      openExternal(repositoryUrl)
    }}
  >
    GitHub · vitorhugo-dotnet/go-script-sql-runner
  </a>
</footer>
```

- [ ] **Step 7: Run focused tests and typecheck**

Run:

```bash
cd frontend
npm test -- src/platform/browser.test.ts src/components/MainView.test.tsx
npx tsc -b
```

Expected: PASS.

- [ ] **Step 8: Commit Task 3**

```bash
git add frontend/src/platform/browser.ts frontend/src/platform/browser.test.ts frontend/src/api/runner.ts frontend/src/App.tsx frontend/src/components/MainView.test.tsx
git commit -m "feat: add external repository footer link"
```

---

### Task 4: Regenerate the Windows icon with transparent alpha

**Files:**
- Modify: `build/windows/icon.ico`

**Interfaces:**
- Consumes: the existing icon artwork stored in `build/windows/icon.ico`.
- Produces: a multi-size ICO whose background-connected matte pixels have alpha `0`, while preserving non-background artwork.

- [ ] **Step 1: Inspect the existing icon frames and alpha**

Download/read `build/windows/icon.ico` and run a Pillow inspection that prints every embedded frame size and the RGBA corner pixel. Verify the screenshot symptom is reproducible as an opaque matte in at least the title-bar-sized frame.

- [ ] **Step 2: Extract the largest frame and remove only the connected background matte**

Convert the largest frame to RGBA. Starting from the four corners, flood-fill only contiguous pixels matching the corner matte color within a small tolerance and set those pixels to alpha `0`. Do not globally erase black/dark pixels, because dark pixels may be intentional artwork.

- [ ] **Step 3: Regenerate the ICO as PNG-compressed alpha frames**

Resize the cleaned RGBA source with high-quality resampling and save frames for:

```text
16x16, 24x24, 32x32, 48x48, 64x64, 128x128, 256x256
```

Write them back to `build/windows/icon.ico` with alpha preserved.

- [ ] **Step 4: Verify the generated icon programmatically**

Reopen every ICO frame with Pillow and assert:

```text
mode converts cleanly to RGBA
corner alpha == 0
at least one pixel has alpha == 255
all required frame sizes are present
```

- [ ] **Step 5: Commit Task 4**

```bash
git add build/windows/icon.ico
git commit -m "fix: preserve transparency in Windows icon"
```

---

### Task 5: Full regression and Windows build verification

**Files:**
- No new source files unless a test failure reveals a defect in Tasks 1-4.

**Interfaces:**
- Consumes: completed frontend and icon changes.
- Produces: evidence that the branch matches the repository's CI contract.

- [ ] **Step 1: Run the complete Go suite**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run the complete frontend suite**

```bash
cd frontend
npm ci
npm test
npx tsc -b
npm run build
```

Expected: PASS.

- [ ] **Step 3: Build the Windows application with the CI command**

From the repository root on Windows or the GitHub Actions Windows runner:

```powershell
wails build -clean -webview2 embed -windowsconsole
```

Expected: `build/bin/go-script-sql-runner.exe` exists.

- [ ] **Step 4: Perform the final Windows visual check**

Open the built executable and verify:

```text
- title-bar/taskbar icon has transparent surroundings, not a black square
- schema popup has no horizontal scrollbar
- vertical scrollbar matches the dark UI
- ArrowUp/ArrowDown visibly move the highlighted schema
- Enter commits the highlighted schema
- GitHub footer link opens the system default browser
```

- [ ] **Step 5: Commit any verification-only correction, otherwise leave the branch unchanged**

If a correction was necessary, make a focused commit with the failing behavior in its message. Do not squash away the TDD history before review.
