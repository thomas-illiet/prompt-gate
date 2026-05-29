export interface AppRowAction<TItem = unknown> {
  color?: string
  disabled?: boolean | ((item: TItem) => boolean)
  icon: string
  key: string
  onSelect?: (item: TItem) => void
  title: string | ((item: TItem) => string)
}

export interface AppRowActionSelectContext<TItem = unknown> {
  action: AppRowAction<TItem>
  item?: TItem
}
