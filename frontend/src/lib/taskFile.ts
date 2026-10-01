import type { CharacterTask, Subtask, TaskFile, TaskFileStep } from '../services/api'

// Leaves of a subtask tree (nodes with no children) — the units progress is
// counted in, so a parent never double-counts its own checkbox.
export function countLeaves(subs: Subtask[]): { done: number; total: number } {
  let done = 0
  let total = 0
  for (const s of subs) {
    if (s.children && s.children.length > 0) {
      const c = countLeaves(s.children)
      done += c.done
      total += c.total
    } else {
      total += 1
      if (s.completed) done += 1
    }
  }
  return { done, total }
}

function toStep(s: Subtask, withProgress: boolean): TaskFileStep {
  const step: TaskFileStep = { name: s.name }
  if (s.children && s.children.length > 0) {
    step.steps = s.children.map((c) => toStep(c, withProgress))
  } else if (withProgress && s.completed) {
    step.completed = true
  }
  return step
}

// Builds the shareable file for a task. Without progress it is a clean
// template: nothing checked, so whoever imports it starts fresh.
export function buildTaskFile(task: CharacterTask, withProgress: boolean): TaskFile {
  const file: TaskFile = {
    format: 'pq-companion-task',
    version: 1,
    task: {
      name: task.name,
      description: task.description,
      steps: task.subtasks.map((s) => toStep(s, withProgress)),
    },
  }
  if (withProgress && task.completed) file.task.completed = true
  return file
}

export function taskFileName(task: CharacterTask): string {
  const base = task.name.replace(/[^a-z0-9]+/gi, '-').replace(/^-+|-+$/g, '').toLowerCase()
  return `${base || 'task'}.pqtask.json`
}

export function downloadTaskFile(task: CharacterTask, withProgress: boolean): void {
  const blob = new Blob([JSON.stringify(buildTaskFile(task, withProgress), null, 2)], {
    type: 'application/json',
  })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = taskFileName(task)
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
