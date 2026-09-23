// Icon names the platform uses. Module manifests and views name icons with the
// Material Design Icons convention ("mdi-laptop"); UiIcon maps a name to the
// Iconify Tailwind class ("icon-[mdi--laptop]"). Tailwind only emits CSS for
// class strings it can see at build time, so every icon that may be named at
// runtime (manifest nav entries!) MUST be listed here — the list doubles as the
// safelist scanned through @source. Add a name here when a manifest or view
// starts using a new icon.

export const ICONS = [
  'mdi-account-arrow-left', // icon-[mdi--account-arrow-left]
  'mdi-account-arrow-right', // icon-[mdi--account-arrow-right]
  'mdi-account-circle-outline', // icon-[mdi--account-circle-outline]
  'mdi-account-group-outline', // icon-[mdi--account-group-outline]
  'mdi-account-key-outline', // icon-[mdi--account-key-outline]
  'mdi-account-multiple-outline', // icon-[mdi--account-multiple-outline]
  'mdi-account-network-outline', // icon-[mdi--account-network-outline]
  'mdi-account-plus', // icon-[mdi--account-plus]
  'mdi-account-plus-outline', // icon-[mdi--account-plus-outline]
  'mdi-alert-circle-outline', // icon-[mdi--alert-circle-outline]
  'mdi-alert-outline', // icon-[mdi--alert-outline]
  'mdi-application-brackets-outline', // icon-[mdi--application-brackets-outline]
  'mdi-arrow-left', // icon-[mdi--arrow-left]
  'mdi-arrow-right', // icon-[mdi--arrow-right]
  'mdi-autorenew', // icon-[mdi--autorenew]
  'mdi-bell-outline', // icon-[mdi--bell-outline]
  'mdi-broadcast', // icon-[mdi--broadcast]
  'mdi-calendar-alert', // icon-[mdi--calendar-alert]
  'mdi-call-split', // icon-[mdi--call-split]
  'mdi-camera', // icon-[mdi--camera]
  'mdi-camera-outline', // icon-[mdi--camera-outline]
  'mdi-cancel', // icon-[mdi--cancel]
  'mdi-cash', // icon-[mdi--cash]
  'mdi-certificate-outline', // icon-[mdi--certificate-outline]
  'mdi-check', // icon-[mdi--check]
  'mdi-check-circle-outline', // icon-[mdi--check-circle-outline]
  'mdi-check-decagram', // icon-[mdi--check-decagram]
  'mdi-chevron-down', // icon-[mdi--chevron-down]
  'mdi-chevron-left', // icon-[mdi--chevron-left]
  'mdi-chevron-right', // icon-[mdi--chevron-right]
  'mdi-chevron-up', // icon-[mdi--chevron-up]
  'mdi-circle', // icon-[mdi--circle]
  'mdi-circle-small', // icon-[mdi--circle-small]
  'mdi-city', // icon-[mdi--city]
  'mdi-clipboard-check-outline', // icon-[mdi--clipboard-check-outline]
  'mdi-clipboard-list-outline', // icon-[mdi--clipboard-list-outline]
  'mdi-clipboard-text-clock-outline', // icon-[mdi--clipboard-text-clock-outline]
  'mdi-clock-alert-outline', // icon-[mdi--clock-alert-outline]
  'mdi-clock-outline', // icon-[mdi--clock-outline]
  'mdi-close', // icon-[mdi--close]
  'mdi-cloud-off-outline', // icon-[mdi--cloud-off-outline]
  'mdi-cog-outline', // icon-[mdi--cog-outline]
  'mdi-console', // icon-[mdi--console]
  'mdi-content-copy', // icon-[mdi--content-copy]
  'mdi-cpu-64-bit', // icon-[mdi--cpu-64-bit]
  'mdi-database-arrow-up-outline', // icon-[mdi--database-arrow-up-outline]
  'mdi-database-outline', // icon-[mdi--database-outline]
  'mdi-delete', // icon-[mdi--delete]
  'mdi-delete-outline', // icon-[mdi--delete-outline]
  'mdi-desktop-classic', // icon-[mdi--desktop-classic]
  'mdi-devices', // icon-[mdi--devices]
  'mdi-dice-multiple-outline', // icon-[mdi--dice-multiple-outline]
  'mdi-domain', // icon-[mdi--domain]
  'mdi-domain-plus', // icon-[mdi--domain-plus]
  'mdi-door', // icon-[mdi--door]
  'mdi-dots-vertical', // icon-[mdi--dots-vertical]
  'mdi-download', // icon-[mdi--download]
  'mdi-drag', // icon-[mdi--drag]
  'mdi-earth', // icon-[mdi--earth]
  'mdi-email-open-outline', // icon-[mdi--email-open-outline]
  'mdi-email-outline', // icon-[mdi--email-outline]
  'mdi-eye-off-outline', // icon-[mdi--eye-off-outline]
  'mdi-eye-outline', // icon-[mdi--eye-outline]
  'mdi-file-certificate-outline', // icon-[mdi--file-certificate-outline]
  'mdi-file-cog-outline', // icon-[mdi--file-cog-outline]
  'mdi-file-document-edit-outline', // icon-[mdi--file-document-edit-outline]
  'mdi-file-document-multiple-outline', // icon-[mdi--file-document-multiple-outline]
  'mdi-file-document-outline', // icon-[mdi--file-document-outline]
  'mdi-file-search-outline', // icon-[mdi--file-search-outline]
  'mdi-file-upload-outline', // icon-[mdi--file-upload-outline]
  'mdi-filter-outline', // icon-[mdi--filter-outline]
  'mdi-flag', // icon-[mdi--flag]
  'mdi-folder', // icon-[mdi--folder]
  'mdi-folder-move-outline', // icon-[mdi--folder-move-outline]
  'mdi-folder-open-outline', // icon-[mdi--folder-open-outline]
  'mdi-folder-outline', // icon-[mdi--folder-outline]
  'mdi-folder-plus-outline', // icon-[mdi--folder-plus-outline]
  'mdi-group', // icon-[mdi--group]
  'mdi-harddisk', // icon-[mdi--harddisk]
  'mdi-history', // icon-[mdi--history]
  'mdi-home-outline', // icon-[mdi--home-outline]
  'mdi-inbox-outline', // icon-[mdi--inbox-outline]
  'mdi-information-outline', // icon-[mdi--information-outline]
  'mdi-ip', // icon-[mdi--ip]
  'mdi-ip-network', // icon-[mdi--ip-network]
  'mdi-ip-network-outline', // icon-[mdi--ip-network-outline]
  'mdi-key', // icon-[mdi--key]
  'mdi-key-outline', // icon-[mdi--key-outline]
  'mdi-key-plus', // icon-[mdi--key-plus]
  'mdi-key-variant', // icon-[mdi--key-variant]
  'mdi-lan', // icon-[mdi--lan]
  'mdi-lan-connect', // icon-[mdi--lan-connect]
  'mdi-lan-pending', // icon-[mdi--lan-pending]
  'mdi-laptop', // icon-[mdi--laptop]
  'mdi-layers', // icon-[mdi--layers]
  'mdi-license', // icon-[mdi--license]
  'mdi-lightbulb-on-outline', // icon-[mdi--lightbulb-on-outline]
  'mdi-link-variant', // icon-[mdi--link-variant]
  'mdi-lock-open-outline', // icon-[mdi--lock-open-outline]
  'mdi-lock-outline', // icon-[mdi--lock-outline]
  'mdi-logout', // icon-[mdi--logout]
  'mdi-magnify', // icon-[mdi--magnify]
  'mdi-magnify-scan', // icon-[mdi--magnify-scan]
  'mdi-map-marker', // icon-[mdi--map-marker]
  'mdi-map-marker-outline', // icon-[mdi--map-marker-outline]
  'mdi-memory', // icon-[mdi--memory]
  'mdi-menu', // icon-[mdi--menu]
  'mdi-message-text-outline', // icon-[mdi--message-text-outline]
  'mdi-monitor-dashboard', // icon-[mdi--monitor-dashboard]
  'mdi-monitor-multiple', // icon-[mdi--monitor-multiple]
  'mdi-office-building', // icon-[mdi--office-building]
  'mdi-open-in-new', // icon-[mdi--open-in-new]
  'mdi-package-down', // icon-[mdi--package-down]
  'mdi-package-variant', // icon-[mdi--package-variant]
  'mdi-package-variant-closed', // icon-[mdi--package-variant-closed]
  'mdi-paperclip', // icon-[mdi--paperclip]
  'mdi-pencil', // icon-[mdi--pencil]
  'mdi-pencil-outline', // icon-[mdi--pencil-outline]
  'mdi-plus', // icon-[mdi--plus]
  'mdi-plus-box-multiple', // icon-[mdi--plus-box-multiple]
  'mdi-plus-box-outline', // icon-[mdi--plus-box-outline]
  'mdi-power', // icon-[mdi--power]
  'mdi-power-off', // icon-[mdi--power-off]
  'mdi-progress-clock', // icon-[mdi--progress-clock]
  'mdi-pulse', // icon-[mdi--pulse]
  'mdi-radar', // icon-[mdi--radar]
  'mdi-radio-tower', // icon-[mdi--radio-tower]
  'mdi-refresh', // icon-[mdi--refresh]
  'mdi-reload', // icon-[mdi--reload]
  'mdi-restart', // icon-[mdi--restart]
  'mdi-restart-alert', // icon-[mdi--restart-alert]
  'mdi-send-check-outline', // icon-[mdi--send-check-outline]
  'mdi-send-outline', // icon-[mdi--send-outline]
  'mdi-server', // icon-[mdi--server]
  'mdi-server-network', // icon-[mdi--server-network]
  'mdi-shape-outline', // icon-[mdi--shape-outline]
  'mdi-shield-account', // icon-[mdi--shield-account]
  'mdi-shield-account-outline', // icon-[mdi--shield-account-outline]
  'mdi-shield-alert-outline', // icon-[mdi--shield-alert-outline]
  'mdi-shield-check', // icon-[mdi--shield-check]
  'mdi-shield-check-outline', // icon-[mdi--shield-check-outline]
  'mdi-shield-half-full', // icon-[mdi--shield-half-full]
  'mdi-shield-key-outline', // icon-[mdi--shield-key-outline]
  'mdi-shield-off-outline', // icon-[mdi--shield-off-outline]
  'mdi-sort', // icon-[mdi--sort]
  'mdi-source-branch', // icon-[mdi--source-branch]
  'mdi-star', // icon-[mdi--star]
  'mdi-subdirectory-arrow-right', // icon-[mdi--subdirectory-arrow-right]
  'mdi-sync', // icon-[mdi--sync]
  'mdi-tag-multiple-outline', // icon-[mdi--tag-multiple-outline]
  'mdi-target', // icon-[mdi--target]
  'mdi-theme-light-dark', // icon-[mdi--theme-light-dark]
  'mdi-tray', // icon-[mdi--tray]
  'mdi-tray-remove', // icon-[mdi--tray-remove]
  'mdi-trending-down', // icon-[mdi--trending-down]
  'mdi-truck-outline', // icon-[mdi--truck-outline]
  'mdi-upload', // icon-[mdi--upload]
  'mdi-view-dashboard-outline', // icon-[mdi--view-dashboard-outline]
  'mdi-view-module-outline', // icon-[mdi--view-module-outline]
  'mdi-weather-night', // icon-[mdi--weather-night]
  'mdi-weather-sunny', // icon-[mdi--weather-sunny]
  'mdi-white-balance-sunny', // icon-[mdi--white-balance-sunny]
] as const

