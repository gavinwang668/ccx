export function maskApiKey(key: string): string {
  if (!key) return ''

  if (key.length <= 8) {
    return '***'
  }

  if (key.length <= 12) {
    return `${key.slice(0, 3)}***${key.slice(-3)}`
  }

  return `${key.slice(0, 6)}***${key.slice(-3)}`
}
