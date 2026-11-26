// The JobWatch radar mark is shared by the header and the generated
// favicon. The shapes use explicit colours so the same geometry can be
// serialized into an SVG data URI as well as rendered inline.
export interface MarkColors {
  body: string
  face: string
  hand: string
}

export function brandMarkShapes({ body, face, hand }: MarkColors): string {
  return (
    `<circle cx="16" cy="16" r="11.5" fill="${body}"/>` +
    `<circle cx="16" cy="16" r="8.5" fill="${face}"/>` +
    `<circle cx="16" cy="16" r="5.5" fill="none" stroke="${hand}" ` +
    `stroke-width="1.3" opacity="0.55"/>` +
    `<circle cx="16" cy="16" r="2.8" fill="none" stroke="${hand}" ` +
    `stroke-width="1.3" opacity="0.55"/>` +
    `<path d="M16 16 22.4 9.6" fill="none" stroke="${hand}" ` +
    `stroke-width="2" stroke-linecap="round"/>` +
    `<circle cx="22.4" cy="9.6" r="1.8" fill="${hand}"/>` +
    `<circle cx="16" cy="16" r="1.5" fill="${hand}"/>`
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