export type IconName = (typeof ICONS)[number] | (string & {})

const known: ReadonlySet<string> = new Set(ICONS)

/** Maps an "mdi-*" name to its Iconify Tailwind class; unknown names fall back to a generic glyph. */
export function iconClass(name: string | undefined): string {
  if (!name) return 'icon-[mdi--circle-outline]'
  const n = name.startsWith('mdi-') ? name : 'mdi-' + name
  if (!known.has(n) && import.meta.env?.DEV) console.warn('[freya/ui] icon not in safelist:', n)
  return 'icon-[mdi--' + n.slice(4) + ']'
}

/** Safelist literal so Tailwind scans every class in this file. */
export const ICON_CLASS_SAFELIST = [
  'icon-[mdi--account-arrow-left]',
  'icon-[mdi--account-arrow-right]',
  'icon-[mdi--account-circle-outline]',
  'icon-[mdi--account-group-outline]',
  'icon-[mdi--account-key-outline]',
  'icon-[mdi--account-multiple-outline]',
  'icon-[mdi--account-network-outline]',
  'icon-[mdi--account-plus]',
  'icon-[mdi--account-plus-outline]',
  'icon-[mdi--alert-circle-outline]',
  'icon-[mdi--alert-outline]',
  'icon-[mdi--application-brackets-outline]',
  'icon-[mdi--arrow-left]',
  'icon-[mdi--arrow-right]',
  'icon-[mdi--autorenew]',
  'icon-[mdi--bell-outline]',
  'icon-[mdi--broadcast]',
  'icon-[mdi--calendar-alert]',
  'icon-[mdi--camera]',
  'icon-[mdi--camera-outline]',
  'icon-[mdi--cancel]',
  'icon-[mdi--cash]',
  'icon-[mdi--certificate-outline]',
  'icon-[mdi--check]',
  'icon-[mdi--check-circle-outline]',
  'icon-[mdi--check-decagram]',
  'icon-[mdi--chevron-down]',
  'icon-[mdi--chevron-left]',
  'icon-[mdi--chevron-right]',
  'icon-[mdi--chevron-up]',
  'icon-[mdi--circle]',
  'icon-[mdi--circle-small]',
  'icon-[mdi--city]',
  'icon-[mdi--clipboard-check-outline]',
  'icon-[mdi--clipboard-list-outline]',
  'icon-[mdi--clipboard-text-clock-outline]',
  'icon-[mdi--clock-alert-outline]',
  'icon-[mdi--clock-outline]',
  'icon-[mdi--close]',
  'icon-[mdi--cloud-off-outline]',
  'icon-[mdi--cog-outline]',
  'icon-[mdi--console]',
  'icon-[mdi--content-copy]',
  'icon-[mdi--cpu-64-bit]',
  'icon-[mdi--database-arrow-up-outline]',
  'icon-[mdi--database-outline]',
  'icon-[mdi--delete]',
  'icon-[mdi--delete-outline]',
  'icon-[mdi--desktop-classic]',
  'icon-[mdi--devices]',
  'icon-[mdi--dice-multiple-outline]',
  'icon-[mdi--domain]',
  'icon-[mdi--domain-plus]',
  'icon-[mdi--door]',
  'icon-[mdi--dots-vertical]',
  'icon-[mdi--download]',
  'icon-[mdi--drag]',
  'icon-[mdi--earth]',
  'icon-[mdi--email-open-outline]',
  'icon-[mdi--email-outline]',
  'icon-[mdi--eye-off-outline]',
  'icon-[mdi--eye-outline]',
  'icon-[mdi--file-certificate-outline]',
  'icon-[mdi--file-cog-outline]',
  'icon-[mdi--file-document-edit-outline]',
  'icon-[mdi--file-document-multiple-outline]',
  'icon-[mdi--file-document-outline]',
  'icon-[mdi--file-search-outline]',
  'icon-[mdi--file-upload-outline]',
  'icon-[mdi--filter-outline]',
  'icon-[mdi--flag]',
  'icon-[mdi--folder]',
  'icon-[mdi--folder-move-outline]',
  'icon-[mdi--folder-open-outline]',
  'icon-[mdi--folder-outline]',
  'icon-[mdi--folder-plus-outline]',
  'icon-[mdi--group]',
  'icon-[mdi--harddisk]',
  'icon-[mdi--history]',
  'icon-[mdi--home-outline]',
  'icon-[mdi--inbox-outline]',
  'icon-[mdi--information-outline]',
  'icon-[mdi--ip]',
  'icon-[mdi--ip-network]',
  'icon-[mdi--ip-network-outline]',
  'icon-[mdi--key]',
  'icon-[mdi--key-outline]',
  'icon-[mdi--key-plus]',
  'icon-[mdi--key-variant]',
  'icon-[mdi--lan]',
  'icon-[mdi--lan-connect]',
  'icon-[mdi--lan-pending]',
  'icon-[mdi--laptop]',
  'icon-[mdi--layers]',
  'icon-[mdi--license]',
  'icon-[mdi--lightbulb-on-outline]',
  'icon-[mdi--link-variant]',
  'icon-[mdi--lock-open-outline]',
  'icon-[mdi--lock-outline]',
  'icon-[mdi--logout]',
  'icon-[mdi--magnify]',
  'icon-[mdi--magnify-scan]',
  'icon-[mdi--map-marker]',
  'icon-[mdi--map-marker-outline]',
  'icon-[mdi--memory]',
  'icon-[mdi--menu]',
  'icon-[mdi--message-text-outline]',
  'icon-[mdi--monitor-dashboard]',
  'icon-[mdi--monitor-multiple]',
  'icon-[mdi--office-building]',
  'icon-[mdi--open-in-new]',
  'icon-[mdi--package-down]',
  'icon-[mdi--package-variant]',
  'icon-[mdi--package-variant-closed]',
  'icon-[mdi--paperclip]',
  'icon-[mdi--pencil]',
  'icon-[mdi--pencil-outline]',
  'icon-[mdi--plus]',
  'icon-[mdi--plus-box-multiple]',
  'icon-[mdi--power]',
  'icon-[mdi--power-off]',
  'icon-[mdi--progress-clock]',
  'icon-[mdi--pulse]',
  'icon-[mdi--radar]',
  'icon-[mdi--radio-tower]',
  'icon-[mdi--refresh]',
  'icon-[mdi--reload]',
  'icon-[mdi--restart]',
  'icon-[mdi--restart-alert]',
  'icon-[mdi--send-check-outline]',
  'icon-[mdi--send-outline]',
  'icon-[mdi--server]',
  'icon-[mdi--server-network]',
  'icon-[mdi--shape-outline]',
  'icon-[mdi--shield-account]',
  'icon-[mdi--shield-account-outline]',
  'icon-[mdi--shield-alert-outline]',
  'icon-[mdi--shield-check]',
  'icon-[mdi--shield-check-outline]',
  'icon-[mdi--shield-half-full]',
  'icon-[mdi--shield-key-outline]',
  'icon-[mdi--shield-off-outline]',
  'icon-[mdi--sort]',
  'icon-[mdi--source-branch]',
  'icon-[mdi--star]',
  'icon-[mdi--subdirectory-arrow-right]',
  'icon-[mdi--sync]',
  'icon-[mdi--tag-multiple-outline]',
  'icon-[mdi--target]',
  'icon-[mdi--theme-light-dark]',
  'icon-[mdi--trending-down]',
  'icon-[mdi--truck-outline]',
  'icon-[mdi--upload]',
  'icon-[mdi--view-dashboard-outline]',
  'icon-[mdi--view-module-outline]',
  'icon-[mdi--weather-night]',
  'icon-[mdi--weather-sunny]',
  'icon-[mdi--white-balance-sunny]',
  'icon-[mdi--circle-outline]',
  'icon-[mdi--call-split]',
  'icon-[mdi--plus-box-outline]',
  'icon-[mdi--tray]',
  'icon-[mdi--tray-remove]',
] as const
