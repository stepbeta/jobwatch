// The JobWatch watch mark is shared by the header and the generated
// favicon. The shapes use explicit colours so the same geometry can be
// serialized into an SVG data URI as well as rendered inline.
export interface MarkColors {
  body: string
  face: string
  hand: string
}

export function brandMarkShapes({ body, face, hand }: MarkColors): string {
  return (
    `<rect x="13" y="2.5" width="6" height="3.5" rx="1.2" fill="${body}"/>` +
    `<rect x="14.5" y="5" width="3" height="3.5" fill="${body}"/>` +
    `<circle cx="16" cy="18" r="11" fill="${body}"/>` +
    `<circle cx="16" cy="18" r="8" fill="${face}"/>` +
    `<path d="M16 18V12.8M16 18l4.2 2.6" fill="none" stroke="${hand}" ` +
    `stroke-width="2" stroke-linecap="round"/>`
  )
}

export function brandMarkSvg(colors: MarkColors, extra = ''): string {
  return (
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">' +
    brandMarkShapes(colors) +
    extra +
    '</svg>'
  )
}
