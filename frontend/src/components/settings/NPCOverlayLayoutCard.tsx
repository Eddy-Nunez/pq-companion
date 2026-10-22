import React, { useEffect, useMemo, useState } from 'react'
import { GripVertical } from 'lucide-react'
import {
  DndContext,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  closestCenter,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  rectSortingStrategy,
  verticalListSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { restrictToVerticalAxis } from '@dnd-kit/modifiers'
import { loadEnums } from '../../lib/enumsCache'
import {
  NPC_SECTIONS,
  resolveSectionOrder,
  sortByTagOrder,
  type NPCSectionId,
} from '../../lib/npcOverlayLayout'
import { emphasisStyle, resolveTagStyle } from '../../lib/npcTagStyle'
import type { EnumsCatalog } from '../../types/enums'
import type { NPCOverlayTagStyle } from '../../types/config'

interface Props {
  sectionOrder: string[]
  tagOrder: number[]
  /** Read-only: used to preview tags in their configured colours. */
  tagStyle: NPCOverlayTagStyle
  onChange: (sectionOrder: string[], tagOrder: number[]) => void
}

const SECTION_LABELS = new Map(NPC_SECTIONS.map((s) => [s.id, s.label]))

function SortableSection({ id, index }: { id: NPCSectionId; index: number }): React.ReactElement {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id })
  return (
    <div
      ref={setNodeRef}
      className="flex items-center gap-2 rounded px-2 py-1.5"
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        backgroundColor: 'var(--color-surface-2)',
        border: '1px solid var(--color-border)',
        opacity: isDragging ? 0.4 : 1,
      }}
    >
      <span
        {...attributes}
        {...listeners}
        className="cursor-grab touch-none active:cursor-grabbing"
        style={{ color: 'var(--color-muted)' }}
        title="Drag to reorder"
      >
        <GripVertical size={14} />
      </span>
      <span className="w-4 text-right text-xs tabular-nums" style={{ color: 'var(--color-muted)' }}>
        {index + 1}
      </span>
      <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
        {SECTION_LABELS.get(id)}
      </span>
    </div>
  )
}

function SortableTag({
  code,
  name,
  hint,
  tagStyle,
}: {
  code: number
  name: string
  hint?: string
  tagStyle: NPCOverlayTagStyle
}): React.ReactElement {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: String(code),
  })
  const { color, emphasized } = resolveTagStyle(code, tagStyle)
  return (
    <span
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      title={hint}
      className="cursor-grab touch-none select-none rounded px-1.5 py-0.5 text-[10px] font-semibold text-white active:cursor-grabbing"
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        backgroundColor: color,
        whiteSpace: 'nowrap',
        opacity: isDragging ? 0.4 : 1,
        ...(emphasized ? emphasisStyle(color) : null),
      }}
    >
      {name}
    </span>
  )
}

export default function NPCOverlayLayoutCard({
  sectionOrder,
  tagOrder,
  tagStyle,
  onChange,
}: Props): React.ReactElement {
  const [catalog, setCatalog] = useState<EnumsCatalog | null>(null)

  useEffect(() => {
    loadEnums().then(setCatalog).catch(() => {})
  }, [])

  // dnd-kit sensors: a small activation distance lets a click through without
  // starting a drag, and keyboard reordering comes for free.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const sections = resolveSectionOrder(sectionOrder)

  const tags = useMemo(() => {
    const all = Object.entries(catalog?.special_abilities ?? {})
      .map(([code, meta]) => ({ code: Number(code), name: meta.name, hint: meta.description || undefined }))
      .sort((a, b) => a.code - b.code)
    return sortByTagOrder(all, tagOrder)
  }, [catalog, tagOrder])

  const onSectionDragEnd = ({ active, over }: DragEndEvent): void => {
    if (!over || active.id === over.id) return
    const from = sections.indexOf(active.id as NPCSectionId)
    const to = sections.indexOf(over.id as NPCSectionId)
    onChange(arrayMove(sections, from, to), tagOrder)
  }

  const onTagDragEnd = ({ active, over }: DragEndEvent): void => {
    if (!over || active.id === over.id) return
    const codes = tags.map((t) => t.code)
    const from = codes.indexOf(Number(active.id))
    const to = codes.indexOf(Number(over.id))
    onChange(sectionOrder, arrayMove(codes, from, to))
  }

  const linkStyle: React.CSSProperties = { color: 'var(--color-primary)' }
  const subheading = 'text-xs font-semibold uppercase tracking-wide'

  return (
    <section
      className="rounded-lg p-4"
      style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
    >
      <h2
        className="mb-1 text-sm font-semibold uppercase tracking-wide"
        style={{ color: 'var(--color-muted)' }}
      >
        NPC Overlay Layout
      </h2>
      <p className="mb-4 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Drag to reorder the Stats view on both overlay surfaces. Put the information your role
        needs first — a puller might lead with special abilities, a nuker with resists.
      </p>

      <div className="grid gap-6 md:grid-cols-[14rem_1fr]">
        <div>
          <div className="mb-2 flex items-center justify-between">
            <h3 className={subheading} style={{ color: 'var(--color-muted)' }}>Sections</h3>
            {sectionOrder.length > 0 && (
              <button onClick={() => onChange([], tagOrder)} className="text-xs" style={linkStyle}>
                Reset
              </button>
            )}
          </div>
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            modifiers={[restrictToVerticalAxis]}
            onDragEnd={onSectionDragEnd}
          >
            <SortableContext items={sections} strategy={verticalListSortingStrategy}>
              <div className="flex flex-col gap-1">
                {sections.map((id, i) => (
                  <SortableSection key={id} id={id} index={i} />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <h3 className={subheading} style={{ color: 'var(--color-muted)' }}>
              Special ability order
            </h3>
            {tagOrder.length > 0 && (
              <button onClick={() => onChange(sectionOrder, [])} className="text-xs" style={linkStyle}>
                Reset
              </button>
            )}
          </div>
          <p className="mb-2 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            Drag the tags you care about most to the front. A target shows only the tags it has,
            in this order.
          </p>
          <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onTagDragEnd}>
            <SortableContext items={tags.map((t) => String(t.code))} strategy={rectSortingStrategy}>
              <div className="flex flex-wrap gap-1.5">
                {tags.map((t) => (
                  <SortableTag key={t.code} code={t.code} name={t.name} hint={t.hint} tagStyle={tagStyle} />
                ))}
              </div>
            </SortableContext>
          </DndContext>
        </div>
      </div>
    </section>
  )
}
