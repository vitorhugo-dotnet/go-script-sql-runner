import { useEffect, useId, useMemo, useRef, useState } from 'react'

interface SchemaSelectProps {
  schemas: string[]
  value: string | null
  onChange: (schema: string | null) => void
  disabled: boolean
}

function filterSchemas(schemas: string[], query: string, value: string | null) {
  const normalized = query.trim().toLocaleLowerCase()
  if (!normalized || value === query) return schemas
  return schemas.filter((schema) => schema.toLocaleLowerCase().includes(normalized))
}

function firstActiveIndex(schemas: string[], value: string | null) {
  if (schemas.length === 0) return null
  const selectedIndex = value ? schemas.indexOf(value) : -1
  return selectedIndex >= 0 ? selectedIndex : 0
}

export default function SchemaSelect({ schemas, value, onChange, disabled }: SchemaSelectProps) {
  const [query, setQuery] = useState(value ?? '')
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState<number | null>(null)
  const listboxId = useId()
  const activeOptionRef = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    setQuery(value ?? '')
  }, [value])

  const filtered = useMemo(() => filterSchemas(schemas, query, value), [query, schemas, value])

  useEffect(() => {
    if (!open || activeIndex === null) return
    activeOptionRef.current?.scrollIntoView({ block: 'nearest' })
  }, [activeIndex, filtered, open])

  const select = (schema: string) => {
    setQuery(schema)
    setOpen(false)
    setActiveIndex(null)
    onChange(schema)
  }

  const openWithActiveOption = () => {
    setOpen(true)
    setActiveIndex((current) => {
      if (current !== null && current < filtered.length) return current
      return firstActiveIndex(filtered, value)
    })
  }

  return (
    <div className="relative min-w-40">
      <input
        aria-label="Schema"
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={open && !disabled}
        aria-controls={listboxId}
        aria-activedescendant={
          open && activeIndex !== null ? `${listboxId}-option-${activeIndex}` : undefined
        }
        className="w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-xs text-slate-100 outline-none focus:ring-2 focus:ring-sky-500 disabled:cursor-not-allowed disabled:opacity-45"
        placeholder={disabled ? 'Connect first' : 'Select schema…'}
        disabled={disabled}
        value={query}
        onFocus={openWithActiveOption}
        onChange={(event) => {
          const next = event.target.value
          const nextFiltered = filterSchemas(schemas, next, value)
          setQuery(next)
          setOpen(true)
          setActiveIndex(nextFiltered.length > 0 ? 0 : null)
          if (next !== value) onChange(null)
        }}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown') {
            event.preventDefault()
            setOpen(true)
            setActiveIndex((current) => {
              if (filtered.length === 0) return null
              if (current === null) return firstActiveIndex(filtered, value)
              return Math.min(current + 1, filtered.length - 1)
            })
            return
          }

          if (event.key === 'ArrowUp') {
            event.preventDefault()
            setOpen(true)
            setActiveIndex((current) => {
              if (filtered.length === 0) return null
              if (current === null) return firstActiveIndex(filtered, value)
              return Math.max(current - 1, 0)
            })
            return
          }

          if (event.key === 'Enter') {
            if (open && activeIndex !== null && filtered[activeIndex]) {
              event.preventDefault()
              select(filtered[activeIndex])
            }
            return
          }

          if (event.key === 'Escape') {
            setOpen(false)
            setActiveIndex(null)
          }
        }}
      />

      {open && !disabled && (
        <div
          id={listboxId}
          role="listbox"
          className="absolute left-0 top-full z-40 mt-1 max-h-48 w-full min-w-48 overflow-auto rounded-md border border-slate-700 bg-slate-950 p-1 shadow-xl"
        >
          {filtered.length === 0 ? (
            <div className="px-2 py-1.5 text-xs text-slate-500">No schemas found</div>
          ) : (
            filtered.map((schema, index) => {
              const active = index === activeIndex
              return (
                <button
                  key={schema}
                  id={`${listboxId}-option-${index}`}
                  ref={active ? activeOptionRef : undefined}
                  type="button"
                  role="option"
                  aria-selected={schema === value}
                  className={`block w-full rounded px-2 py-1.5 text-left text-xs focus:outline-none ${
                    active
                      ? 'bg-slate-800 text-slate-50'
                      : 'text-slate-200 hover:bg-slate-800 focus:bg-slate-800'
                  }`}
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => select(schema)}
                >
                  {schema}
                </button>
              )
            })
          )}
        </div>
      )}
    </div>
  )
}
