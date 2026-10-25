export type CsvCellValue = string | number | null | undefined

// csvCell escapes spreadsheet control characters, quotes, and line breaks.
function csvCell(value: CsvCellValue) {
  let text = value == null ? '' : String(value)
  if (/^[=+\-@]/.test(text)) text = `'${text}`
  return `"${text.replaceAll('"', '""')}"`
}

// rowsToCsv serializes tabular values into a spreadsheet-safe CSV document.
export function rowsToCsv(rows: readonly (readonly CsvCellValue[])[]) {
  return rows.map((row) => row.map(csvCell).join(',')).join('\r\n')
}

// downloadCsv triggers a UTF-8 CSV download compatible with spreadsheets.
export function downloadCsv(csv: string, filename: string) {
  const blob = new Blob([`\uFEFF${csv}`], {
    type: 'text/csv;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
