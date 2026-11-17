import type { ReactNode } from 'react'

// Markdown is a deliberately small renderer for interview answers. It
// never uses dangerouslySetInnerHTML: every piece of answer text becomes
// a React text node, so raw HTML in an answer is displayed literally and
// cannot execute. Supported syntax (and nothing more): ATX headings,
// unordered and ordered lists, fenced code blocks, inline code, bold and
// italic.

const INLINE = /(`[^`\n]+`|\*\*[^*\n]+\*\*|\*[^*\n]+\*|_[^_\n]+_)/g

const HEADING = /^\s*(#{1,6})\s+(.*)$/
const FENCE = /^\s*(`{3,}|~{3,})(.*)$/
const BULLET = /^\s*[-*+]\s+(.*)$/
const ORDERED = /^\s*\d+[.)]\s+(.*)$/

function inline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = []
  let last = 0
  let n = 0
  for (const match of text.matchAll(INLINE)) {
    const start = match.index ?? 0
    if (start > last) nodes.push(text.slice(last, start))
    const token = match[0]
    const key = `${keyPrefix}-${n++}`
    if (token.startsWith('`')) {
      nodes.push(<code key={key}>{token.slice(1, -1)}</code>)
    } else if (token.startsWith('**')) {
      nodes.push(<strong key={key}>{inline(token.slice(2, -2), key)}</strong>)
    } else {
      nodes.push(<em key={key}>{inline(token.slice(1, -1), key)}</em>)
    }
    last = start + token.length
  }
  if (last < text.length) nodes.push(text.slice(last))
  return nodes
}

function isBlockStart(line: string): boolean {
  return FENCE.test(line) || HEADING.test(line) || BULLET.test(line) || ORDERED.test(line)
}

export function Markdown({ source }: { source: string }) {
  const lines = source.replace(/\r\n?/g, '\n').split('\n')
  const blocks: ReactNode[] = []
  let i = 0
  let key = 0
  const nextKey = () => `md-${key++}`

  while (i < lines.length) {
    const line = lines[i]
    if (line.trim() === '') {
      i++
      continue
    }

    const fence = line.match(FENCE)
    if (fence) {
      const marker = fence[1]
      const lang = fence[2].trim()
      const code: string[] = []
      i++
      while (i < lines.length && !lines[i].trimStart().startsWith(marker)) {
        code.push(lines[i])
        i++
      }
      if (i < lines.length) i++
      blocks.push(
        <pre key={nextKey()}>
          <code className={lang ? `language-${lang}` : undefined}>{code.join('\n')}</code>
        </pre>,
      )
      continue
    }

    const heading = line.match(HEADING)
    if (heading) {
      const Tag = `h${heading[1].length}` as 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6'
      blocks.push(<Tag key={nextKey()}>{inline(heading[2], nextKey())}</Tag>)
      i++
      continue
    }

    const ordered = line.match(ORDERED)
    const bullet = line.match(BULLET)
    if (ordered || bullet) {
      const matcher = ordered ? ORDERED : BULLET
      const items: string[] = []
      while (i < lines.length) {
        const m = lines[i].match(matcher)
        if (!m) break
        items.push(m[1])
        i++
      }
      const listKey = nextKey()
      const children = items.map((item, n) => (
        <li key={`${listKey}-${n}`}>{inline(item, `${listKey}-${n}`)}</li>
      ))
      blocks.push(ordered ? <ol key={listKey}>{children}</ol> : <ul key={listKey}>{children}</ul>)
      continue
    }

    const paragraph: string[] = []
    while (i < lines.length && !isBlockStart(lines[i]) && lines[i].trim() !== '') {
      paragraph.push(lines[i].trim())
      i++
    }
    const paragraphKey = nextKey()
    blocks.push(<p key={paragraphKey}>{inline(paragraph.join(' '), paragraphKey)}</p>)
  }

  return <div className="markdown">{blocks}</div>
}
