import { exportPopFlags } from '../services/api'

function downloadJson(data: unknown, filename: string): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// downloadPopFlagsExport fetches the export envelope and saves it as a JSON
// file. With a character, only that character is exported (same envelope
// shape, one entry) and the filename carries their name.
export async function downloadPopFlagsExport(character?: string): Promise<void> {
  const data = await exportPopFlags(character ? [character] : undefined)
  const day = new Date().toISOString().slice(0, 10)
  const who = character ? `-${character.toLowerCase()}` : ''
  downloadJson(data, `pq-companion-popflags${who}-${day}.json`)
}
