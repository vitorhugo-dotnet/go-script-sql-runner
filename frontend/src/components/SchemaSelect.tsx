import { useEffect, useMemo, useState } from 'react'

interface SchemaSelectProps {
  schemas: string[]
  value: string | null
  onChange: (schema: string | null) => void
  disabled: boolean
}

export default function SchemaSelect({ schemas, value, onChange, disabled }: SchemaSelectProps) {
  const [query, setQuery] = useState(value ?? '')
  const [open, setOpen] = useState(false)

  useEffect(() => {
    setQuery(value ?? '')
  }, [value])

  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase()
    if (!normalized || value === query) return schemas
    return schemas.filter((schema) => schema.toLocaleLowerCase().includes(normalized))
  }, [query, schemas, value])

  const select = (schema: string) => {
    setQuery(schema)
    setOpen(false)
    onChange(schema)
  }

  return (
    <div className="relative min-w-40">
      <input
        aria-label="Schema"
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={open && !disabled}
        aria-controls="schema-options"
        className="w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-xs text-slate-100 outline-none focus:ring-2 focus:ring-sky-500 disabled:cursor-not-allowed disabled:opacity-45"
        placeholder={disabled ? 'Connect first' : 'Select schema…'}
        disabled={disabled}
        value={query}
        onFocus={() => setOpen(true)}
        onChange={(event) => {
          const next = event.target.value
          setQuery(next)
          setOpen(true)
          if (next !== value) onChange(null)
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') setOpen(false)
        }}
      />

      {open && !disabled && (
        <div
          id="schema-options"
          role="listbox"
          className="absolute left-0 top-full z-40 mt-1 max-h-48 w-full min-w-48 overflow-auto rounded-md border border-slate-700 bg-slate-950 p-1 shadow-xl"
        >
          {filtered.length === 0 ? (
            <div className="px-2 py-1.5 text-xs text-slate-500">No schemas found</div>
          ) : (
            filtered.map((schema) => (
              <button
                key={schema}
                type="button"
                role="option"
                aria-selected={schema === value}
                className="block w-full rounded px-2 py-1.5 text-left text-xs text-slate-200 hover:bg-slate-800 focus:bg-slate-800 focus:outline-none"
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => select(schema)}
              >
                {schema}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  )
}
