import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ListChecks, Plus, Pencil, Trash2, Check, X, GripVertical, ChevronDown, ChevronRight,
  Download, Upload,
} from 'lucide-react'
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
  verticalListSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
  arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { restrictToVerticalAxis } from '@dnd-kit/modifiers'
import {
  listCharacters,
  listCharacterTasks,
  createCharacterTask,
  updateCharacterTask,
  deleteCharacterTask,
  reorderCharacterTasks,
  createCharacterSubtask,
  updateCharacterSubtask,
  deleteCharacterSubtask,
  importCharacterTask,
  MAX_SUBTASK_DEPTH,
  type Character,
  type CharacterTask,
  type Subtask,
} from '../services/api'
import { useActiveCharacter } from '../contexts/ActiveCharacterContext'
import CharacterSubTabs from '../components/CharacterSubTabs'
import { countLeaves, downloadTaskFile } from '../lib/taskFile'

interface TaskFormProps {
  initialName: string
  initialDescription: string
  saving: boolean
  error: string | null
  onSave: (name: string, description: string) => void
  onCancel: () => void
  saveLabel?: string
}

function TaskForm({
  initialName, initialDescription, saving, error, onSave, onCancel, saveLabel = 'Save',
}: TaskFormProps): React.ReactElement {
  const [name, setName] = useState(initialName)
  const [description, setDescription] = useState(initialDescription)

  return (
    <div
      className="rounded-lg p-4 space-y-3"
      style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
    >
      <input
        type="text"
        placeholder="Task name"
        value={name}
        autoFocus
        onChange={(e) => setName(e.target.value)}
        className="w-full rounded px-3 py-2 text-sm"
        style={{
          backgroundColor: 'var(--color-surface-2)',
          border: '1px solid var(--color-border)',
          color: 'var(--color-foreground)',
          outline: 'none',
        }}
      />
      <textarea
        placeholder="Description (optional)"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        rows={3}
        className="w-full rounded px-3 py-2 text-sm resize-y"
        style={{
          backgroundColor: 'var(--color-surface-2)',
          border: '1px solid var(--color-border)',
          color: 'var(--color-foreground)',
          outline: 'none',
        }}
      />
      {error && <p className="text-xs" style={{ color: '#f87171' }}>{error}</p>}
      <div className="flex items-center justify-end gap-2">
        <button
          onClick={onCancel}
          disabled={saving}
          className="flex items-center gap-1.5 rounded px-3 py-1.5 text-sm"
          style={{
            backgroundColor: 'var(--color-surface-2)',
            border: '1px solid var(--color-border)',
            color: 'var(--color-muted-foreground)',
            cursor: saving ? 'not-allowed' : 'pointer',
          }}
        >
          <X size={13} />
          Cancel
        </button>
        <button
          onClick={() => onSave(name.trim(), description)}
          disabled={saving || !name.trim()}
          className="flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium"
          style={{
            backgroundColor: 'var(--color-primary)',
            color: '#fff',
            border: 'none',
            cursor: saving || !name.trim() ? 'not-allowed' : 'pointer',
            opacity: saving || !name.trim() ? 0.6 : 1,
          }}
        >
          <Check size={13} />
          {saveLabel}
        </button>
      </div>
    </div>
  )
}

interface SubtaskNodeProps {
  subtask: Subtask
  // 1 = direct child of the task. Children render at depth + 1 until
  // MAX_SUBTASK_DEPTH, where the add-child control is hidden.
  depth: number
  onToggle: (sub: Subtask) => void
  onDelete: (sub: Subtask) => void
  onAddChild: (parent: Subtask, name: string) => void
}

