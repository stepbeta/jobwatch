import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// React 19 requires the act environment flag for testing-library.
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

// jsdom does not implement scrollIntoView; the timeline cue uses it.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}

afterEach(() => {
  cleanup()
  window.localStorage.clear()
})
