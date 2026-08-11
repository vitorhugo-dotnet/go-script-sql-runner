# Go Script SQL Runner — Schema Selector UI Polish

**Date:** 2026-08-10  
**Status:** Approved  
**Scope:** Desktop schema selector interaction, Windows icon transparency, and repository footer link

## 1. Goal

Polish the runtime schema-selection UI introduced by the runtime schema feature without changing its persistence or execution semantics.

The change must improve keyboard navigation, remove horizontal scrolling from the schema list, make the vertical scrollbar visually consistent with the dark UI, fix the Windows application icon background, and expose a GitHub repository link at the bottom-left of the desktop window.

## 2. Schema combobox keyboard behavior

`SchemaSelect` remains a local React component with no new third-party dependency.

When the selector has loaded schemas and is enabled:

- focusing/clicking the input opens the list;
- `ArrowDown` opens the list when needed and moves the active option downward;
- `ArrowUp` opens the list when needed and moves the active option upward;
- navigation stops at the first/last option and does not wrap;
- opening a list with no active option activates the currently selected schema when it is still visible, otherwise the first filtered option;
- typing/filtering resets the active option to the first filtered result;
- `Enter` confirms the currently active option, closes the list, and calls `onChange` with that schema;
- if there are no filtered options, `Enter` does nothing;
- `Escape` closes the list without changing the selected schema;
- the active option must automatically scroll into view while navigating with the keyboard;
- mouse selection continues to work as it does today.

The combobox must expose the active option through `aria-activedescendant`, while options continue using `role="option"` and `aria-selected` for the committed value.

## 3. Schema list layout and scrollbar

The schema list must never expose a horizontal scrollbar.

Rules:

- vertical scrolling remains enabled when the list exceeds its maximum height;
- horizontal overflow is hidden;
- long schema names stay inside the selector width and are visually truncated instead of expanding the popup;
- the vertical scrollbar is thin and styled to match the existing slate/dark UI;
- the scrollbar styling may use standards-based properties plus WebKit pseudo-elements for the embedded Chromium/WebView environment, with graceful fallback when unsupported.

No scrollbar library or additional runtime dependency is introduced.

## 4. Windows icon transparency

`build/windows/icon.ico` must be regenerated/fixed so its image layers preserve alpha transparency instead of carrying a black matte/background.

The icon must continue to contain the sizes required by the Windows/Wails build. The visible artwork must remain the existing Go Script SQL Runner logo; this task changes transparency/encoding, not the logo concept.

Acceptance is based on the packaged Windows application showing the icon without a black square in the title bar/taskbar where Windows renders transparency.

## 5. GitHub footer link

Add a compact footer row at the bottom of the main desktop layout, below the logs area.

The left side contains a subtle link identifying the repository, for example:

`GitHub · vitorhugo-dotnet/go-script-sql-runner`

Clicking the link must open the repository in the user's default system browser rather than navigating the embedded WebView.

The project currently uses Wails v2.13.0, so implementation should use the official Wails v2 browser runtime (`BrowserOpenURL`) rather than a custom shell command or platform-specific launcher.

The footer must stay visually secondary and must not consume meaningful workspace height.

## 6. Testing

Frontend tests must cover:

- `ArrowDown` and `ArrowUp` move the active schema through the filtered list;
- keyboard navigation does not wrap past list boundaries;
- `Enter` commits the active schema;
- `Escape` closes without committing a new schema;
- filtering resets keyboard navigation to a valid visible option;
- no-result filtering makes `Enter` a no-op;
- committed selection remains distinct from the merely active/highlighted option;
- the repository footer link is rendered and invokes the external-browser behavior through a testable boundary.

The icon fix is verified through the Windows build artifact/manual visual check because browser/unit tests cannot validate Windows shell rendering.

## 7. Non-goals

This change does not:

- persist the selected schema;
- auto-select a schema after connecting;
- change schema discovery or backend execution logic;
- add a component library;
- redesign the application layout beyond the small footer row and schema popup polish;
- change the logo artwork itself.

## 8. Acceptance criteria

The change is complete when:

1. schemas can be navigated with `ArrowUp` and `ArrowDown`;
2. `Enter` confirms the highlighted schema;
3. `Escape` closes the list without an unintended selection;
4. the highlighted schema remains visible while keyboard-scrolling;
5. the schema popup has no horizontal scrollbar;
6. the vertical scrollbar visually matches the dark UI;
7. the Windows application icon no longer renders with a black background around the logo;
8. a repository link appears at the bottom-left of the desktop window;
9. clicking that link opens the repository in the default system browser through Wails' official browser runtime;
10. existing behavior requiring an explicit runtime schema remains unchanged;
11. relevant frontend tests and the existing project test suite pass.
