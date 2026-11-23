export const NUMBER_COUNT = 100
export const PRICE = 5

export const reservedNumbers = new Set([
  3, 7, 12, 18, 24, 31, 36, 42, 47, 55, 63, 68, 74, 81, 89, 94,
])

export function formatNumber(number) {
  return String(number).padStart(2, '0')
}

export function formatCurrency(value) {
  return `R$ ${value.toFixed(2).replace('.', ',')}`
}

export function formatSelectionCount(count) {
  return `${count} ${count === 1 ? 'número' : 'números'}`
}