function SubtaskNode({
  subtask, depth, onToggle, onDelete, onAddChild,
}: SubtaskNodeProps): React.ReactElement {
  const [open, setOpen] = useState(true)
  const [adding, setAdding] = useState(false)
  const [childName, setChildName] = useState('')
  const children = subtask.children ?? []
  const hasChildren = children.length > 0
  const canNest = depth < MAX_SUBTASK_DEPTH
  const progress = hasChildren ? countLeaves(children) : null

  function commitChild() {
    const name = childName.trim()
    if (!name) return
    onAddChild(subtask, name)
    setChildName('')
    setOpen(true)
  }

  return (
    <div>
      <div className="flex items-center gap-2 py-1">
        <button
          onClick={() => setOpen((v) => !v)}
          disabled={!hasChildren}
          className="rounded p-0.5"
          style={{
            color: 'var(--color-muted)',
            background: 'none',
            border: 'none',
            visibility: hasChildren ? 'visible' : 'hidden',
            cursor: 'pointer',
          }}
          title={open ? 'Collapse' : 'Expand'}
          aria-label={open ? 'Collapse' : 'Expand'}
        >
          {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </button>
        <input
          type="checkbox"
          checked={subtask.completed}
          onChange={() => onToggle(subtask)}
          className="h-4 w-4 cursor-pointer"
          style={{ accentColor: 'var(--color-primary)' }}
        />
        <span
          className="flex-1 text-sm"
          style={{
            color: subtask.completed ? 'var(--color-muted-foreground)' : 'var(--color-foreground)',
            textDecoration: subtask.completed ? 'line-through' : 'none',
          }}
        >
          {subtask.name}
        </span>
        {progress && (
          <span
            className="text-[10px] px-1.5 py-0.5 rounded"
            style={{
              backgroundColor: 'var(--color-surface-2)',
              color: 'var(--color-muted-foreground)',
            }}
          >
            {progress.done}/{progress.total}
          </span>
        )}
        {canNest && (
          <button
            onClick={() => setAdding((v) => !v)}
            className="rounded p-1 transition-colors hover:bg-(--color-surface-2)"
            style={{ color: 'var(--color-muted)' }}
            title="Add a sub-step"
          >
            <Plus size={12} />
          </button>
        )}
        <button
          onClick={() => onDelete(subtask)}
          className="rounded p-1 transition-colors hover:bg-(--color-surface-2)"
          style={{ color: 'var(--color-muted)' }}
          title={hasChildren ? 'Delete this step and everything under it' : 'Delete subtask'}
        >
          <Trash2 size={12} />
        </button>
      </div>
      {(hasChildren && open || adding) && (
        <div
          className="ml-2.5 pl-3"
          style={{ borderLeft: '1px solid var(--color-border)' }}
        >
          {open && children.map((c) => (
            <SubtaskNode
              key={c.id}
              subtask={c}
              depth={depth + 1}
              onToggle={onToggle}
              onDelete={onDelete}
              onAddChild={onAddChild}
            />
          ))}
          {adding && (
            <div className="my-1 flex items-center gap-2">
              <input
                type="text"
                autoFocus
                placeholder="Add a sub-step…"
                value={childName}
                onChange={(e) => setChildName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') commitChild()
                  if (e.key === 'Escape') { setAdding(false); setChildName('') }
                }}
                className="flex-1 rounded px-2 py-1 text-xs"
                style={{
                  backgroundColor: 'var(--color-surface-2)',
                  border: '1px solid var(--color-border)',
                  color: 'var(--color-foreground)',
                  outline: 'none',
                }}
              />
              <button
                onClick={commitChild}
                disabled={!childName.trim()}
                className="rounded px-2 py-1 text-xs font-medium"
                style={{
                  backgroundColor: 'var(--color-primary)',
                  color: '#fff',
                  border: 'none',
                  cursor: childName.trim() ? 'pointer' : 'not-allowed',
                  opacity: childName.trim() ? 1 : 0.6,
                }}
              >
                Add
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

interface TaskCardProps {
  task: CharacterTask
  expanded: boolean
  editing: boolean
  saving: boolean
  formError: string | null
  deleteConfirm: boolean
  onToggleExpanded: () => void
  onToggleComplete: () => void
  onStartEdit: () => void
  onSaveEdit: (name: string, description: string) => void
  onCancelEdit: () => void
  onRequestDelete: () => void
  onConfirmDelete: () => void
  onCancelDelete: () => void
  onAddSubtask: (parent: Subtask | null, name: string) => void
  onToggleSubtask: (subtask: Subtask) => void
  onDeleteSubtask: (subtask: Subtask) => void
  onExport: (withProgress: boolean) => void
}

function TaskCard(props: TaskCardProps): React.ReactElement {
  const {
    task, expanded, editing, saving, formError, deleteConfirm,
    onToggleExpanded, onToggleComplete, onStartEdit, onSaveEdit, onCancelEdit,
    onRequestDelete, onConfirmDelete, onCancelDelete,
    onAddSubtask, onToggleSubtask, onDeleteSubtask, onExport,
  } = props
  const [newSubtask, setNewSubtask] = useState('')
  // dnd-kit sortable: native HTML5 DnD was unreliable in Electron on Windows,
  // so task reordering is pointer-based. The grip button (below) carries the
  // drag listeners so the rest of the card stays interactive.
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
  })

  // Leaf-based so nested steps count once each, not as parent + children.
  const { done: completedSubs, total: totalSubs } = countLeaves(task.subtasks)

  if (editing) {
    return (
      <TaskForm
        initialName={task.name}
        initialDescription={task.description}
        saving={saving}
        error={formError}
        onSave={onSaveEdit}
        onCancel={onCancelEdit}
        saveLabel="Save"
      />
    )
  }

  return (
    <div
      ref={setNodeRef}
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.4 : 1,
      }}
    >
      <div
        className="rounded-lg overflow-hidden"
        style={{
          backgroundColor: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
        }}
      >
        <div className="flex items-start gap-2 px-3 py-3">
          <button
            {...attributes}
            {...listeners}
            className="mt-0.5 cursor-grab touch-none active:cursor-grabbing rounded p-1 hover:bg-(--color-surface-2)"
            style={{ color: 'var(--color-muted)' }}
            title="Drag to reorder"
            aria-label="Drag to reorder"
          >
            <GripVertical size={14} />
          </button>
          <input
            type="checkbox"
            checked={task.completed}
            onChange={onToggleComplete}
            className="mt-1 h-4 w-4 cursor-pointer"
            style={{ accentColor: 'var(--color-primary)' }}
          />
          <button
            onClick={onToggleExpanded}
            className="flex-1 min-w-0 text-left"
            style={{ background: 'none', border: 'none', padding: 0, cursor: 'pointer' }}
          >
            <div className="flex items-center gap-2">
              <span
                className="text-sm font-medium"
                style={{
                  color: task.completed ? 'var(--color-muted-foreground)' : 'var(--color-foreground)',
                  textDecoration: task.completed ? 'line-through' : 'none',
                }}
              >
                {task.name}
              </span>
              {totalSubs > 0 && (
                <span
                  className="text-xs px-1.5 py-0.5 rounded"
                  style={{
                    backgroundColor: 'var(--color-surface-2)',
                    color: 'var(--color-muted-foreground)',
                  }}
                >
                  {completedSubs}/{totalSubs}
                </span>
              )}
            </div>
            {task.description && (
              <p
                className="mt-0.5 text-xs whitespace-pre-wrap"
                style={{ color: 'var(--color-muted-foreground)' }}
              >
                {task.description}
              </p>
            )}
          </button>
          <button
            onClick={onToggleExpanded}
            className="rounded p-1 transition-colors hover:bg-(--color-surface-2)"
            style={{ color: 'var(--color-muted)' }}
            title={expanded ? 'Collapse' : 'Expand'}
          >
            {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>
          <button
            onClick={onStartEdit}
            className="rounded p-1 transition-colors hover:bg-(--color-surface-2)"
            style={{ color: 'var(--color-muted)' }}
            title="Edit"
          >
            <Pencil size={14} />
          </button>
          <button
            onClick={onRequestDelete}
            className="rounded p-1 transition-colors hover:bg-(--color-surface-2)"
            style={{ color: 'var(--color-muted)' }}
            title="Delete"
          >
            <Trash2 size={14} />
          </button>
        </div>

        {expanded && (
          <div
            className="px-4 pt-2 pb-3"
            style={{ borderTop: '1px solid var(--color-border)' }}
          >
            <p
              className="mb-2 text-xs font-semibold uppercase tracking-wide"
              style={{ color: 'var(--color-muted)' }}
            >
              Subtasks
            </p>
            {task.subtasks.length === 0 && (
              <p className="mb-2 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
                No subtasks yet.
              </p>
            )}
            <div className="space-y-0.5">
              {task.subtasks.map((sub) => (
                <SubtaskNode
                  key={sub.id}
                  subtask={sub}
                  depth={1}
                  onToggle={onToggleSubtask}
                  onDelete={onDeleteSubtask}
                  onAddChild={onAddSubtask}
                />
              ))}
            </div>
            <div className="mt-2 flex items-center gap-2">
              <input
                type="text"
                placeholder="Add a subtask…"
                value={newSubtask}
                onChange={(e) => setNewSubtask(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && newSubtask.trim()) {
                    onAddSubtask(null, newSubtask.trim())
                    setNewSubtask('')
                  }
                }}
                className="flex-1 rounded px-2.5 py-1.5 text-sm"
                style={{
                  backgroundColor: 'var(--color-surface-2)',
                  border: '1px solid var(--color-border)',
                  color: 'var(--color-foreground)',
                  outline: 'none',
                }}
              />
              <button
                onClick={() => {
                  if (newSubtask.trim()) {
                    onAddSubtask(null, newSubtask.trim())
                    setNewSubtask('')
                  }
                }}
                disabled={!newSubtask.trim()}
                className="flex items-center gap-1 rounded px-2.5 py-1.5 text-xs font-medium"
                style={{
                  backgroundColor: 'var(--color-primary)',
                  color: '#fff',
                  border: 'none',
                  cursor: newSubtask.trim() ? 'pointer' : 'not-allowed',
                  opacity: newSubtask.trim() ? 1 : 0.6,
                }}
              >
                <Plus size={11} />
                Add
              </button>
            </div>
            <div className="mt-3 flex items-center gap-3 text-xs" style={{ color: 'var(--color-muted)' }}>
              <span className="flex items-center gap-1"><Download size={11} /> Export</span>
              <button
                onClick={() => onExport(false)}
                className="underline-offset-2 hover:underline"
                style={{ background: 'none', border: 'none', padding: 0, cursor: 'pointer', color: 'var(--color-muted-foreground)' }}
                title="Save as a clean template (nothing checked) to share"
              >
                as template
              </button>
              <button
                onClick={() => onExport(true)}
                className="underline-offset-2 hover:underline"
                style={{ background: 'none', border: 'none', padding: 0, cursor: 'pointer', color: 'var(--color-muted-foreground)' }}
                title="Save including which steps are checked"
              >
                with progress
              </button>
            </div>
          </div>
        )}
      </div>

      {deleteConfirm && (
        <div
          className="mt-1 flex items-center justify-between rounded px-4 py-2 text-xs"
          style={{
            backgroundColor: 'color-mix(in srgb, #f87171 10%, var(--color-surface))',
            border: '1px solid color-mix(in srgb, #f87171 30%, transparent)',
          }}
        >
          <span style={{ color: 'var(--color-foreground)' }}>Delete "{task.name}"?</span>
          <div className="flex gap-2">
            <button
              onClick={onCancelDelete}
              className="rounded px-2 py-0.5"
              style={{
                backgroundColor: 'var(--color-surface-2)',
                border: '1px solid var(--color-border)',
                color: 'var(--color-muted-foreground)',
                cursor: 'pointer',
              }}
            >
              Cancel
            </button>
            <button
              onClick={onConfirmDelete}
              className="rounded px-2 py-0.5 font-medium"
              style={{ backgroundColor: '#ef4444', color: '#fff', border: 'none', cursor: 'pointer' }}
            >
              Delete
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

type Mode = 'idle' | 'creating' | { editing: number }

export default function CharacterTasksPage(): React.ReactElement {
  const { active: activeCharacter } = useActiveCharacter()
  const [viewedCharacter, setViewedCharacter] = useState('')
  // Default the viewed character to the active character once it's known.
  useEffect(() => {
    if (!viewedCharacter && activeCharacter) setViewedCharacter(activeCharacter)
  }, [activeCharacter, viewedCharacter])
  const [activeChar, setActiveChar] = useState<Character | null>(null)
  const [tasks, setTasks] = useState<CharacterTask[]>([])
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [mode, setMode] = useState<Mode>('idle')
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null)
  // A small activation distance lets the grip be clicked without starting a
  // drag; the keyboard sensor brings arrow-key reordering for free.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const load = useCallback(async () => {
    setLoadError(null)
    if (!viewedCharacter) { setActiveChar(null); setTasks([]); return }
    try {
      const charsResp = await listCharacters()
      const found = charsResp.characters.find(
        (c) => c.name.toLowerCase() === viewedCharacter.toLowerCase()
      ) ?? null
      setActiveChar(found)
      if (found) {
        const tasksResp = await listCharacterTasks(found.id)
        setTasks(tasksResp.tasks)
      } else {
        setTasks([])
      }
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to load tasks')
    }
  }, [viewedCharacter])

  useEffect(() => {
    setLoading(true)
    load().finally(() => setLoading(false))
  }, [load])

  const charID = activeChar?.id ?? null

  function toggleExpanded(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleCreate(name: string, description: string) {
    if (charID === null) return
    setSaving(true)
    setFormError(null)
    try {
      const created = await createCharacterTask(charID, { name, description, completed: false })
      setTasks((prev) => [...prev, { ...created, subtasks: created.subtasks ?? [] }])
      setMode('idle')
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : 'Failed to create task')
    } finally {
      setSaving(false)
    }
  }

  async function handleSaveEdit(taskID: number, name: string, description: string) {
    if (charID === null) return
    const existing = tasks.find((t) => t.id === taskID)
    if (!existing) return
    setSaving(true)
    setFormError(null)
    try {
      await updateCharacterTask(charID, taskID, { name, description, completed: existing.completed })
      setTasks((prev) => prev.map((t) => (t.id === taskID ? { ...t, name, description } : t)))
      setMode('idle')
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : 'Failed to update task')
    } finally {
      setSaving(false)
    }
  }

  async function handleToggleComplete(task: CharacterTask) {
    if (charID === null) return
    const next = !task.completed
    setTasks((prev) => prev.map((t) => (t.id === task.id ? { ...t, completed: next } : t)))
    try {
      await updateCharacterTask(charID, task.id, {
        name: task.name,
        description: task.description,
        completed: next,
      })
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to update task')
      setTasks((prev) => prev.map((t) => (t.id === task.id ? { ...t, completed: task.completed } : t)))
    }
  }

  async function handleDelete(taskID: number) {
    if (charID === null) return
    try {
      await deleteCharacterTask(charID, taskID)
      setTasks((prev) => prev.filter((t) => t.id !== taskID))
      setDeleteConfirm(null)
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to delete task')
    }
  }

  // Cascade and roll-up completion live on the server (a parent completes when
  // all its children do; checking a parent checks everything under it), so
  // subtask mutations just re-fetch rather than duplicate that logic here.
  async function refreshTasks() {
    if (charID === null) return
    const resp = await listCharacterTasks(charID)
    setTasks(resp.tasks)
  }

  async function handleAddSubtask(taskID: number, parent: Subtask | null, name: string) {
    if (charID === null) return
    try {
      await createCharacterSubtask(charID, taskID, {
        name,
        completed: false,
        parent_id: parent ? parent.id : null,
      })
      await refreshTasks()
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to add subtask')
    }
  }

  async function handleToggleSubtask(taskID: number, sub: Subtask) {
    if (charID === null) return
    try {
      await updateCharacterSubtask(charID, taskID, sub.id, { name: sub.name, completed: !sub.completed })
      await refreshTasks()
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to update subtask')
    }
  }

  async function handleDeleteSubtask(taskID: number, sub: Subtask) {
    if (charID === null) return
    try {
      await deleteCharacterSubtask(charID, taskID, sub.id)
      await refreshTasks()
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to delete subtask')
    }
  }

  async function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = '' // let the same file be picked again
    if (!file || charID === null) return
    setLoadError(null)
    try {
      const parsed = JSON.parse(await file.text())
      const created = await importCharacterTask(charID, parsed)
      await refreshTasks()
      setExpanded((prev) => new Set(prev).add(created.id))
    } catch (err: unknown) {
      setLoadError(
        err instanceof SyntaxError
          ? 'That file is not valid JSON.'
          : err instanceof Error ? err.message : 'Failed to import task',
      )
    }
  }

  async function commitReorder(orderedIDs: number[]) {
    if (charID === null) return
    try {
      await reorderCharacterTasks(charID, orderedIDs)
    } catch (err: unknown) {
      setLoadError(err instanceof Error ? err.message : 'Failed to reorder tasks')
      load()
    }
  }

  function handleDragEnd(e: DragEndEvent) {
    const { active, over } = e
    if (!over || active.id === over.id) return
    setTasks((prev) => {
      const sourceIdx = prev.findIndex((t) => t.id === active.id)
      const targetIdx = prev.findIndex((t) => t.id === over.id)
      if (sourceIdx === -1 || targetIdx === -1) return prev
      const next = arrayMove(prev, sourceIdx, targetIdx)
      void commitReorder(next.map((t) => t.id))
      return next
    })
  }

  const headerSubtitle = useMemo(() => {
    if (!viewedCharacter) return 'Select a character to track tasks.'
    if (!activeChar) return `No tracked character matches "${viewedCharacter}".`
    const completed = tasks.filter((t) => t.completed).length
    return `${activeChar.name} — ${completed}/${tasks.length} complete`
  }, [viewedCharacter, activeChar, tasks])

  return (
    <div className="flex h-full flex-col">
      <CharacterSubTabs
        value={viewedCharacter}
        onChange={setViewedCharacter}
      />
      <div className="flex-1 overflow-auto p-6">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <ListChecks size={20} style={{ color: 'var(--color-primary)' }} />
          <div>
            <h1 className="text-lg font-semibold" style={{ color: 'var(--color-foreground)' }}>
              Tasks
            </h1>
            <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
              {headerSubtitle}
            </p>
          </div>
        </div>
        {activeChar && mode === 'idle' && (
          <div className="flex items-center gap-2">
          <label
            className="flex cursor-pointer items-center gap-1.5 rounded px-3 py-1.5 text-sm"
            style={{
              backgroundColor: 'var(--color-surface-2)',
              border: '1px solid var(--color-border)',
              color: 'var(--color-muted-foreground)',
            }}
            title="Import a task file (.pqtask.json) shared by another player"
          >
            <Upload size={14} />
            Import
            <input
              type="file"
              accept=".json,application/json"
              onChange={handleImportFile}
              className="hidden"
            />
          </label>
          <button
            onClick={() => { setMode('creating'); setFormError(null) }}
            className="flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium"
            style={{
              backgroundColor: 'var(--color-primary)',
              color: '#fff',
              border: 'none',
              cursor: 'pointer',
            }}
          >
            <Plus size={14} />
            Add Task
          </button>
          </div>
        )}
      </div>

      {loadError && (
        <div
          className="mb-4 rounded px-4 py-3 text-sm"
          style={{
            backgroundColor: 'color-mix(in srgb, #f87171 12%, transparent)',
            border: '1px solid color-mix(in srgb, #f87171 30%, transparent)',
            color: '#f87171',
          }}
        >
          {loadError}
        </div>
      )}

      {mode === 'creating' && (
        <div className="mb-4">
          <TaskForm
            initialName=""
            initialDescription=""
            saving={saving}
            error={formError}
            onSave={handleCreate}
            onCancel={() => { setMode('idle'); setFormError(null) }}
            saveLabel="Add Task"
          />
        </div>
      )}

      {loading ? (
        <p className="text-sm" style={{ color: 'var(--color-muted-foreground)' }}>Loading…</p>
      ) : !activeChar ? (
        <div
          className="flex flex-col items-center justify-center rounded-lg py-12 text-center"
          style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
        >
          <ListChecks size={32} style={{ color: 'var(--color-muted)', marginBottom: '12px' }} />
          <p className="text-sm font-medium" style={{ color: 'var(--color-foreground)' }}>
            No active character
          </p>
          <p className="mt-1 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            Select a character on the Active Character tab to start tracking tasks.
          </p>
        </div>
      ) : tasks.length === 0 && mode !== 'creating' ? (
        <div
          className="flex flex-col items-center justify-center rounded-lg py-12 text-center"
          style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
        >
          <ListChecks size={32} style={{ color: 'var(--color-muted)', marginBottom: '12px' }} />
          <p className="text-sm font-medium" style={{ color: 'var(--color-foreground)' }}>
            No tasks yet
          </p>
          <p className="mt-1 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            Click Add Task to create your first to-do for {activeChar.name}.
          </p>
        </div>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          modifiers={[restrictToVerticalAxis]}
          onDragEnd={handleDragEnd}
        >
          <SortableContext
            items={tasks.map((t) => t.id)}
            strategy={verticalListSortingStrategy}
          >
            <div className="space-y-2">
              {tasks.map((task) => (
                <TaskCard
                  key={task.id}
                  task={task}
                  expanded={expanded.has(task.id)}
                  editing={typeof mode === 'object' && mode.editing === task.id}
                  saving={saving}
                  formError={formError}
                  deleteConfirm={deleteConfirm === task.id}
                  onToggleExpanded={() => toggleExpanded(task.id)}
                  onToggleComplete={() => handleToggleComplete(task)}
                  onStartEdit={() => { setMode({ editing: task.id }); setFormError(null) }}
                  onSaveEdit={(name, description) => handleSaveEdit(task.id, name, description)}
                  onCancelEdit={() => { setMode('idle'); setFormError(null) }}
                  onRequestDelete={() => setDeleteConfirm(task.id)}
                  onConfirmDelete={() => handleDelete(task.id)}
                  onCancelDelete={() => setDeleteConfirm(null)}
                  onAddSubtask={(parent, name) => handleAddSubtask(task.id, parent, name)}
                  onToggleSubtask={(sub) => handleToggleSubtask(task.id, sub)}
                  onDeleteSubtask={(sub) => handleDeleteSubtask(task.id, sub)}
                  onExport={(withProgress) => downloadTaskFile(task, withProgress)}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}
      </div>
    </div>
  )
}
