import { brandMarkShapes } from '../brand'

// The header mark follows the active palette through CSS variables, while
// the generated favicon uses fixed colours (see favicon.ts).
export function BrandMark({ size = 20 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      aria-hidden="true"
      focusable="false"
      dangerouslySetInnerHTML={{
        __html: brandMarkShapes({
          body: 'var(--accent)',
          face: 'var(--panel)',
          hand: 'var(--accent)',
        }),
      }}
    />
  )
}
