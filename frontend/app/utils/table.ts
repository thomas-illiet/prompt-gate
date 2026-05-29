import type { DataTableHeader } from 'vuetify'

export type AppTableSortDir = 'asc' | 'desc'

export const appTableItemsPerPageOptions = [10, 25, 50, 100] as const

export const appTableCenteredColumnProps = { class: 'text-center' } as const

// appTableCenteredColumn applies consistent center alignment to a data table column.
export function appTableCenteredColumn(
  header: DataTableHeader,
): DataTableHeader {
  return {
    ...header,
    align: 'center',
    cellProps: appTableCenteredColumnProps,
    headerProps: appTableCenteredColumnProps,
  }
}
