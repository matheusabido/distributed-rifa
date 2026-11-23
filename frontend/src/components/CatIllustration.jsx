export default function CatIllustration() {
  return (
    <svg
      className="cat-illustration"
      viewBox="0 0 420 420"
      role="img"
      aria-label="Gatinho azul sorrindo"
    >
      <defs>
        <linearGradient id="catBody" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#dff3ff" />
          <stop offset="1" stopColor="#9fd7f5" />
        </linearGradient>
        <linearGradient id="catShirt" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#1170df" />
          <stop offset="1" stopColor="#084daa" />
        </linearGradient>
      </defs>
      <ellipse cx="210" cy="363" rx="126" ry="25" fill="#0b55ad" opacity=".12" />
      <path
        d="M120 197 87 91l90 57m123 49 33-106-90 57"
        fill="url(#catBody)"
        stroke="#173e69"
        strokeWidth="8"
        strokeLinejoin="round"
      />
      <path d="m106 118 45 29-34 20Z" fill="#f3a9b9" opacity=".75" />
      <path d="m314 118-45 29 34 20Z" fill="#f3a9b9" opacity=".75" />
      <path
        d="M111 210c0-74 44-114 99-114s99 40 99 114c0 68-38 111-99 111s-99-43-99-111Z"
        fill="url(#catBody)"
        stroke="#173e69"
        strokeWidth="8"
      />
      <path
        d="M129 273c30-25 132-25 162 0l-8 79H137Z"
        fill="url(#catShirt)"
        stroke="#173e69"
        strokeWidth="8"
      />
      <path
        d="M156 187c8-9 21-9 29 0m50 0c8-9 21-9 29 0"
        fill="none"
        stroke="#173e69"
        strokeWidth="8"
        strokeLinecap="round"
      />
      <path d="m201 218 9 7 9-7" fill="#e88da2" stroke="#173e69" strokeWidth="5" strokeLinejoin="round" />
      <path
        d="M210 226c0 15-24 16-29 5m29-5c0 15 24 16 29 5"
        fill="none"
        stroke="#173e69"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <path
        d="M145 222 83 207m63 34-67 7m196-26 62-15m-63 34 67 7"
        fill="none"
        stroke="#173e69"
        strokeWidth="4"
        strokeLinecap="round"
        opacity=".65"
      />
      <path d="M188 302h44l-4 32h-36Z" fill="#fff" opacity=".95" />
      <path d="M210 309v18m-9-9h18" stroke="#e14968" strokeWidth="5" strokeLinecap="round" />
    </svg>
  )
}
