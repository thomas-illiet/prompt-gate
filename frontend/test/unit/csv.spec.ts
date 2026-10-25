import { afterEach, describe, expect, it, vi } from 'vitest'

import { downloadCsv, rowsToCsv } from '../../app/utils/csv'

describe('CSV utilities', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('quotes values and protects spreadsheet formulas', () => {
    expect(
      rowsToCsv([
        ['Name', 'Value'],
        ['Lovelace, "Ada"', '=unsafe'],
        ['Line\nbreak', null],
      ]),
    ).toBe(
      '"Name","Value"\r\n"Lovelace, ""Ada""","\'=unsafe"\r\n"Line\nbreak",""',
    )
  })

  it('downloads UTF-8 CSV content with a BOM and revokes its object URL', async () => {
    let capturedBlob: Blob | null = null
    const click = vi.fn()
    const revokeObjectURL = vi.fn()

    vi.stubGlobal('URL', {
      createObjectURL: vi.fn((blob: Blob) => {
        capturedBlob = blob
        return 'blob:csv'
      }),
      revokeObjectURL,
    })
    vi.stubGlobal('document', {
      createElement: vi.fn(() => ({
        click,
        download: '',
        href: '',
      })),
    })

    downloadCsv('"Date"\r\n"2026-10-25"', 'daily.csv')

    expect(capturedBlob).not.toBeNull()
    const bytes = new Uint8Array(await capturedBlob!.arrayBuffer())
    expect([...bytes.slice(0, 3)]).toEqual([0xef, 0xbb, 0xbf])
    expect(new TextDecoder().decode(bytes.slice(3))).toBe(
      '"Date"\r\n"2026-10-25"',
    )
    expect(capturedBlob!.type).toBe('text/csv;charset=utf-8')
    expect(click).toHaveBeenCalledOnce()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:csv')
  })
})
