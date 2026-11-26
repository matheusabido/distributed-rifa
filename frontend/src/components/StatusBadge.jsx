import { raffleStatuses } from '../raffle.js'

export default function StatusBadge({ status }) {
  return (
    <span className={`raffle-status raffle-status--${status}`}>
      {raffleStatuses[status] || status}
    </span>
  )
}
