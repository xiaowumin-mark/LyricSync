export function formatMs(value) {
  const ms = Number(value || 0)
  const total = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(total / 60)
  const seconds = String(total % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
}

export function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString()
}

export function percent(value) {
  const next = Number(value || 0)
  return Math.round(Math.min(1, Math.max(0, next)) * 100)
}

export function audioPercent(value) {
  const next = Math.min(1, Math.max(0, Number(value || 0)))
  if (!next) return 0
  return Math.round((1 - Math.exp(-next * 9)) * 100)
}

export function serviceColor(status) {
  if (status === 'running') return 'success'
  if (status === 'error') return 'error'
  if (status === 'disabled') return 'grey'
  return 'warning'
}

export function playbackColor(state) {
  if (state === 'playing') return 'success'
  if (state === 'paused') return 'warning'
  return 'grey'
}
