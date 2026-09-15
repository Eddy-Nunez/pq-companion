import React, { useMemo, useRef, useState } from 'react'
import { Upload, X, AlertTriangle, CheckCircle2, SkipForward, FileJson, Sparkles } from 'lucide-react'
import {
  previewRaidImport,
  commitRaidImport,
  provisionRaidRoles,
} from '../../services/api'
import type {
  RaidPack,
  RaidImportPreview,
  RaidImportPreviewItem,
  RaidImportCommitResult,
} from '../../types/raid'

interface RaidImportDialogProps {
  onClose: () => void
  // Called when the user finishes (or dismisses) the wizard so the editor can
  // reload encounters even after a partial import.
  onDone: () => void
}

interface ItemChoice {
  selected: boolean
  overwrite: boolean // only meaningful when the item already exists
}

// RaidImportDialog walks the trigger-import wizard's shape: pick a pack file,
// review the per-encounter preview (validation errors, conflict choice),
// commit the selected subset, then show the outcome. The backend never
// persists anything before commit.
export default function RaidImportDialog({
  onClose,
  onDone,
}: RaidImportDialogProps): React.ReactElement {
  const [preview, setPreview] = useState<RaidImportPreview | null>(null)
  // Per-item choice keyed by index in preview.encounters. Defaults mirror the
  // backend contract: selected when valid, conflicts default to skip.
  const [choices, setChoices] = useState<Record<number, ItemChoice>>({})
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [committing, setCommitting] = useState(false)
  const [commitError, setCommitError] = useState<string | null>(null)
  const [result, setResult] = useState<RaidImportCommitResult | null>(null)
  // Taxonomy paths provisioned (or already present) during this wizard run —
  // shared across items since provisioning is path-global.
  const [coveredPaths, setCoveredPaths] = useState<Set<string>>(new Set())
  const [provisioningItem, setProvisioningItem] = useState<number | null>(null)
  const [provisionError, setProvisionError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const hasErrors = (item: RaidImportPreviewItem): boolean =>
    !!item.errors && item.errors.length > 0

  const missingAll = (item: RaidImportPreviewItem): string[] =>
    item.missing_roles ?? []

  const effectiveMissing = (item: RaidImportPreviewItem): string[] =>
    missingAll(item).filter((p) => !coveredPaths.has(p))

  const importable = (item: RaidImportPreviewItem): boolean =>
    !hasErrors(item) && effectiveMissing(item).length === 0

  const selectedCount = useMemo(
    () =>
      preview
        ? preview.encounters.filter((it, i) => choices[i]?.selected && importable(it)).length
        : 0,
    [preview, choices, coveredPaths],
  )

  async function handleFile(file: File): Promise<void> {
    setLoadError(null)
    setLoading(true)
    setPreview(null)
    setResult(null)
    try {
      const text = await file.text()
      let parsed: unknown
      try {
        parsed = JSON.parse(text)
      } catch {
        throw new Error('Not a valid JSON file')
      }
      const p = await previewRaidImport(parsed as RaidPack)
      setPreview(p)
      const next: Record<number, ItemChoice> = {}
      p.encounters.forEach((item, i) => {
        next[i] = { selected: !hasErrors(item) && (item.missing_roles ?? []).length === 0, overwrite: false }
      })
      setChoices(next)
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  function setChoice(i: number, patch: Partial<ItemChoice>): void {
    setChoices((prev) => ({ ...prev, [i]: { ...prev[i], ...patch } }))
  }

  // handleProvision creates stub taxonomy rows for an item's missing comp
  // paths. Provisioning is idempotent and path-global: after success every
  // item whose missing paths are now covered unlocks (and auto-selects, so
  // the CTA flow ends with the encounter ready to import).
  async function handleProvision(i: number): Promise<void> {
    if (!preview) return
    const item = preview.encounters[i]
    const paths = effectiveMissing(item)
    if (paths.length === 0) return
    setProvisioningItem(i)
    setProvisionError(null)
    try {
      const res = await provisionRaidRoles(paths)
      const covered = new Set(coveredPaths)
      for (const p of [...res.created, ...res.present]) covered.add(p)
      setCoveredPaths(covered)
      // Unlock + auto-select every item this just made importable.
      setChoices((prev) => {
        const next = { ...prev }
        preview.encounters.forEach((it, j) => {
          const missing = (it.missing_roles ?? []).filter((p) => !covered.has(p))
          if (!hasErrors(it) && missing.length === 0 && next[j]) {
            next[j] = { ...next[j], selected: true }
          }
        })
        return next
      })
    } catch (err) {
      setProvisionError(err instanceof Error ? err.message : String(err))
    } finally {
      setProvisioningItem(null)
    }
  }

  async function handleCommit(): Promise<void> {
    if (!preview || selectedCount === 0) return
    setCommitting(true)
    setCommitError(null)
    try {
      const req = {
        pack_name: preview.pack_name,
        encounters: preview.encounters
          .map((item, i) => ({ item, choice: choices[i] }))
          .filter(({ choice }) => choice?.selected)
          .map(({ item, choice }) => ({
            encounter: item.encounter,
            overwrite: choice.overwrite,
          })),
      }
      setResult(await commitRaidImport(req))
    } catch (err) {
      setCommitError(err instanceof Error ? err.message : String(err))
    } finally {
      setCommitting(false)
    }
  }

  function finish(): void {
    onDone()
    onClose()
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ backgroundColor: 'rgba(0,0,0,0.6)' }}
      onClick={() => !committing && finish()}
    >
      <div
        className="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-lg"
        style={{
          backgroundColor: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-4 py-3"
          style={{ borderBottom: '1px solid var(--color-border)' }}
        >
          <div className="flex items-center gap-2">
            <Upload size={15} style={{ color: 'var(--color-accent)' }} />
            <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
              Import Raid Encounters
            </span>
            {preview && (
              <span
                className="rounded px-1.5 py-0.5 text-[11px] font-medium"
                style={{
                  backgroundColor: 'var(--color-surface-2)',
                  color: 'var(--color-muted-foreground)',
                  border: '1px solid var(--color-border)',
                }}
              >
                {preview.pack_name || 'pack'} · {preview.encounters.length} encounter
                {preview.encounters.length === 1 ? '' : 's'}
              </span>
            )}
          </div>
          <button onClick={finish} className="rounded p-1" style={{ color: 'var(--color-muted-foreground)' }}>
            <X size={15} />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-4 py-3">
          {loadError ? (
            <div
              className="rounded p-3 text-xs"
              style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-danger)' }}
            >
              {loadError}
            </div>
          ) : null}

          {!preview && !loadError && (
            <div className="flex flex-col items-center gap-3 py-8">
              <FileJson size={28} style={{ color: 'var(--color-muted-foreground)' }} />
              <p className="text-xs text-center max-w-sm" style={{ color: 'var(--color-muted-foreground)' }}>
                Choose a raid composition pack (.json) exported from PQ Companion. You&apos;ll review every
                encounter before anything is saved — encounters already in your knowledge base default to
                being skipped.
              </p>
              <button
                onClick={() => fileInputRef.current?.click()}
                disabled={loading}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded font-medium"
                style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
              >
                <Upload size={14} /> {loading ? 'Reading…' : 'Choose pack file'}
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".json,application/json"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) void handleFile(f)
                  e.target.value = ''
                }}
              />
            </div>
          )}

          {preview && !result && (
            <div className="flex flex-col gap-2">
              {preview.encounters.map((item, i) => {
                const choice = choices[i]
                const bad = hasErrors(item)
                return (
                  <div
                    key={`${item.encounter.id}-${i}`}
                    className="flex flex-col gap-1 rounded px-3 py-2"
                    style={{
                      backgroundColor: 'var(--color-surface-2)',
                      border: '1px solid var(--color-border)',
                      opacity: bad ? 0.75 : 1,
                    }}
                  >
                    <div className="flex items-center gap-2 flex-wrap">
                      <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--color-foreground)' }}>
                        <input
                          type="checkbox"
                          checked={!!choice?.selected}
                          disabled={bad || committing || effectiveMissing(item).length > 0}
                          onChange={(e) => setChoice(i, { selected: e.target.checked })}
                        />
                        {item.encounter.name}
                      </label>
                      <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
                        {item.encounter.id} · {item.encounter.zone}
                        {item.encounter.zone_id ? ` (${item.encounter.zone_id})` : ''}
                      </span>
                      {item.exists && (
                        <span
                          className="rounded px-1.5 py-0.5 text-[10px] font-medium"
                          style={{ backgroundColor: 'var(--color-surface)', color: 'var(--color-accent)', border: '1px solid var(--color-border)' }}
                        >
                          already exists
                        </span>
                      )}
                    </div>
                    {item.exists && !bad && (
                      <label className="flex items-center gap-1.5 text-[11px] pl-5" style={{ color: 'var(--color-muted-foreground)' }}>
                        On conflict:
                        <select
                          value={choice?.overwrite ? 'overwrite' : 'skip'}
                          disabled={committing || !choice?.selected}
                          onChange={(e) => setChoice(i, { overwrite: e.target.value === 'overwrite' })}
                          className="rounded px-1 py-0.5 text-[11px]"
                          style={{
                            backgroundColor: 'var(--color-surface)',
                            color: 'var(--color-foreground)',
                            border: '1px solid var(--color-border)',
                          }}
                        >
                          <option value="skip">Skip (keep mine)</option>
                          <option value="overwrite">Overwrite (use pack)</option>
                        </select>
                      </label>
                    )}
                    {missingAll(item).length > 0 && (
                      <div className="flex items-center gap-1.5 text-[11px] pl-5 flex-wrap">
                        <span className="flex items-start gap-1.5" style={{ color: 'var(--color-accent)' }}>
                          <AlertTriangle size={11} className="mt-0.5 shrink-0" />
                          <span>
                            Missing role{missingAll(item).length === 1 ? '' : 's'} not in your taxonomy:{' '}
                            <span className="font-medium">{missingAll(item).join(', ')}</span>
                            {effectiveMissing(item).length === 0
                              ? ' — created as stubs (no class mapping until you edit them)'
                              : ' — auto-provision to enable import; stubs have no class mapping until you edit them'}
                          </span>
                        </span>
                        <button
                          onClick={() => void handleProvision(i)}
                          disabled={provisioningItem !== null || committing}
                          className="flex items-center gap-1 px-2 py-0.5 text-[11px] rounded font-medium disabled:opacity-60"
                          style={
                            effectiveMissing(item).length === 0
                              ? { backgroundColor: 'var(--color-success)', color: '#fff' }
                              : { backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }
                          }
                        >
                          {effectiveMissing(item).length === 0 ? (
                            <>
                              <CheckCircle2 size={11} /> Roles created
                            </>
                          ) : (
                            <>
                              <Sparkles size={11} />{' '}
                              {provisioningItem === i ? 'Provisioning…' : 'Auto-provision'}
                            </>
                          )}
                        </button>
                      </div>
                    )}
                    {(item.errors ?? []).map((e) => (
                      <div key={e} className="flex items-start gap-1.5 text-[11px] pl-5" style={{ color: 'var(--color-danger)' }}>
                        <X size={11} className="mt-0.5 shrink-0" /> {e}
                      </div>
                    ))}
                    {(item.warnings ?? []).map((w) => (
                      <div key={w} className="flex items-start gap-1.5 text-[11px] pl-5" style={{ color: 'var(--color-accent)' }}>
                        <AlertTriangle size={11} className="mt-0.5 shrink-0" /> {w}
                      </div>
                    ))}
                  </div>
                )
              })}
            </div>
          )}

          {result && (
            <div className="flex flex-col gap-2 text-xs">
              <div className="flex items-center gap-2 font-medium" style={{ color: 'var(--color-foreground)' }}>
                <CheckCircle2 size={14} style={{ color: 'var(--color-success)' }} /> Import complete
              </div>
              {result.saved.length > 0 && (
                <div style={{ color: 'var(--color-success)' }}>Saved: {result.saved.join(', ')}</div>
              )}
              {result.skipped.length > 0 && (
                <div className="flex items-start gap-1.5" style={{ color: 'var(--color-muted-foreground)' }}>
                  <SkipForward size={11} className="mt-0.5 shrink-0" /> Skipped (already exist): {result.skipped.join(', ')}
                </div>
              )}
              {result.failed && Object.keys(result.failed).length > 0 && (
                <div
                  className="rounded p-2"
                  style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-danger)' }}
                >
                  {Object.entries(result.failed).map(([id, err]) => (
                    <div key={id}>
                      {id}: {err}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {commitError && (
            <div
              className="mt-2 rounded p-3 text-xs"
              style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-danger)' }}
            >
              {commitError}
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          className="flex items-center justify-end gap-2 px-4 py-3"
          style={{ borderTop: '1px solid var(--color-border)' }}
        >
          {result ? (
            <button
              onClick={finish}
              className="px-3 py-1.5 text-sm rounded font-medium"
              style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
            >
              Done
            </button>
          ) : (
            <>
              <button
                onClick={finish}
                disabled={committing}
                className="px-3 py-1.5 text-sm rounded"
                style={{ color: 'var(--color-muted-foreground)' }}
              >
                Cancel
              </button>
              <button
                onClick={() => void handleCommit()}
                disabled={committing || selectedCount === 0}
                className="px-3 py-1.5 text-sm rounded font-medium disabled:opacity-50"
                style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
              >
                {committing ? 'Importing…' : `Import ${selectedCount} encounter${selectedCount === 1 ? '' : 's'}`}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
