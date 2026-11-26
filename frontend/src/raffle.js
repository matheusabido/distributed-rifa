const currency = new Intl.NumberFormat('pt-BR', {
  style: 'currency',
  currency: 'BRL',
})

export const raffleStatuses = {
  draft: 'Rascunho',
  open: 'Aberta',
  closed: 'Aguardando sorteio',
  drawn: 'Sorteada',
}

export function formatCurrency(cents) {
  return currency.format(cents / 100)
}

export function formatNumber(number) {
  return String(number).padStart(2, '0')
}
