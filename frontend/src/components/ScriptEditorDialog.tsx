import Editor from '@monaco-editor/react'
import { useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import '../monacoSetup'

interface ScriptEditorDialogProps {
  open: boolean
  scriptName: string
  initialContent: string
  saving: boolean
  onCancel: () => void
  onSave: (content: string) => Promise<boolean>
}

function trapTab(event: KeyboardEvent<HTMLDivElement>, container: HTMLDivElement | null) {
  if (event.key !== 'Tab' || !container) return
  const focusable = Array.from(container.querySelectorAll<HTMLElement>(
    'button:not([disabled]), textarea:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])',
  ))
  if (focusable.length === 0) {
    event.preventDefault()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

export default function ScriptEditorDialog({
  open,
  scriptName,
  initialContent,
  saving,
  onCancel,
  onSave,
}: ScriptEditorDialogProps) {
  const [draft, setDraft] = useState(initialContent)
  const [discardOpen, setDiscardOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [portal] = useState(() => document.createElement('div'))
  const titleID = useId()
  const discardTitleID = useId()
  const editorDialogRef = useRef<HTMLDivElement>(null)
  const discardDialogRef = useRef<HTMLDivElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  const keepRef = useRef<HTMLButtonElement>(null)
  const busy = saving || submitting

  useEffect(() => {
    if (!open) return
    setDraft(initialContent)
    setDiscardOpen(false)
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    document.body.appendChild(portal)
    const background = Array.from(document.body.children).filter((element) => element !== portal)
    const previousInert = background.map((element) => element.hasAttribute('inert'))
    background.forEach((element) => element.setAttribute('inert', ''))
    closeRef.current?.focus()
    return () => {
      background.forEach((element, index) => {
        if (!previousInert[index]) element.removeAttribute('inert')
      })
      portal.remove()
      previousFocus?.focus()
    }
  }, [initialContent, open, portal])

  useEffect(() => {
    if (discardOpen) keepRef.current?.focus()
  }, [discardOpen])

  if (!open) return null

  const requestClose = () => {
    if (busy) return
    if (draft !== initialContent) {
      setDiscardOpen(true)
    } else {
      onCancel()
    }
  }

  const save = async () => {
    if (busy) return
    setSubmitting(true)
    try {
      if (await onSave(draft)) onCancel()
    } catch {
      // The caller reports save errors; leave the draft available for retry.
    } finally {
      setSubmitting(false)
    }
  }

  return createPortal(
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/75 p-3 text-slate-100">
      <div
        ref={editorDialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        className="flex max-h-[calc(100vh-1.5rem)] w-[min(95vw,1000px)] flex-col overflow-hidden rounded-xl border border-slate-700 bg-slate-950 shadow-2xl"
        inert={discardOpen}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault()
            requestClose()
          }
          trapTab(event, editorDialogRef.current)
        }}
      >
        <div className="flex items-center gap-3 border-b border-slate-800 px-4 py-3">
          <h2 id={titleID} className="min-w-0 flex-1 truncate text-sm font-semibold">Edit {scriptName}</h2>
          <button
            ref={closeRef}
            type="button"
            aria-label="Close editor"
            className="rounded-md px-2 py-1 text-slate-300 hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:opacity-45"
            disabled={busy}
            onClick={requestClose}
          >
            ×
          </button>
        </div>
        <div className="min-h-[420px] flex-1 border-b border-slate-800">
          <Editor
            height="420px"
            language="sql"
            theme="vs-dark"
            value={draft}
            onChange={(value) => setDraft(value ?? '')}
            options={{ ariaLabel: 'SQL content', automaticLayout: true, minimap: { enabled: false } }}
          />
        </div>
        <div className="flex justify-end gap-2 px-4 py-3">
          <button
            type="button"
            className="rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-xs hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:opacity-45"
            disabled={busy}
            onClick={requestClose}
          >
            Cancel
          </button>
          <button
            type="button"
            className="rounded-md border border-sky-700 bg-sky-900 px-3 py-2 text-xs hover:bg-sky-800 focus:outline-none focus:ring-2 focus:ring-sky-500 disabled:opacity-45"
            disabled={busy}
            onClick={() => void save()}
          >
            Save
          </button>
        </div>
      </div>
      {discardOpen && (
        <div className="fixed inset-0 z-10 grid place-items-center bg-black/70 p-4">
          <div
            ref={discardDialogRef}
            role="alertdialog"
            aria-modal="true"
            aria-labelledby={discardTitleID}
            className="w-[min(92vw,420px)] rounded-xl border border-slate-700 bg-slate-950 p-4 shadow-2xl"
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault()
                event.stopPropagation()
                setDiscardOpen(false)
                closeRef.current?.focus()
              }
              trapTab(event, discardDialogRef.current)
            }}
          >
            <h3 id={discardTitleID} className="text-sm font-semibold">Discard changes?</h3>
            <p className="mt-2 text-xs text-slate-300">Your unsaved SQL edits will be lost.</p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                ref={keepRef}
                type="button"
                className="rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-sky-500"
                onClick={() => {
                  setDiscardOpen(false)
                  closeRef.current?.focus()
                }}
              >
                Keep editing
              </button>
              <button
                type="button"
                className="rounded-md border border-red-700 bg-red-900 px-3 py-2 text-xs focus:outline-none focus:ring-2 focus:ring-red-500"
                onClick={onCancel}
              >
                Discard changes
              </button>
            </div>
          </div>
        </div>
      )}
    </div>,
    portal,
  )
}
